package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveImporterUnknownSource(t *testing.T) {
	_, err := resolveImporter("gitwatch", "")
	if err == nil {
		t.Fatal("expected error for unknown source, got nil")
	}
	if !strings.Contains(err.Error(), "unknown import source") {
		t.Errorf("error = %q, want it to mention 'unknown import source'", err.Error())
	}
}

func TestResolveImporterPathWithAutoRejected(t *testing.T) {
	// auto-detect + a path is nonsensical: there's no single source to point at
	// without naming the tool. resolveImporter should reject it explicitly
	// rather than silently ignoring the path (the old dead-value failure mode).
	_, err := resolveImporter("", "/some/path")
	if err == nil {
		t.Fatal("expected error for auto + path, got nil")
	}
	if !strings.Contains(err.Error(), "explicit source") {
		t.Errorf("error = %q, want it to mention 'explicit source'", err.Error())
	}
}

func TestResolveImporterPathWithYadmRejected(t *testing.T) {
	// yadm reads `yadm list -a` and has no notion of a source path. A path
	// passed with yadm should be rejected, not silently dropped.
	_, err := resolveImporter("yadm", "/some/path")
	if err == nil {
		t.Fatal("expected error for yadm + path, got nil")
	}
	if !strings.Contains(err.Error(), "does not accept a path") {
		t.Errorf("error = %q, want it to mention 'does not accept a path'", err.Error())
	}
}

func TestResolveImporterBrewfileHonoursPath(t *testing.T) {
	// The seventh dead-value bug: an explicit Brewfile path used to be
	// declared, written into a global, and read with `_ = importSource`, but
	// never plumbed into NewBrewfile (which always got ""). Now the path
	// reaches the constructor. A missing file should surface NewBrewfile's
	// "not found" error — proving the path reached it, rather than the old
	// behaviour of silently scanning CWD.
	_, err := resolveImporter("brewfile", "/nonexistent/no-such-Brewfile")
	if err == nil {
		t.Fatal("expected error for missing Brewfile path, got nil (path was likely ignored)")
	}
	if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "Brewfile not found") && !strings.Contains(err.Error(), "no file or directory") {
		t.Errorf("error = %q, want a not-found error proving the path reached NewBrewfile", err.Error())
	}
}

func TestResolveImporterChezmoiHonoursPath(t *testing.T) {
	// Same dead-value class: an explicit chezmoi source path must reach
	// NewChezmoi, not be discarded. A missing dir should surface its
	// "source dir not found" error.
	_, err := resolveImporter("chezmoi", "/nonexistent/no-such-chezmoi-dir")
	if err == nil {
		t.Fatal("expected error for missing chezmoi dir, got nil (path was likely ignored)")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want a not-found error proving the path reached NewChezmoi", err.Error())
	}
}

// --- runImport writer-injection tests (session #42) ---

// setupImportTest writes a nestor.yml in a temp dir, points cfgFile at it, and
// returns the dir (so callers can place a Brewfile or dotfile source inside).
func setupImportTest(t *testing.T) (dir, cfgPath string) {
	t.Helper()
	dir = t.TempDir()
	cfgPath = filepath.Join(dir, "nestor.yml")
	content := `version: 1
packages:
  common:
    - git
dotfiles:
  source: ` + dir + `/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0755); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = "" })
	return dir, cfgPath
}

// writeImportBrewfile creates a Brewfile in dir and returns its path.
func writeImportBrewfile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "Brewfile")
	content := []byte(`brew "ripgrep"
brew "fd"
cask "visual-studio-code"
mas "Xcode"
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestImportBrewfileDryRunOutput asserts the preview path writes to the buffer,
// reports packages found + skipped, and does not write to config. Exercises the
// full fetch → display → dry-run-return path through the injected writer.
func TestImportBrewfileDryRunOutput(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	importDryRun = true
	t.Cleanup(func() { importDryRun = false })

	var buf bytes.Buffer
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("runImport dry-run: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"source = brewfile",
		"packages found: 3",
		"+ homebrew: ripgrep",
		"+ homebrew/cask: visual-studio-code",
		"skipped: 1",
		"(dry-run, nothing written)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot:\n%s", want, out)
		}
	}

	// Config must not have changed on dry-run
	raw, _ := os.ReadFile(cfgPath)
	if strings.Contains(string(raw), "ripgrep") {
		t.Error("dry-run should not write ripgrep into config")
	}
}

// TestImportBrewfileNothingNew asserts the "all items already in config" path
// writes to the buffer and returns nil without error.
func TestImportBrewfileNothingNew(t *testing.T) {
	dir, _ := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	var buf bytes.Buffer
	importDryRun = false

	// First import adds the items
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("first import: %v", err)
	}

	// Second import should find nothing new
	buf.Reset()
	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("second import: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"source = brewfile",
		"packages found: 3",
		"nothing new to add",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("second-import output missing %q\ngot:\n%s", want, out)
		}
	}
	if strings.Contains(out, "imported ") {
		t.Errorf("second import should not report items imported\ngot:\n%s", out)
	}
}

// TestImportBrewfileImportAndWrite asserts the write path reports the count and
// actually persists packages into the config file.
func TestImportBrewfileImportAndWrite(t *testing.T) {
	dir, cfgPath := setupImportTest(t)
	brewPath := writeImportBrewfile(t, dir)

	var buf bytes.Buffer
	importDryRun = false

	if err := runImport("brewfile", brewPath, &buf); err != nil {
		t.Fatalf("runImport: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "imported 3 new items") {
		t.Errorf("expected 'imported 3 new items' in output\ngot:\n%s", out)
	}
	if !strings.Contains(out, cfgPath) {
		t.Errorf("expected output to mention config path %s\ngot:\n%s", cfgPath, out)
	}

	// Config file should now contain the imported packages
	raw, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{"homebrew: ripgrep", "homebrew: fd", "homebrew/cask: visual-studio-code"} {
		if !strings.Contains(s, want) {
			t.Errorf("config file missing %q after import", want)
		}
	}
}

// TestImportMissingConfigReturnsError asserts a config load failure surfaces
// as an error from runImport (not a panic or silent success).
func TestImportMissingConfigReturnsError(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "does-not-exist.yml")
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = "" })

	var buf bytes.Buffer
	err := runImport("brewfile", "", &buf)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "import:") {
		t.Errorf("error should be wrapped as 'import:', got: %v", err)
	}
	if buf.Len() > 0 {
		t.Errorf("no output expected on config load failure, got: %q", buf.String())
	}
}
