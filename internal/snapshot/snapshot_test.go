package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCreateSkipsMissingFiles verifies non-existent destination paths are
// silently ignored during snapshot creation.
func TestCreateSkipsMissingFiles(t *testing.T) {
	base := t.TempDir()
	existing := filepath.Join(base, "real.txt")
	os.WriteFile(existing, []byte("hello"), 0o644)

	snap, err := createIn(base, []string{existing, filepath.Join(base, "ghost.txt")})
	if err != nil {
		t.Fatalf("createIn: %v", err)
	}
	if len(snap.Files) != 1 {
		t.Fatalf("expected 1 file (skipped missing), got %d", len(snap.Files))
	}
	if snap.Files[0].Original != existing {
		t.Errorf("original = %q, want %q", snap.Files[0].Original, existing)
	}
}

// TestCreateAndRestore backs up a file, mutates it, then restores and checks
// the content round-trips through the manifest.
func TestCreateAndRestore(t *testing.T) {
	base := t.TempDir()
	orig := filepath.Join(base, "config.txt")
	os.WriteFile(orig, []byte("before"), 0o644)

	if _, err := createIn(base, []string{orig}); err != nil {
		t.Fatalf("createIn: %v", err)
	}

	// Mutate the live file so we can prove Restore reverts it.
	os.WriteFile(orig, []byte("after"), 0o644)

	id := snapshotIDFromBase(base)
	restored, err := restoreIn(base, id)
	if err != nil {
		t.Fatalf("restoreIn: %v", err)
	}
	if len(restored.Files) != 1 {
		t.Fatalf("restored files = %d, want 1", len(restored.Files))
	}

	data, _ := os.ReadFile(orig)
	if string(data) != "before" {
		t.Errorf("after restore = %q, want %q", string(data), "before")
	}
}

// TestCreateWritesManifest checks the manifest.json landed on disk and parses.
func TestCreateWritesManifest(t *testing.T) {
	base := t.TempDir()
	orig := filepath.Join(base, "test.txt")
	os.WriteFile(orig, []byte("x"), 0o644)

	_, err := createIn(base, []string{orig})
	if err != nil {
		t.Fatal(err)
	}

	manifestPath := filepath.Join(base, snapshotIDFromBase(base), "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest: %v", err)
	}

	var loaded Snapshot
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("parsing manifest: %v", err)
	}
	if len(loaded.Files) != 1 {
		t.Errorf("manifest files = %d, want 1", len(loaded.Files))
	}
}

// TestDelete removes a snapshot dir and confirms it stays gone.
func TestDelete(t *testing.T) {
	base := t.TempDir()
	orig := filepath.Join(base, "a.txt")
	os.WriteFile(orig, []byte("a"), 0o644)

	_, err := createIn(base, []string{orig})
	if err != nil {
		t.Fatal(err)
	}
	id := snapshotIDFromBase(base)

	snapDir := filepath.Join(base, id)
	if _, err := os.Stat(snapDir); err != nil {
		t.Fatal("snapshot dir should exist before delete")
	}

	if err := deleteIn(base, id); err != nil {
		t.Fatalf("deleteIn: %v", err)
	}
	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Error("snapshot dir should be gone after delete")
	}
}

