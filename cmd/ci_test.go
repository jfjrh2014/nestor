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
