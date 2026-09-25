package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCIForTest isolates config path and the quiet flag to the test.
func runCIForTest(t *testing.T, cfgPath string, quiet bool, out *bytes.Buffer) error {
	t.Helper()
	cfgFile = cfgPath
	ciQuiet = quiet
	defer func() {
		cfgFile = ""
		ciQuiet = false
	}()

	return runCI(out)
}

func TestCIValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\npackages:\n  common:\n    - git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runCIForTest(t, cfgPath, false, out); err != nil {
		t.Fatalf("expected nil error for valid config, got %v", err)
	}
	if !strings.Contains(out.String(), "config valid") {
		t.Fatalf("expected 'config valid' in output:\n%s", out.String())
	}
}

// TestCIInvalidConfig: a config with secret mappings but no provider passes
// config.Load (empty provider = env default, it's in validProviders) but is
// flagged as an error by ci.Validate's validateSecrets.
func TestCIInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
secrets:
  mappings:
    - key: API_TOKEN
      inject:
        `+destFile+`: "token={{.API_TOKEN}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	err := runCIForTest(t, cfgPath, false, out)
	if err == nil {
		t.Fatal("expected error for config that fails ci validation, got nil")
	}
	if !strings.Contains(out.String(), "error") {
		t.Fatalf("expected error count in output:\n%s", out.String())
	}
}

func TestCIQuietOnSuccess(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\npackages:\n  common:\n    - git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runCIForTest(t, cfgPath, true, out); err != nil {
		t.Fatalf("expected nil error for valid config, got %v", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output in quiet mode on success, got %d bytes: %s", out.Len(), out.String())
	}
}

// TestCIQuietOnErrorStillOutputs: quiet mode suppresses output only on success.
// On failure, findings are still printed.
func TestCIQuietOnErrorStillOutputs(t *testing.T) {
	dir := t.TempDir()
	destFile := filepath.Join(dir, "app.conf")
	if err := os.WriteFile(destFile, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
secrets:
  mappings:
    - key: API_TOKEN
      inject:
        `+destFile+`: "token={{.API_TOKEN}}"
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	err := runCIForTest(t, cfgPath, true, out)
	if err == nil {
		t.Fatal("expected error for invalid config even in quiet mode")
	}
	if out.Len() == 0 {
		t.Fatal("expected output on failure even in quiet mode, got empty")
	}
}

func TestCIMissingConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nonexistent.yml")

	out := &bytes.Buffer{}
	err := runCIForTest(t, cfgPath, false, out)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
}

// TestCISourceFallbackPinsSrcExistenceCheck pins the session #87 fix: 'nestor
// ci' passed cfg.Dotfiles.Source straight through, so a config relying on the
// default source dir (dotfiles.source unset) silently skipped template-src
// existence checks — a src that fails at deploy time validated clean. The
// command now falls back to config.DefaultDotfilesSource like every other
// consumer, so a missing src is reported.
func TestCISourceFallbackPinsSrcExistenceCheck(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgPath := filepath.Join(home, ".config", "nestor", "nestor.yml")
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatal(err)
	}
	// No dotfiles.source declared; src/.bashrc.tmpl does NOT exist.
	if err := os.WriteFile(cfgPath, []byte("version: 1\ndotfiles:\n  templates:\n    - src: .bashrc.tmpl\n      dest: ~/.bashrc\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out := &bytes.Buffer{}
	if err := runCIForTest(t, cfgPath, false, out); err != nil {
		t.Fatalf("missing src is a warning, not an error: %v", err)
	}
	if !strings.Contains(out.String(), "src-missing") && !strings.Contains(out.String(), "not found") {
		t.Fatalf("expected a missing-src warning in output:\n%s", out.String())
	}

	// Control: when the template DOES exist, no src warning appears.
	srcDir := filepath.Join(home, ".config", "nestor", "dotfiles")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, ".bashrc.tmpl"), []byte("export NESTOR=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out2 := &bytes.Buffer{}
	if err := runCIForTest(t, cfgPath, false, out2); err != nil {
		t.Fatalf("expected valid config once src exists, got %v:\n%s", err, out2.String())
	}
	if !strings.Contains(out2.String(), "config valid") {
		t.Fatalf("expected 'config valid':\n%s", out2.String())
	}
	if strings.Contains(out2.String(), "not found") {
		t.Fatalf("src exists but a missing-src warning leaked:\n%s", out2.String())
	}
}
