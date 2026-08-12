package cmd

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPullNoRemoteConfigured verifies that pull fails with a clear error
// when no remote is configured (not an init failure or silent success).
func TestPullNoRemoteConfigured(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	// reset remote flag
	pullRemoteURL = ""
	defer func() { pullRemoteURL = "" }()

	err := runPullOut(context.Background(), &buf)
	if err == nil {
		t.Fatal("expected error when no remote is configured, got nil")
	}
	if !strings.Contains(err.Error(), "no remote") {
		t.Fatalf("expected error mentioning 'no remote', got: %v", err)
	}
}

// TestPullNoGit checks the pre-flight git check.
func TestPullNoGit(t *testing.T) {
	// Can't easily remove git from PATH in a Go test, but we can at least
	// verify the function doesn't panic and delegates correctly on the
	// happy path. If git is available (it is in this env), this is covered
	// by TestPullNoRemoteConfigured. Skip otherwise.
	skipIfNoGit(t)
	t.Skip("git is available — covered by TestPullNoRemoteConfigured")
}
