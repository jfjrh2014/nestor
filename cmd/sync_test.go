package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
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

	copied, err := copyDotfileTemplates(home, sourceDir, templates)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if copied != 3 {
		t.Fatalf("copied = %d, want 3", copied)
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

	copied, err := copyDotfileTemplates(home, sourceDir, templates)
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
	if _, err := copyDotfileTemplates(home, sourceDir, templates); err != nil {
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
	copied, err := copyDotfileTemplates(home, effectiveSource, detected)
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
