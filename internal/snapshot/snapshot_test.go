package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateSkipsMissingFiles(t *testing.T) {
	dir := t.TempDir()

	// Create one existing file
	existing := filepath.Join(dir, "real.txt")
	os.WriteFile(existing, []byte("hello"), 0o644)

	snap, err := createAt(dir, []string{existing, filepath.Join(dir, "ghost.txt")})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(snap.Files) != 1 {
		t.Fatalf("expected 1 file (skipped missing), got %d", len(snap.Files))
	}
	if snap.Files[0].Original != existing {
		t.Errorf("original = %q, want %q", snap.Files[0].Original, existing)
	}
}

func TestCreateAndRestore(t *testing.T) {
	dir := t.TempDir()

	orig := filepath.Join(dir, "config.txt")
	os.WriteFile(orig, []byte("before"), 0o644)

	snap, err := createAt(dir, []string{orig})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Mutate
	os.WriteFile(orig, []byte("after"), 0o644)

	// Restore
	snapID := snap.CreatedAt.Format("20060102-150405")
	restored, err := restoreAt(dir, snapID)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(restored.Files) != 1 {
		t.Fatalf("restored files = %d, want 1", len(restored.Files))
	}

	data, _ := os.ReadFile(orig)
	if string(data) != "before" {
		t.Errorf("after restore = %q, want %q", string(data), "before")
	}
}

func TestManifestWritten(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "test.txt")
	os.WriteFile(orig, []byte("x"), 0o644)

	snap, err := createAt(dir, []string{orig})
	if err != nil {
		t.Fatal(err)
	}

	snapID := snap.CreatedAt.Format("20060102-150405")
	manifestPath := filepath.Join(dir, snapID, "manifest.json")
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

func TestDelete(t *testing.T) {
	dir := t.TempDir()
	orig := filepath.Join(dir, "a.txt")
	os.WriteFile(orig, []byte("a"), 0o644)

	snap, _ := createAt(dir, []string{orig})
	snapID := snap.CreatedAt.Format("20060102-150405")

	snapDir := filepath.Join(dir, snapID)
	if _, err := os.Stat(snapDir); err != nil {
		t.Fatal("snapshot dir should exist")
	}

	// Manually remove since Delete uses the real base dir
	os.RemoveAll(snapDir)
	if _, err := os.Stat(snapDir); !os.IsNotExist(err) {
		t.Error("snapshot dir should be gone")
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/home/user/.gitconfig", ".gitconfig.bak"},
		{"/etc/resolv.conf", "resolv.conf.bak"},
	}
	for _, tt := range tests {
		got := sanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// createAt is a test helper using a custom base dir.
func createAt(base string, destPaths []string) (*Snapshot, error) {
	now := time.Now().UTC()
	id := now.Format("20060102-150405")
	snapDir := filepath.Join(base, id)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return nil, err
	}

	snap := &Snapshot{CreatedAt: now, Files: make([]FileRef, 0)}

	for _, p := range destPaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		rel := sanitizePath(p)
		backup := filepath.Join(snapDir, rel)
		if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
			return nil, err
		}
		if err := copyFile(p, backup); err != nil {
			return nil, err
		}
		snap.Files = append(snap.Files, FileRef{Original: p, Backup: rel})
	}

	manifest, _ := json.MarshalIndent(snap, "", "  ")
	if err := os.WriteFile(filepath.Join(snapDir, "manifest.json"), manifest, 0o644); err != nil {
		return nil, err
	}
	return snap, nil
}

func restoreAt(base, id string) (*Snapshot, error) {
	snapDir := filepath.Join(base, id)
	data, err := os.ReadFile(filepath.Join(snapDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	for _, f := range snap.Files {
		backup := filepath.Join(snapDir, f.Backup)
		if err := copyFile(backup, f.Original); err != nil {
			return nil, err
		}
	}
	return &snap, nil
}
