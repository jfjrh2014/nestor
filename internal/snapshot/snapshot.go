// Package snapshot captures the current state of dotfile destinations before
// nestor overwrites them, enabling rollback to a previous known-good state.
package snapshot

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// A Snapshot records the files that were backed up before a deploy.
type Snapshot struct {
	ID        string    `json:"id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Files     []FileRef `json:"files"`
}

// FileRef tracks the original location and the backup copy.
type FileRef struct {
	Original string `json:"original"` // where the file lives (e.g. ~/.gitconfig)
	Backup   string `json:"backup"`   // path inside the snapshot dir
}

// userHomeDir is a seam over os.UserHomeDir so tests can redirect the
// snapshot root without touching the real home directory.
var userHomeDir = os.UserHomeDir

// Dir returns the snapshot storage root (~/.config/nestor/snapshots).
func Dir() (string, error) {
	home, err := userHomeDir()
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
	return createIn(base, destPaths)
}

// createIn is the testable core of Create, operating against an explicit base.
func createIn(base string, destPaths []string) (*Snapshot, error) {
	id, snapDir, err := freshSnapshotDir(base)
	if err != nil {
		return nil, err
	}

	snap := &Snapshot{
		ID:        id,
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
	manifestDir := filepath.Dir(filepath.Join(snapDir, "manifest.json"))
	if err := os.MkdirAll(manifestDir, 0o755); err != nil {
		return nil, err
	}
	if err := writeAtomic(filepath.Join(snapDir, "manifest.json"), manifest); err != nil {
		return nil, err
	}

	// id is persisted in the manifest via Snapshot.ID; the dir name carries it too.
	return snap, nil
}

// freshSnapshotDir picks a timestamp ID that does not collide with an existing
// snapshot directory, incrementing the suffix until a free name is found.
// This guards against same-second overwrites when multiple snaps land close
// together (e.g. rapid test runs or scripted deploys).
func freshSnapshotDir(base string) (id string, dir string, err error) {
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", "", fmt.Errorf("creating snapshot base: %w", err)
	}
	id = time.Now().Format("20060102-150405")
	dir = filepath.Join(base, id)
	for i := 1; ; i++ {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				return "", "", fmt.Errorf("creating snapshot dir: %w", mkErr)
			}
			return id, dir, nil
		}
		// collision: append a sequential suffix
		id = fmt.Sprintf("%s-%d", time.Now().Format("20060102-150405"), i)
		dir = filepath.Join(base, id)
		if i > 9999 {
			return "", "", fmt.Errorf("could not allocate snapshot dir after %d attempts", i)
		}
	}
}

// List returns snapshot IDs sorted newest-first.
func List() ([]string, error) {
	base, err := Dir()
	if err != nil {
		return nil, err
	}
	return listIn(base)
}

var listIn = func(base string) ([]string, error) {
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
	return restoreIn(base, id)
}

func restoreIn(base, id string) (*Snapshot, error) {
	if id == "" {
		ids, err := listIn(base)
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

	if snap.ID != "" && snap.ID != id {
		return nil, fmt.Errorf("manifest id %q does not match snapshot dir %q", snap.ID, id)
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
	return deleteIn(base, id)
}

func deleteIn(base, id string) error {
	return os.RemoveAll(filepath.Join(base, id))
}

// Prune keeps the `keep` most recent snapshots and removes the rest, returning
// the IDs of the snapshots it deleted. If keep <= 0 it defaults to keeping all
// snapshots (no-op). It never removes a snapshot that does not appear in List,
// so there is no risk of deleting a directory injected out-of-band.
func Prune(keep int) ([]string, error) {
	base, err := Dir()
	if err != nil {
		return nil, err
	}
	return pruneIn(base, keep)
}

// pruneIn keeps the `keep` most recent snapshots that actually exist and
// removes the rest, returning the real IDs deleted. IDs that vanish from disk
// between the listing and the delete (moved, removed out-of-band by a test or
// a race) are ignored: computing the cut from those phantoms would shift it
// into snapshots the user asked to keep.
func pruneIn(base string, keep int) ([]string, error) {
	if keep <= 0 {
		return nil, nil
	}
	ids, err := listIn(base)
	if err != nil {
		return nil, err
	}
	kept := 0
	var removed []string
	for _, id := range ids { // newest-first
		if _, err := os.Stat(filepath.Join(base, id)); err != nil {
			if os.IsNotExist(err) {
				continue // vanished out-of-band: not ours to prune or count
			}
			return removed, fmt.Errorf("stat snapshot %s: %w", id, err)
		}
		if kept < keep {
			kept++
			continue
		}
		if err := os.RemoveAll(filepath.Join(base, id)); err != nil {
			return removed, fmt.Errorf("delete snapshot %s: %w", id, err)
		}
		removed = append(removed, id)
	}
	return removed, nil
}

// copyFile copies src to dest, preserving file mode. It checks the close
// error on the destination so that write failures (e.g. disk full) are not
// silently turned into truncated backups.
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

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	// preserve mode
	info, _ := in.Stat()
	if info != nil {
		_ = out.Chmod(info.Mode())
	}
	// Close reports deferred write errors (disk full, quota) — check it.
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}

// writeAtomic writes data to path, returning an error if the final close fails.
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := out.Write(data); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return nil
}

// sanitizePath turns an absolute path like /home/user/.config/nestor/foo.conf
// into a flat relative path safe for a backup dir: home+user+.config+nestor+foo.conf
//
// The full path is escaped so that dest files sharing a basename but living in
// different directories (e.g. ~/.gitconfig and ~/work/.gitconfig) do not clobber
// each other inside the snapshot dir.
func sanitizePath(p string) string {
	// Clean the path, strip any volume letter on Windows, then replace every
	// OS path separator with '+' so the result is a single flat file name.
	clean := filepath.Clean(p)
	clean = strings.TrimPrefix(clean, "/")
	if len(clean) >= 2 && clean[1] == ':' {
		// Windows drive letter (C:foo) — drop the colon
		clean = string(clean[0]) + clean[2:]
	}
	// Replace remaining separators. Empty result (root path) -> "root".
	flat := strings.ReplaceAll(clean, string(filepath.Separator), "+")
	if flat == "" {
		flat = "root"
	}
	return flat + ".bak"
}

func expandHome(p string) string {
	if p == "" || p[0] != '~' {
		return p
	}
	home, err := userHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, p[1:])
}
