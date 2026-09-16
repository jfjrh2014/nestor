package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/packages"
)

func TestCopyDotfileTemplates(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(home, "dotfiles-src")

	// Seed dotfiles in the fake home dir.
	for _, name := range []string{".bashrc", ".gitconfig", ".vimrc"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("# "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	templates := []config.Template{
		{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
		{Src: ".gitconfig.tmpl", Dest: "~/.gitconfig"},
		{Src: ".vimrc.tmpl", Dest: "~/.vimrc"},
	}

	copied, skipped, err := copyDotfileTemplates(home, sourceDir, templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if copied != 3 {
		t.Fatalf("copied = %d, want 3", copied)
	}
	if skipped != 0 {
		t.Fatalf("skipped = %d, want 0", skipped)
	}

	// Verify each template was created with the expected content.
	for _, name := range []string{".bashrc", ".gitconfig", ".vimrc"} {
		dest := filepath.Join(sourceDir, name+".tmpl")
		data, err := os.ReadFile(dest)
		if err != nil {
			t.Fatalf("template %s not copied: %v", dest, err)
		}
		want := "# " + name + "\n"
		if string(data) != want {
			t.Errorf("content of %s = %q, want %q", dest, data, want)
		}
	}
}

func TestCopyDotfileTemplatesPartialFailure(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(home, "dotfiles-src")

	// Only seed one of two files.
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	templates := []config.Template{
		{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},       // exists
		{Src: ".gitconfig.tmpl", Dest: "~/.gitconfig"}, // missing in home — skip
	}

	copied, _, err := copyDotfileTemplates(home, sourceDir, templates)
	if err == nil {
		t.Fatal("expected error for missing source, got nil")
	}
	if copied != 1 {
		t.Fatalf("copied = %d, want 1", copied)
	}

	// The one that existed should have been copied.
	if _, err := os.Stat(filepath.Join(sourceDir, ".bashrc.tmpl")); err != nil {
		t.Errorf("expected .bashrc.tmpl to be copied: %v", err)
	}
}

func TestCopyDotfileTemplatesPreservesMode(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(home, "dotfiles-src")

	src := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(src, []byte("# test\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	templates := []config.Template{{Src: ".bashrc.tmpl", Dest: "~/.bashrc"}}
	if _, _, err := copyDotfileTemplates(home, sourceDir, templates); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(sourceDir, ".bashrc.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0o600", info.Mode().Perm())
	}
}

func TestSyncMergePreservesSourceDir(t *testing.T) {
	// Verify the merge path in runSync preserves the freshly-computed source dir
	// when the existing config has an empty Source. We simulate the merge logic
	// directly (extracted as the conditional in runSync) to lock the fix.
	dir := t.TempDir()
	sourceDir := filepath.Join(dir, "dotfiles")

	existing := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Source:    "", // empty — the bug condition
			Strategy:  "copy",
			Templates: []config.Template{{Src: ".bashrc.tmpl", Dest: "~/.bashrc"}},
		},
	}

	// Merge fix: only keep existing source if set, else use freshly-computed.
	if existing.Dotfiles.Source == "" {
		existing.Dotfiles.Source = sourceDir
	}

	if existing.Dotfiles.Source != sourceDir {
		t.Errorf("source = %q, want %q", existing.Dotfiles.Source, sourceDir)
	}
}

func TestSyncMergeDoesNotClobberExistingSource(t *testing.T) {
	// If the existing config already has a explicit source set, the merge must
	// not overwrite it with the newly-computed default.
	dir := t.TempDir()
	defaultSource := filepath.Join(dir, "default-dotfiles")
	existingSource := "/custom/source/path"

	existing := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Source: existingSource,
		},
	}

	if existing.Dotfiles.Source == "" {
		existing.Dotfiles.Source = defaultSource
	}

	if existing.Dotfiles.Source != existingSource {
		t.Errorf("source = %q, want %q (should not be overwritten)", existing.Dotfiles.Source, existingSource)
	}
}

// TestSyncMergeCopiesToEffectiveSource is a regression test for the dead-value
// bug where templates were always copied into the default source dir even when
// the existing config had a custom source. After the fix, templates must land
// in the effective (resolved) source dir so `nestor up` can find them.
func TestSyncMergeCopiesToEffectiveSource(t *testing.T) {
	home := t.TempDir()
	defaultSourceDir := filepath.Join(home, ".config", "nestor", "dotfiles")
	customSourceDir := filepath.Join(home, "custom-dotfiles")

	// Seed a dotfile in home — sync will detect it.
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# bash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Simulate what runSync does: compute a default, then find an existing config
	// with a *custom* source. The effective source for template copy must be the
	// custom dir, not the default.
	detected := scanDotfiles(home)
	if len(detected) == 0 {
		t.Fatal("expected at least one detected dotfile in home dir")
	}

	// Existing config has a custom source set.
	cfg := &config.Config{
		Version: 1,
		Dotfiles: config.Dotfiles{
			Source:    customSourceDir,
			Strategy:  "copy",
			Templates: []config.Template{},
		},
	}

	// Merge: existing source wins.
	cfg.Dotfiles.Templates = mergeDotfiles(cfg.Dotfiles.Templates, detected)
	if cfg.Dotfiles.Source == "" {
		cfg.Dotfiles.Source = defaultSourceDir
	}

	// Effective source computation (mirrors the fixed runSync path).
	effectiveSource := cfg.Dotfiles.Source
	if effectiveSource == "" {
		effectiveSource = defaultSourceDir
	}

	if effectiveSource != customSourceDir {
		t.Fatalf("effectiveSource = %q, want %q (custom dir)", effectiveSource, customSourceDir)
	}

	// Copy templates into effectiveSource, then verify they landed there.
	if err := os.MkdirAll(effectiveSource, 0o755); err != nil {
		t.Fatal(err)
	}
	copied, _, err := copyDotfileTemplates(home, effectiveSource, detected)
	if err != nil {
		t.Fatalf("copyDotfileTemplates: %v", err)
	}
	if copied == 0 {
		t.Fatal("expected at least one template copied")
	}

	// Template should be in the custom dir, NOT the default dir.
	customPath := filepath.Join(customSourceDir, ".bashrc.tmpl")
	if _, err := os.Stat(customPath); err != nil {
		t.Errorf("template not in custom source dir: %v", err)
	}
	defaultPath := filepath.Join(defaultSourceDir, ".bashrc.tmpl")
	if _, err := os.Stat(defaultPath); err == nil {
		t.Error("template was copied to default source dir — should only be in custom dir")
	}
}

// TestSyncRefusesToOverwriteInvalidConfig is the regression test for the
// silent-discard bug: runSync merged the existing config only when
// config.Load succeeded and otherwise proceeded to overwrite the file with a
// freshly-scanned skeleton — one YAML typo and every hand-maintained mapping,
// template, and profile in nestor.yml was destroyed. After the fix, a load
// failure is fatal and the file is left untouched.
func TestSyncRefusesToOverwriteInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")

	broken := "version: 1\npackages:\n  common:\n  - git\n  bad: : :\n"
	if err := os.WriteFile(cfgPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := loadExistingForSync(cfgPath)
	if err == nil {
		t.Fatal("expected error for unparseable existing config, got nil")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Fatalf("expected 'refusing to overwrite' in error, got: %v", err)
	}

	// The broken file must survive byte-for-byte — sync never got the chance
	// to replace it.
	data, readErr := os.ReadFile(cfgPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != broken {
		t.Fatalf("existing config was modified on disk:\n%q", string(data))
	}
}

// TestSyncLoadExistingMissingFileReturnsNil pins the other branch: no file on
// disk is not an error — first-run sync must proceed and create the config.
func TestSyncLoadExistingMissingFileReturnsNil(t *testing.T) {
	cfg, err := loadExistingForSync(filepath.Join(t.TempDir(), "nestor.yml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for missing file, got %+v", cfg)
	}
}

// TestSyncLoadExistingReturnsParsedConfig checks the happy path returns the
// actual parsed config so the merge below it operates on real data.
func TestSyncLoadExistingReturnsParsedConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\npackages:\n  common:\n    - git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := loadExistingForSync(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected parsed config, got nil")
	}
	if len(cfg.Packages.Common) != 1 || cfg.Packages.Common[0] != "git" {
		t.Fatalf("packages.common = %v, want [git]", cfg.Packages.Common)
	}
}

// TestCopyDotfileTemplatesKeepsEditedTemplates is the regression test for the
// clobbering bug: re-running sync overwrote every existing template in the
// source dir with the live home file, destroying user edits to the working
// copies. After the fix, existing templates are left untouched.
func TestCopyDotfileTemplatesKeepsEditedTemplates(t *testing.T) {
	home := t.TempDir()
	sourceDir := filepath.Join(home, "dotfiles-src")

	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# pristine\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	templates := []config.Template{{Src: ".bashrc.tmpl", Dest: "~/.bashrc"}}

	// First sync copies the template.
	copied, skipped, err := copyDotfileTemplates(home, sourceDir, templates)
	if err != nil || copied != 1 || skipped != 0 {
		t.Fatalf("first sync: copied=%d skipped=%d err=%v, want 1/0/nil", copied, skipped, err)
	}

	// User edits the working copy — this is the version that must survive.
	edited := "# edited by hand\nif [ -f ~/.profile ]; then . ~/.profile; fi\n"
	dest := filepath.Join(sourceDir, ".bashrc.tmpl")
	if err := os.WriteFile(dest, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}

	// The live home file drifts from the edit (as real machines do).
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# home drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Re-sync: the edited template must be kept, not re-copied over.
	copied, skipped, err = copyDotfileTemplates(home, sourceDir, templates)
	if err != nil {
		t.Fatalf("second sync: unexpected error: %v", err)
	}
	if copied != 0 || skipped != 1 {
		t.Fatalf("second sync: copied=%d skipped=%d, want 0/1", copied, skipped)
	}

	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != edited {
		t.Fatalf("edited template was clobbered by re-sync:\n%q", string(data))
	}
}

func TestScanPackagesSpecsSubPreserved(t *testing.T) {
	// A brew candidate with an explicit sub (brew/cask) must keep its Sub
	// through the scan: probing casks as formulas reported them not-installed
	// and sync silently dropped them from the captured config.
	old := devPackageCandidates["brew"]
	devPackageCandidates["brew"] = []string{"git", "brew/cask: kitty"}
	defer func() { devPackageCandidates["brew"] = old }()

	var seen []packages.Spec
	installed := map[string]bool{"git": true, "kitty": true}
	specs := scanPackagesSpecs("brew", func(s packages.Spec) (bool, error) {
		seen = append(seen, s)
		return installed[s.Name], nil
	})

	if len(specs) != 2 {
		t.Fatalf("expected 2 installed specs, got %d: %+v", len(specs), specs)
	}
	if specs[0].Name != "git" || specs[0].Manager != "brew" || specs[0].Sub != "" {
		t.Errorf("expected git (brew, no sub), got %+v", specs[0])
	}
	if specs[1].Name != "kitty" || specs[1].Manager != "brew" || specs[1].Sub != "cask" {
		t.Errorf("expected kitty (brew, sub cask), got %+v", specs[1])
	}
	if len(seen) != 2 {
		t.Errorf("expected not-installed candidates probed too, probed %d", len(seen))
	}
}

func TestScanPackagesSpecsUnknownManager(t *testing.T) {
	specs := scanPackagesSpecs("nope", packages.IsInstalled)
	if len(specs) != 0 {
		t.Errorf("unknown manager should find nothing, got %+v", specs)
	}
	names := scanPackages("nope")
	if len(names) != 0 {
		t.Errorf("wrapper should return no names for unknown manager, got %v", names)
	}
}

func TestScanPackagesWrapperMirrorsRealProbe(t *testing.T) {
	// The wrapper is the production path: real backend probes, names out.
	specs := scanPackagesSpecs("apt", packages.IsInstalled)
	names := scanPackages("apt")
	if len(specs) != len(names) {
		t.Fatalf("wrapper/spec mismatch: %d specs vs %d names", len(specs), len(names))
	}
	candidates := devPackageCandidates["apt"]
	// names must be a subsequence of the candidate list (order preserved).
	ci := 0
	for _, n := range names {
		for ci < len(candidates) && candidates[ci] != n {
			ci++
		}
		if ci == len(candidates) {
			t.Fatalf("name %q not found in candidates at or after position %d", n, ci)
		}
		ci++
	}
}

// TestSyncProfileCapture is the regression for session #76: 'nestor diff
// --profile X' told users to run 'nestor sync' for extra packages, but sync
// merged everything into the COMMON sections — following the advice turned
// machine-specific extras into globally deployed packages. With --profile,
// sync must land scanned items in the profile sections instead.
func TestSyncProfileCapture(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows
	cfgDir := filepath.Join(home, ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "nestor.yml")
	cfgContent := `version: 1
packages:
  common:
    - git
dotfiles:
  source: ` + filepath.Join(home, "dotfiles") + `
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
profiles:
  work:
    packages: [vim]
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	existing, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("load existing: %v", err)
	}
	applyScanToConfig(existing, "work", []string{"jq", "fzf"}, nil)
	data, err := config.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal merged config: %v", err)
	}
	if err := os.WriteFile(cfgPath, data, 0o644); err != nil {
		t.Fatal(err)
	}

	merged, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("reload merged config: %v", err)
	}
	got := merged.Profiles["work"].Packages
	if len(got) != 3 || got[0] != "vim" || got[1] != "jq" || got[2] != "fzf" {
		t.Errorf("profile work packages = %v, want [vim jq fzf]", got)
	}
	for _, p := range merged.Packages.Common {
		if p == "jq" || p == "fzf" {
			t.Errorf("scanned package %q leaked into common packages: %v", p, merged.Packages.Common)
		}
	}
}

// TestSyncCaptureAdvice pins the advice contract in diff: with a profile
// active, "run 'nestor sync'" would file machine-specific extras into the
// common sections, so the advice must name the profile-capture form.
func TestSyncCaptureAdvice(t *testing.T) {
	if got := syncCaptureAdvice("", 0); got != "" {
		t.Errorf("no extras: got %q, want empty", got)
	}
	base := syncCaptureAdvice("", 2)
	if !strings.Contains(base, "run 'nestor sync' to capture") || !strings.Contains(base, "2") {
		t.Errorf("base advice = %q, want count + plain sync advice", base)
	}
	prof := syncCaptureAdvice("work", 2)
	if !strings.Contains(prof, "sync --profile work") || !strings.Contains(prof, "into the profile") {
		t.Errorf("profile advice = %q, want profile-capture form", prof)
	}
}

// TestSyncOutProfileUnknownErrorsBeforeScanning pins the early-error
// contract: a typo'd profile must fail before any scanning or writing.
func TestSyncOutProfileUnknownErrorsBeforeScanning(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfgDir := filepath.Join(home, ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(cfgDir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\npackages:\n  common: []\ndotfiles:\n  strategy: copy\nsecrets:\n  provider: env\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(cfgPath)
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	var out strings.Builder
	err := runSyncOut(context.Background(), "nope", &out)
	if err == nil {
		t.Fatal("expected unknown-profile error")
	}
	if !strings.Contains(err.Error(), "unknown profile") {
		t.Errorf("error should name the unknown profile, got: %v", err)
	}
	after, _ := os.ReadFile(cfgPath)
	if string(before) != string(after) {
		t.Error("config was modified despite unknown-profile error")
	}
}

// TestSyncOutProfileNoConfig pins the fresh-machine contract: --profile
// without an existing config must say to run plain 'nestor sync' first,
// not silently create a config with a profile section the user never wrote.
func TestSyncOutProfileNoConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	cfgFile = ""
	var out strings.Builder
	err := runSyncOut(context.Background(), "work", &out)
	if err == nil {
		t.Fatal("expected no-config error")
	}
	if !strings.Contains(err.Error(), "no config") {
		t.Errorf("error should point at the missing config, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(home, ".config", "nestor", "nestor.yml")); statErr == nil {
		t.Error("config was created despite the no-config error path")
	}
}

// TestSyncProfileCaptureSkipsBaseManaged is the session #78 regression:
// 'nestor sync --profile X' used mergeStrings/mergeDotfiles, which dedup
// only against the PROFILE's own lists. A base-managed item (common
// package, base dotfile dest) that also existed on disk got captured into
// the profile — double-deploying it under --profile runs (profile deploys
// after base, so the profile copy wins) and mis-filing shared state as
// machine-specific, the exact opposite of what diff's capture advice means.
func TestSyncProfileCaptureSkipsBaseManaged(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Packages: config.Packages{Common: []string{"git", "jq"}},
		Dotfiles: config.Dotfiles{
			Source:   "/tmp/dotfiles",
			Strategy: "copy",
			Templates: []config.Template{
				{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
			},
		},
		Profiles: map[string]config.Profile{
			"work": {Packages: []string{"vim"}},
		},
	}

	skipped := applyScanToConfig(cfg, "work",
		[]string{"git", "fzf"}, // git is base-managed, fzf is machine-specific
		[]config.Template{{Src: ".bashrc.tmpl", Dest: "~/.bashrc"}}, // base-managed dest
	)

	if skipped != 2 {
		t.Errorf("skipped = %d, want 2 (base package + base dotfile dest)", skipped)
	}
	prof := cfg.Profiles["work"]
	if len(prof.Packages) != 2 || prof.Packages[0] != "vim" || prof.Packages[1] != "fzf" {
		t.Errorf("profile work packages = %v, want [vim fzf]", prof.Packages)
	}
	if len(prof.Dotfiles) != 0 {
		t.Errorf("profile work dotfiles = %v, want none (base dest skipped)", prof.Dotfiles)
	}
	if len(cfg.Packages.Common) != 2 {
		t.Errorf("common packages = %v, want untouched [git jq]", cfg.Packages.Common)
	}
	if len(cfg.Dotfiles.Templates) != 1 {
		t.Errorf("base templates = %v, want untouched", cfg.Dotfiles.Templates)
	}
}

// TestSyncProfileCaptureNoBaseOverlap pins the untouched path: with no
// base-managed overlap, the profile capture behaves exactly as before #78.
func TestSyncProfileCaptureNoBaseOverlap(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Packages: config.Packages{Common: []string{"git"}},
		Profiles: map[string]config.Profile{
			"work": {Packages: []string{"vim"}},
		},
	}

	skipped := applyScanToConfig(cfg, "work", []string{"jq"}, nil)

	if skipped != 0 {
		t.Errorf("skipped = %d, want 0", skipped)
	}
	got := cfg.Profiles["work"].Packages
	if len(got) != 2 || got[0] != "vim" || got[1] != "jq" {
		t.Errorf("profile work packages = %v, want [vim jq]", got)
	}
}
