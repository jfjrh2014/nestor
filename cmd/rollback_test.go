package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/snapshot"
)

// snapshotHome isolates HOME to a temp dir so snapshot.Create/List/Restore/Prune
// operate on a throwaway directory instead of the real ~/.config/nestor.
func snapshotHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	customHome := filepath.Join(dir, "home")
	if err := os.MkdirAll(customHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", customHome)
	t.Setenv("USERPROFILE", customHome)
	return customHome
}

// TestRollbackLatestRestoresFile creates a snapshot, mutates the file,
// then rolls back to the latest snapshot and verifies content is reverted.
func TestRollbackLatestRestoresFile(t *testing.T) {
	snapshotHome(t)
	orig := filepath.Join(os.Getenv("HOME"), "project", ".bashrc")
	os.MkdirAll(filepath.Dir(orig), 0o755)
	os.WriteFile(orig, []byte("old config"), 0o644)

	snap, err := snapshot.Create([]string{orig})
	if err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}
	if len(snap.Files) != 1 {
		t.Fatalf("expected 1 file in snapshot, got %d", len(snap.Files))
	}

	// Mutate the live file.
	os.WriteFile(orig, []byte("new config"), 0o644)

	var buf bytes.Buffer
	if err := runRollback("", &buf); err != nil {
		t.Fatalf("runRollback: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "restored 1 files") {
		t.Errorf("output should mention 'restored 1 files', got: %s", out)
	}

	data, _ := os.ReadFile(orig)
	if string(data) != "old config" {
		t.Errorf("after rollback = %q, want %q", string(data), "old config")
	}
}

// TestRollbackEmptyErrors confirms rollback with no snapshots returns an error.
func TestRollbackEmptyErrors(t *testing.T) {
	snapshotHome(t)
	var buf bytes.Buffer
	err := runRollback("", &buf)
	if err == nil {
		t.Fatal("expected error rolling back with no snapshots")
	}
}

// TestRollbackSpecificSnapshot passes an explicit ID and verifies it restores.
func TestRollbackSpecificSnapshot(t *testing.T) {
	snapshotHome(t)
	orig := filepath.Join(os.Getenv("HOME"), "app.conf")
	os.WriteFile(orig, []byte("v1"), 0o644)

	snap, err := snapshot.Create([]string{orig})
	if err != nil {
		t.Fatalf("snapshot.Create: %v", err)
	}

	ids, err := snapshot.List()
	if err != nil || len(ids) != 1 {
		t.Fatalf("List: ids=%v err=%v", ids, err)
	}

	os.WriteFile(orig, []byte("v2"), 0o644)

	var buf bytes.Buffer
	if err := runRollback(ids[0], &buf); err != nil {
		t.Fatalf("runRollback(%s): %v", ids[0], err)
	}

	if !strings.Contains(buf.String(), ids[0]) {
		t.Errorf("output should mention id %q, got: %s", ids[0], buf.String())
	}

	data, _ := os.ReadFile(orig)
	_ = snap // suppress unused warning
	if string(data) != "v1" {
		t.Errorf("after rollback = %q, want %q", string(data), "v1")
	}
}

// TestSnapshotsListEmpty shows a friendly message when nothing exists.
func TestSnapshotsListEmpty(t *testing.T) {
	snapshotHome(t)
	var buf bytes.Buffer
	if err := runSnapshotsList(&buf); err != nil {
		t.Fatalf("runSnapshotsList: %v", err)
	}
	if !strings.Contains(buf.String(), "no snapshots found") {
		t.Errorf("expected 'no snapshots found', got: %s", buf.String())
	}
}

// TestSnapshotsListPopulated lists created snapshots with a count.
func TestSnapshotsListPopulated(t *testing.T) {
	snapshotHome(t)
	orig := filepath.Join(os.Getenv("HOME"), "f.txt")
	os.WriteFile(orig, []byte("a"), 0o644)

	snapshot.Create([]string{orig})
	snapshot.Create([]string{orig})

	var buf bytes.Buffer
	if err := runSnapshotsList(&buf); err != nil {
		t.Fatalf("runSnapshotsList: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "2 snapshot(s) total") {
		t.Errorf("expected '2 snapshot(s) total', got: %s", out)
	}
}

// TestSnapshotsPruneNothingToRemove returns a noop message when under threshold.
func TestSnapshotsPruneNothingToRemove(t *testing.T) {
	snapshotHome(t)
	orig := filepath.Join(os.Getenv("HOME"), "g.txt")
	os.WriteFile(orig, []byte("a"), 0o644)
	snapshot.Create([]string{orig})

	var buf bytes.Buffer
	if err := runSnapshotsPrune(10, &buf); err != nil {
		t.Fatalf("runSnapshotsPrune: %v", err)
	}
	if !strings.Contains(buf.String(), "nothing to prune") {
		t.Errorf("expected 'nothing to prune', got: %s", buf.String())
	}
}

// TestSnapshotsPruneRemovesOld prunes beyond keep and reports the removed IDs.
func TestSnapshotsPruneRemovesOld(t *testing.T) {
	snapshotHome(t)
	orig := filepath.Join(os.Getenv("HOME"), "h.txt")
	os.WriteFile(orig, []byte("a"), 0o644)

	for i := 0; i < 4; i++ {
		if _, err := snapshot.Create([]string{orig}); err != nil {
			t.Fatalf("snapshot.Create #%d: %v", i, err)
		}
	}

	var buf bytes.Buffer
	if err := runSnapshotsPrune(2, &buf); err != nil {
		t.Fatalf("runSnapshotsPrune: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "pruned 2 snapshot(s)") {
		t.Errorf("expected 'pruned 2 snapshot(s)', got: %s", out)
	}

	ids, _ := snapshot.List()
	if len(ids) != 2 {
		t.Errorf("after prune, expected 2 remaining, got %d", len(ids))
	}
}
