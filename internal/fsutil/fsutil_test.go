package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileSyncRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.yml")

	if err := WriteFileSync(path, []byte("first\n"), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := WriteFileSync(path, []byte("second: 1\n"), 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "second: 1\n" {
		t.Errorf("dest = %q, want %q", got, "second: 1\n")
	}

	// Default-mode write (perm 0) must not fail the chmod.
	if err := WriteFileSync(path, []byte("third\n"), 0); err != nil {
		t.Fatalf("write with default perm: %v", err)
	}
}

func TestWriteFileSyncKeepsDestOnFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.yml")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	// 1) temp-file creation fails: dest is a directory sitting where the
	// temp file needs to be created is not directly expressible, so use the
	// classic blocker — dest's parent is a file, so CreateTemp fails and the
	// not-yet-existing dest must not appear.
	blockedParent := filepath.Join(dir, "file-not-dir")
	if err := os.WriteFile(blockedParent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileSync(filepath.Join(blockedParent, "cfg.yml"), []byte("d"), 0o644); err == nil {
		t.Fatal("expected error writing under a file")
	}

	// 2) rename fails because the dest path is a directory; everything up
	// to the rename succeeded, so this is the crash analogue — the dest's
	// previous state (the directory and its child) must be untouched and no
	// temp litter left behind.
	destDir := path
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(destDir, "keep.txt")
	if err := os.WriteFile(child, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileSync(destDir, []byte("new\n"), 0o644); err == nil {
		t.Fatal("expected rename error when dest is a directory")
	}
	if string(mustRead(t, child)) != "original\n" {
		t.Errorf("dest damaged on failure: child got %q, want %q", mustRead(t, child), "original\n")
	}

	// 3) a successful write also leaves no temp files behind.
	goodPath := filepath.Join(dir, "good.yml")
	if err := WriteFileSync(goodPath, []byte("good\n"), 0o644); err != nil {
		t.Fatalf("good write: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var litter []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".nestor-write-") {
			litter = append(litter, e.Name())
		}
	}
	if len(litter) != 0 {
		t.Errorf("temp litter left behind: %v", litter)
	}
	if string(mustRead(t, goodPath)) != "good\n" {
		t.Errorf("good write did not land")
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestWritePermExistingKeepsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, []byte("x"), 0o640); err != nil {
		t.Fatal(err)
	}
	perm, err := WritePerm(path, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if perm != 0o640 {
		t.Fatalf("existing file mode = %v, want 0640", perm)
	}
}

func TestWritePermMissingGetsFallback(t *testing.T) {
	perm, err := WritePerm(filepath.Join(t.TempDir(), "absent"), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if perm != 0o600 {
		t.Fatalf("missing file perm = %v, want fallback 0600", perm)
	}
}

func TestWritePermStatErrorSurfaces(t *testing.T) {
	dir := t.TempDir()
	blocker := filepath.Join(dir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := WritePerm(filepath.Join(blocker, "child"), 0o600); err == nil {
		t.Fatal("expected stat error through a file-as-dir path, got nil")
	}
}
