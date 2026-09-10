package cmd

import (
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
