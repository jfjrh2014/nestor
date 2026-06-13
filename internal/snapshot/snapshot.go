// Package snapshot captures the current state of dotfile destinations before
// nestor overwrites them, enabling rollback to a previous known-good state.
package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// A Snapshot records the files that were backed up before a deploy.
type Snapshot struct {
	CreatedAt time.Time `json:"created_at"`
	Files     []FileRef `json:"files"`
}

// FileRef tracks the original location and the backup copy.
type FileRef struct {
	Original string `json:"original"` // where the file lives (e.g. ~/.gitconfig)
	Backup   string `json:"backup"`   // path inside the snapshot dir
}

// Dir returns the snapshot storage root (~/.config/nestor/snapshots).
func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "nestor", "snapshots"), nil
}

// Create backs up all existing files at the given paths and writes a
// manifest.json into a timestamped snapshot directory.
// Paths that don't exist are skipped silently.
func Create(destPaths []string) (*Snapshot, error) {
	base, err := Dir()
	if err != nil {
		return nil, err
	}

	id := time.Now().Format("20060102-150405")
	snapDir := filepath.Join(base, id)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating snapshot dir: %w", err)
	}

	snap := &Snapshot{
		CreatedAt: time.Now().UTC(),
		Files:     make([]FileRef, 0, len(destPaths)),
	}

	for _, p := range destPaths {
		p = expandHome(p)
		if _, err := os.Stat(p); err != nil {
			// doesn't exist yet — nothing to back up
			continue
		}
		rel := sanitizePath(p)
		backup := filepath.Join(snapDir, rel)
		if err := os.MkdirAll(filepath.Dir(backup), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir backup %s: %w", backup, err)
		}
		if err := copyFile(p, backup); err != nil {
			return nil, fmt.Errorf("backup %s: %w", p, err)
		}
		snap.Files = append(snap.Files, FileRef{Original: p, Backup: rel})
	}

	// write manifest
	manifest, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(snapDir, "manifest.json"), manifest, 0o644); err != nil {
		return nil, err
	}

	return snap, nil
}

// List returns snapshot IDs sorted newest-first.
func List() ([]string, error) {
	base, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	// sorted newest first (dir names are timestamps)
	var ids []string
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].IsDir() {
			ids = append(ids, entries[i].Name())
		}
	}
	return ids, nil
}

// Restore copies backed-up files back to their original locations.
// Pass an empty id to restore the most recent snapshot.
func Restore(id string) (*Snapshot, error) {
	base, err := Dir()
	if err != nil {
		return nil, err
	}

	if id == "" {
		ids, err := List()
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("no snapshots found")
		}
		id = ids[0]
	}

	snapDir := filepath.Join(base, id)
	data, err := os.ReadFile(filepath.Join(snapDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("reading snapshot %s: %w", id, err)
	}

	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parsing manifest: %w", err)
	}

	for _, f := range snap.Files {
		backup := filepath.Join(snapDir, f.Backup)
		if err := copyFile(backup, f.Original); err != nil {
			return nil, fmt.Errorf("restore %s: %w", f.Original, err)
		}
	}

	return &snap, nil
}

// Delete removes a snapshot directory.
func Delete(id string) error {
	base, err := Dir()
	if err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(base, id))
}

func copyFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// preserve mode
	info, _ := in.Stat()
	if info != nil {
		out.Chmod(info.Mode())
	}
	return nil
}

// sanitizePath turns an absolute path like /home/user/.gitconfig
// into a flat relative path safe for a backup dir: home_user_.gitconfig
func sanitizePath(p string) string {
	return filepath.Base(p) + ".bak"
}

func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}