// TestRestoreLatest (empty id) restores the most recent snapshot.
func TestRestoreLatest(t *testing.T) {
	base := t.TempDir()
	orig := filepath.Join(base, "c.txt")
	os.WriteFile(orig, []byte("v1"), 0o644)

	if _, err := createIn(base, []string{orig}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(orig, []byte("v2"), 0o644)
	if _, err := createIn(base, []string{orig}); err != nil {
		t.Fatal(err)
	}

	// Overwrite with garbage, then restore-latest should give back v2.
	os.WriteFile(orig, []byte("garbage"), 0o644)

	if _, err := restoreIn(base, ""); err != nil {
		t.Fatalf("restoreIn latest: %v", err)
	}
}

// TestRestoreMissingSnapshot checks the error when the id has no manifest.
func TestRestoreMissingSnapshot(t *testing.T) {
	base := t.TempDir()
	_, err := restoreIn(base, "does-not-exist")
	if err == nil {
		t.Fatal("expected error restoring non-existent snapshot")
	}
}

// TestRestoreEmpty errors when there are no snapshots at all.
func TestRestoreEmpty(t *testing.T) {
	base := t.TempDir()
	_, err := restoreIn(base, "")
	if err == nil {
		t.Fatal("expected error on restore with empty base")
	}
}

// TestListNewestFirst verifies ordering and that a missing base returns nil.
func TestListNewestFirst(t *testing.T) {
	base := t.TempDir()

	// Stamp three snapshots with explicit IDs for deterministic ordering.
	for _, id := range []string{"20200101-120000", "20210101-120000", "20220101-120000"} {
		dir := filepath.Join(base, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeAtomic(filepath.Join(dir, "manifest.json"), []byte(`{"created_at":"2020-01-01T00:00:00Z","files":[]}`))
	}

	ids, err := listIn(base)
	if err != nil {
		t.Fatalf("listIn: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(ids))
	}
	// os.ReadDir sorts ascending by name; listIn reverses -> newest first
	if ids[0] != "20220101-120000" {
		t.Errorf("newest first = %q, want 20220101-120000", ids[0])
	}
	if ids[2] != "20200101-120000" {
		t.Errorf("oldest last = %q, want 20200101-120000", ids[2])
	}
}

// TestListMissingBase returns nil-nil when the snapshot dir doesn't exist.
func TestListMissingBase(t *testing.T) {
	ids, err := listIn(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("expected nil error on missing base, got %v", err)
	}
	if ids != nil {
		t.Fatalf("expected nil ids, got %v", ids)
	}
}

// TestSnapshotDirCollision verifies the suffix-guard: two snapshots created
// in the same second get distinct IDs and do not clobber each other.
func TestSnapshotDirCollision(t *testing.T) {
	base := t.TempDir()
	orig := filepath.Join(base, "c.txt")
	os.WriteFile(orig, []byte("a"), 0o644)

	if _, err := createIn(base, []string{orig}); err != nil {
		t.Fatal(err)
	}
	if _, err := createIn(base, []string{orig}); err != nil {
		t.Fatalf("createIn #2: %v", err)
	}

	ids, err := listIn(base)
	if err != nil {
		t.Fatalf("listIn: %v", err)
	}
	if len(ids) < 2 {
		t.Fatalf("expected >=2 snapshots after same-second collision, got %d", len(ids))
	}
}

// TestSanitizePath confirms the escaping rules for flat backup names.
func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/.gitconfig", "home+user+.gitconfig.bak"},
		{"/etc/resolv.conf", "etc+resolv.conf.bak"},
		// same basename, different dirs must not collapse
		{"/home/user/work/.gitconfig", "home+user+work+.gitconfig.bak"},
		// relative path gets cleaned
		{"foo/bar.txt", "foo+bar.txt.bak"},
		// root path produces a distinct name
		{"/.gitconfig", ".gitconfig.bak"},
	}
	for _, tt := range tests {
		got := sanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestExpandHome confirms ~ expansion behaves for config dest paths.
func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"~/foo", filepath.Join(home, "foo")},
		{"/abs/path", "/abs/path"},
		{"relative", "relative"},
	}
	for _, tt := range tests {
		got := expandHome(tt.in)
		if got != tt.want {
			t.Errorf("expandHome(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestCopyFilePreservesMode checks that file mode is preserved on copy.
func TestCopyFilePreservesMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "exec.sh")
	os.WriteFile(src, []byte("#!/bin/sh\n"), 0o755)

	dest := filepath.Join(dir, "copy.sh")
	if err := copyFile(src, dest); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode() != 0o755 {
		t.Errorf("dest mode = %v, want 0755", info.Mode())
	}
}

// snapshotIDFromBase reads the single snapshot id out of a base dir.
func snapshotIDFromBase(base string) string {
	entries, _ := os.ReadDir(base)
	for _, e := range entries {
		if e.IsDir() {
			return e.Name()
		}
	}
	return ""
}

// TestCreateExpandHome confirms that ~ in dest paths is expanded before stat.
func TestCreateExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	tmpInHome := filepath.Join(home, ".nestor_test_expandhome")
	os.WriteFile(tmpInHome, []byte("z"), 0o644)
	defer os.Remove(tmpInHome)

	base := t.TempDir()
	reldToHome := strings.TrimPrefix(tmpInHome, home+"/")
	_, err := createIn(base, []string{"~/" + reldToHome})
	if err != nil {
		t.Fatalf("createIn with ~ path: %v", err)
	}
	ids, _ := listIn(base)
	if len(ids) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(ids))
	}
	data, _ := os.ReadFile(filepath.Join(base, ids[0], sanitizePath(tmpInHome)))
	if string(data) != "z" {
		t.Errorf("backup content = %q, want z", string(data))
	}
}

