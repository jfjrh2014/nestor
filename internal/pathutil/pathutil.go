// Package pathutil holds small path helpers shared across internal packages.
package pathutil

import (
	"fmt"
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

// ResolveHome expands a leading "~" or "~/..." against home and rejects the
// "~user/..." form. When home is empty (the lookup failed) any tilde form is
// refused instead of passing through: a dest like "~/.bashrc" written
// unexpanded lands under a literal "~" directory relative to the working
// directory — silent corruption (probed: Deploy reported "deployed" while
// creating ./~/.bashrc). Plain and absolute paths pass through unchanged.
func ResolveHome(path, home string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~") && home == "" {
		return "", fmt.Errorf("cannot expand %q: home directory is unavailable (HOME not set and no user record)", path)
	}
	if IsOtherUserTilde(path) {
		return "", fmt.Errorf("%q uses the ~user/... form, which nestor does not expand (it would write into your own home)", path)
	}
	return ExpandHome(path, home), nil
}
