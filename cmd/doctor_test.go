package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runDoctorForTest runs the doctor logic with config and output isolated to the
// test. Returns (output, err) so tests can assert on both.
func runDoctorForTest(t *testing.T, cfgPath string) (string, error) {
	t.Helper()
	cfgFile = cfgPath
	defer func() { cfgFile = "" }()

	out := &bytes.Buffer{}
	err := runDoctorOut(context.Background(), out)
	return out.String(), err
}

// TestDoctorNoConfig confirms doctor returns a clear error when no config is
// found at the given path, instead of panicking or silently passing.
func TestDoctorNoConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "does-not-exist.yml")

	_, err := runDoctorForTest(t, cfgPath)
	if err == nil {
		t.Fatal("expected error for missing config, got nil")
	}
	if !strings.Contains(err.Error(), "config") {
		t.Fatalf("expected config error, got: %v", err)
	}
}

// TestDoctorEmptyConfig is the "green baseline": a valid config with no
// packages, dotfiles, or secrets declared should run to completion without
// errors and report a clean environment.
func TestDoctorEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
packages:
  common: []
dotfiles:
  source: `+filepath.Join(dir, "dotfiles")+`
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctorForTest(t, cfgPath)
	if err != nil {
		t.Fatalf("expected nil error for empty config, got: %v", err)
	}

	// Each section should report its "nothing declared" line.
	for _, want := range []string{"no packages declared", "no dotfiles declared", "no secrets declared"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output, got:\n%s", want, out)
		}
	}
}

// TestDoctorMissingSourceDir confirms doctor flags a missing dotfiles source
// directory as an issue rather than silently ignoring it.
func TestDoctorMissingSourceDir(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	missingSource := filepath.Join(dir, "no-such-dir")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
dotfiles:
  source: `+missingSource+`
  strategy: copy
  templates:
    - src: bashrc.tmpl
      dest: ~/.bashrc
`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctorForTest(t, cfgPath)
	if err != nil {
		t.Fatalf("doctor should not hard-fail on a missing source dir (it reports issues), got: %v", err)
	}
	if !strings.Contains(out, "source dir missing") {
		t.Fatalf("expected 'source dir missing' in output, got:\n%s", out)
	}
}

// TestDoctorDetectsMissingPackage confirms doctor reports a declared package
// that is not installed on this machine as a missing-package issue.
func TestDoctorDetectsMissingPackage(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte(`version: 1
packages:
  common:
    - nonexistent-pkg-xyz-12345
dotfiles:
  source: `+filepath.Join(dir, "dotfiles")+`
  strategy: copy
  templates: []
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0o755); err != nil {
		t.Fatal(err)
	}

	out, err := runDoctorForTest(t, cfgPath)
	if err != nil {
		t.Fatalf("expected nil error (doctor reports issues, not hard failures), got: %v", err)
	}
	if !strings.Contains(out, "missing: nonexistent-pkg-xyz-12345") {
		t.Fatalf("expected 'missing: nonexistent-pkg-xyz-12345' in output, got:\n%s", out)
	}
}
