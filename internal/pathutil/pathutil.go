// Package pathutil holds small path helpers shared across internal packages.
package pathutil

import (
	"path/filepath"
	"strings"
)

// ExpandHome resolves a leading "~" or "~/..." against the given home
// directory. Anything else — including "~user/..." and "~shared/..." — is
// returned unchanged: only the current user's home is expandable, and
// silently re-rooting "~root/.bashrc" at the wrong home is how dotfiles end
// up in the wrong place. An empty home (lookup failed) also leaves the path
// unchanged rather than re-rooting it at a relative path.
func ExpandHome(path, home string) string {
	if path == "" || home == "" {
		return path
	}
	if path == "~" {
		return filepath.Clean(home)
	}
	for _, sep := range []string{"~/", "~" + string(filepath.Separator)} {
		if strings.HasPrefix(path, sep) {
			return filepath.Join(home, path[len(sep):])
		}
	}
	return path
}

// IsOtherUserTilde reports whether path uses the "~user/..." tilde form,
// which nestor does not expand: it would silently write into the current
// user's home instead of the named user's. "~", "~/..." and everything
// without a leading tilde are not foreign.
func IsOtherUserTilde(path string) bool {
	if !strings.HasPrefix(path, "~") || path == "~" {
		return false
	}
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return false
	}
	return true
}
