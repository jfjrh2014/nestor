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

// IsSelfInject reports whether path points at nestor's own config file
// (nestor.yml) once resolved against home. Injecting a secret INTO the
// config is corruption by definition: every injection rewrites nestor.yml
// with a rendered "key: value" line that the YAML has no schema for, and
// the next config load bricks every nestor command. Tilde and plain forms
// are compared in their expanded shape, so "~/.config/nestor/nestor.yml"
// and the absolute path are caught alike. Shared by config validation,
// 'nestor add' prompting, and the inject engine (defense in depth).
func IsSelfInject(path, home string) bool {
	if path == "" {
		return false
	}
	expanded := ExpandHome(path, home)
	target := filepath.Join(home, ".config", "nestor", "nestor.yml")
	return expanded == target
}

// IsManagedDotfileDest reports whether path equals one of the given
// dotfile destinations, comparing in the expanded shape so "~/.bashrc" and
// the absolute path collide. A secret injected INTO a managed dotfile never
// survives: templates render with no secret data, so the next deploy
// overwrites the injected value silently. Used by config validation to
// refuse the mapping at load time.
func IsManagedDotfileDest(path string, dests []string, home string) bool {
	if path == "" || len(dests) == 0 {
		return false
	}
	expanded := ExpandHome(path, home)
	for _, d := range dests {
		if ExpandHome(d, home) == expanded {
			return true
		}
	}
	return false
}

// SamePath reports whether two dest spellings refer to the same file,
// comparing in the expanded shape so "~/.bashrc" and the absolute path
// collide. Duplicate detection keyed on raw strings let both spellings
// coexist: validate passed, two templates deployed to one file last-wins,
// and the first deploy re-spelled the second entry's dest. Shared by config
// validation, 'nestor add' duplicate checks, and sync's merge dedup.
func SamePath(a, b, home string) bool {
	return ExpandHome(a, home) == ExpandHome(b, home)
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
