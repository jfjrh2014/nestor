package cmd

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serveYAML spins up a test HTTP server that responds with the given body at any path.
func serveYAML(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, string(body))
	}))
}

// validMinimalYAML is a minimal valid nestor.yml used by the local HTTP test server
// in the restore package tests. We reuse it here for the writer-injection tests.
const validMinimalYAML = "version: 1\npackages:\n  common:\n    - git\n"

// TestRestoreEmptyURLReturnsError verifies the flag guard fires before any fetch.
func TestRestoreEmptyURLReturnsError(t *testing.T) {
	var buf bytes.Buffer
	// reset flag state to defaults so prior test runs don't leak
	restoreOutput = ""
	restoreDryRun = false
	restoreForce = false

	err := runRestoreOut("", &buf)
	if err == nil {
		t.Fatal("expected error for empty --from, got nil")
	}
	if !strings.Contains(err.Error(), "--from") {
		t.Fatalf("expected error to mention --from, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("expected no output before guard, got %q", buf.String())
	}
}

// TestRestoreDryRunWritesNoFile uses a real HTTP server (from the restore package)
// to verify the dry-run path fetches, validates, previews, but does not write.
func TestRestoreDryRunWritesNoFile(t *testing.T) {
	dir := t.TempDir()
	data := []byte(validMinimalYAML)

	// Spin up a tiny HTTP server serving a YAML file.
	srv := serveYAML(t, data)
	defer srv.Close()

	dest := filepath.Join(dir, "nestor.yml")
	restoreOutput = dest
	restoreDryRun = true
	restoreForce = false
	t.Cleanup(func() {
		restoreOutput = ""
		restoreDryRun = false
		restoreForce = false
	})

	var buf bytes.Buffer
	if err := runRestoreOut(srv.URL+"/nestor.yml", &buf); err != nil {
		t.Fatalf("runRestoreOut dry-run failed: %v", err)
	}

	out := buf.String()
	for _, want := range []string{"fetching", "fetched", "valid", "preview", "dry run"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull:\n%s", want, out)
		}
	}

	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("dry run should not write file %s, got stat err=%v", dest, err)
	}
}

// TestRestoreWritePathCreatesFile verifies the non-dry-run path writes the file
// to the configured output destination.
func TestRestoreWritePathCreatesFile(t *testing.T) {
	dir := t.TempDir()
	data := []byte(validMinimalYAML)

	srv := serveYAML(t, data)
	defer srv.Close()

	dest := filepath.Join(dir, "out.yml")
	restoreOutput = dest
	restoreDryRun = false
	restoreForce = false
	t.Cleanup(func() {
		restoreOutput = ""
		restoreDryRun = false
		restoreForce = false
	})

	var buf bytes.Buffer
	if err := runRestoreOut(srv.URL+"/out.yml", &buf); err != nil {
		t.Fatalf("runRestoreOut write failed: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("expected file at %s: %v", dest, err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("file content mismatch:\nwant %q\ngot  %q", data, got)
	}
	if !strings.Contains(buf.String(), "config written to") {
		t.Errorf("output missing write confirmation, got: %s", buf.String())
	}
}

// TestRestoreRefusesOverwriteWithoutForce locks in that --force is required.
func TestRestoreRefusesOverwriteWithoutForce(t *testing.T) {
	dir := t.TempDir()
	data := []byte(validMinimalYAML)

	srv := serveYAML(t, data)
	defer srv.Close()

	dest := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(dest, []byte("old content"), 0o644); err != nil {
		t.Fatal(err)
	}

	restoreOutput = dest
	restoreDryRun = false
	restoreForce = false
	t.Cleanup(func() {
		restoreOutput = ""
		restoreDryRun = false
		restoreForce = false
	})

	var buf bytes.Buffer
	err := runRestoreOut(srv.URL+"/nestor.yml", &buf)
	if err == nil {
		t.Fatal("expected overwrite error without --force, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}

	got, _ := os.ReadFile(dest)
	if string(got) != "old content" {
		t.Errorf("file should be unchanged, got %q", got)
	}
}
