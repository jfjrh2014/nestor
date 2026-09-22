// Package fsutil holds file-write primitives shared by the config, sync,
// snapshot and restore paths.
package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileSync writes data to path durably. It creates a temp file in the
// destination's directory, writes and fsyncs the data, fsyncs the directory,
// then renames the temp file over the destination. A crash or an error at any
// point leaves the destination holding either the previous contents or the
// complete new contents — never a truncated half-file. On any failure the
// temp file is removed and the destination is left byte-for-byte unchanged.
//
// perm is applied to the new file regardless of umask (CreateTemp starts at
// 0600; a chmod error is a real error, matching the copy paths).
func WriteFileSync(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	// Matches the historical writers: 'sync' created ~/.config/nestor on
	// first use and the snapshot writer created nested backup dirs. Doing it
	// here keeps that guarantee for every caller.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".nestor-write-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	success := false
	defer func() {
		if !success {
			tmp.Close()
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return err
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	success = true

	// Best-effort: fsyncing the directory makes the rename itself durable.
	// If it fails the new contents are already visible at path, so the
	// caller has nothing to recover — reporting an error here would claim a
	// failed write over a file that was in fact updated.
	if d, derr := os.Open(dir); derr == nil {
		d.Sync()
		d.Close()
	}
	return nil
}

// CopyFileSync copies src to dest durably, preserving src's mode. It reads
// src, then hands off to WriteFileSync so the same temp+fsync+rename
// guarantee applies: a crash or an error at any point leaves the destination
// holding either the previous contents or the complete new copy — never a
// truncated half-file. This matters most on restore, where dest is the only
// surviving copy of a file (the backup at src can be re-read to retry).
//
// On any failure the temp file is removed and the destination is left
// byte-for-byte unchanged.
func CopyFileSync(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat src: %w", err)
	}
	return WriteFileSync(dest, data, info.Mode().Perm())
}

// WritePerm resolves the mode a write to path should use: an existing file
// keeps its own mode — a later chmod of the dest must survive rewrites, and
// a rewrite must never widen a private file — while a not-yet-existing file
// gets fallbackPerm. Any other stat error is returned.
func WritePerm(path string, fallbackPerm os.FileMode) (os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fallbackPerm, nil
		}
		return 0, err
	}
	return info.Mode().Perm(), nil
}