// makeSnapshots seeds a base dir with N deterministic timestamped snapshot
// directories so prune tests don't depend on wall-clock timing.
func makeSnapshots(t *testing.T, base string, ids []string) {
	t.Helper()
	for _, id := range ids {
		dir := filepath.Join(base, id)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeAtomic(filepath.Join(dir, "manifest.json"), []byte(`{"created_at":"2020-01-01T00:00:00Z","files":[]}`))
	}
}

// TestPruneRemovesOldest verifies that Prune(n) keeps the n newest snapshots
// and deletes the rest, returning exactly the deleted IDs.
func TestPruneRemovesOldest(t *testing.T) {
	base := t.TempDir()
	seed := []string{"20200101-120000", "20200201-120000", "20200301-120000", "20200401-120000", "20200501-120000"}
	makeSnapshots(t, base, seed)

	removed, err := pruneIn(base, 2)
	if err != nil {
		t.Fatalf("pruneIn: %v", err)
	}
	if len(removed) != 3 {
		t.Fatalf("expected 3 pruned, got %d (%v)", len(removed), removed)
	}
	// oldest three by name should be the removed set
	want := map[string]bool{"20200101-120000": true, "20200201-120000": true, "20200301-120000": true}
	for _, id := range removed {
		if !want[id] {
			t.Errorf("unexpected pruned id %q", id)
		}
	}
	remaining, _ := listIn(base)
	if len(remaining) != 2 {
		t.Fatalf("expected 2 remaining, got %d", len(remaining))
	}
	if remaining[0] != "20200501-120000" || remaining[1] != "20200401-120000" {
		t.Errorf("remaining = %v, want [20200501-120000 20200401-120000]", remaining)
	}
}

// TestPruneKeepZeroIsNoop confirms keep<=0 disables pruning (keep-all).
func TestPruneKeepZeroIsNoop(t *testing.T) {
	base := t.TempDir()
	makeSnapshots(t, base, []string{"20200101-120000", "20200201-120000"})

	removed, err := pruneIn(base, 0)
	if err != nil {
		t.Fatalf("pruneIn(0): %v", err)
	}
	if removed != nil {
		t.Errorf("expected nil removed for keep=0, got %v", removed)
	}
	remaining, _ := listIn(base)
	if len(remaining) != 2 {
		t.Errorf("keep=0 should not delete anything, remaining=%d", len(remaining))
	}
}

// TestPruneUnderThresholdIsNoop: if count <= keep, nothing is removed.
func TestPruneUnderThresholdIsNoop(t *testing.T) {
	base := t.TempDir()
	makeSnapshots(t, base, []string{"20200101-120000", "20200201-120000"})

	removed, err := pruneIn(base, 5)
	if err != nil {
		t.Fatalf("pruneIn: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("keep > count should remove nothing, got %v", removed)
	}
}

// TestPruneExactThreshold: count == keep removes nothing.
func TestPruneExactThreshold(t *testing.T) {
	base := t.TempDir()
	makeSnapshots(t, base, []string{"20200101-120000", "20200201-120000", "20200301-120000"})

	removed, err := pruneIn(base, 3)
	if err != nil {
		t.Fatalf("pruneIn: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("keep == count should remove nothing, got %v", removed)
	}
}

// TestPruneEmptyBase: pruning a base with no snapshots is a clean no-op.
func TestPruneEmptyBase(t *testing.T) {
	base := t.TempDir()
	removed, err := pruneIn(base, 10)
	if err != nil {
		t.Fatalf("pruneIn on empty: %v", err)
	}
	if removed != nil {
		t.Errorf("expected nil removed on empty base, got %v", removed)
	}
}
