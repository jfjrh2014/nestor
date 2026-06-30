// Package vcs handles git-based version control for the nestor config
// directory. It delegates to the system git binary, the same pattern
// secrets uses to delegate to provider CLIs.
package vcs

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrGitNotFound is returned when the git binary is not on PATH.
var ErrGitNotFound = errors.New("git not found on PATH — install git to use config sync")

// HasGit reports whether the git binary is available.
func HasGit() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// IsRepo reports whether dir is inside a git working tree.
func IsRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	return cmd.Run() == nil
}

// Init creates a git repo in dir if one does not yet exist.
// It also writes a .gitignore that excludes local-only artifacts.
func Init(dir string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	if IsRepo(dir) {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating repo dir: %w", err)
	}
	if out, err := exec.Command("git", "init", dir).CombinedOutput(); err != nil {
		return fmt.Errorf("git init: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return WriteGitignore(dir)
}

// WriteGitignore creates a .gitignore in dir that excludes files that are
// local-only and should not be shared across machines.
func WriteGitignore(dir string) error {
	gitignore := `# nestor local-only files
snapshots/
*.local.yml
*.local.yaml
secrets.env
*.key
*.pem
`
	path := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(path); err == nil {
		return nil // already exists, don't overwrite
	}
	return os.WriteFile(path, []byte(gitignore), 0o644)
}

// RemoteSet reports whether a remote with the given name exists.
func RemoteSet(dir, name string) bool {
	cmd := exec.Command("git", "-C", dir, "remote", "get-url", name)
	return cmd.Run() == nil
}

// SetRemote sets (or replaces) the URL for a named remote.
func SetRemote(dir, name, url string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	if RemoteSet(dir, name) {
		out, err := exec.Command("git", "-C", dir, "remote", "set-url", name, url).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git remote set-url: %w (%s)", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	out, err := exec.Command("git", "-C", dir, "remote", "add", name, url).CombinedOutput()
	if err != nil {
		return fmt.Errorf("git remote add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// GetRemote returns the URL for a named remote, or "" if not set.
func GetRemote(dir, name string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", name).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// Status returns the count of staged, modified, and untracked paths.
// Returns (staged, modified, untracked, error).
func Status(dir string) (staged, modified, untracked int, err error) {
	if !HasGit() {
		return 0, 0, 0, ErrGitNotFound
	}
	// porcelain v1: first two chars are XY status codes.
	// X = staged, Y = worktree.
	out, runErr := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if runErr != nil {
		return 0, 0, 0, fmt.Errorf("git status: %w", runErr)
	}
	// NOTE: do NOT strings.TrimSpace the whole output. Porcelain's first
	// column is a space for "modified-not-staged" entries; trimming the whole
	// string strips that leading space and flips modified entries into staged.
	// Split on newlines first, then trim each line's trailing \r only.
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 1 {
			continue
		}
		x := byte(' ')
		y := byte(' ')
		if len(line) >= 1 {
			x = line[0]
		}
		if len(line) >= 2 {
			y = line[1]
		}
		// Untracked: "??"
		if x == '?' && y == '?' {
			untracked++
			continue
		}
		// Modified in worktree (not staged): " M"
		if x == ' ' && (y == 'M' || y == 'D') {
			modified++
			continue
		}
		// Anything with a non-space X is staged
		if x != ' ' && x != '?' {
			staged++
		}
	}
	return staged, modified, untracked, nil
}

// HasChanges reports whether there are staged, modified, or untracked files.
func HasChanges(dir string) (bool, error) {
	s, m, u, err := Status(dir)
	if err != nil {
		return false, err
	}
	return s+m+u > 0, nil
}

// Commit stages all changes and creates a commit with the given message.
// If the tree is clean, Commit is a no-op and returns nil.
func Commit(dir, message string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	// Stage everything
	if out, err := exec.Command("git", "-C", dir, "add", "-A").CombinedOutput(); err != nil {
		return fmt.Errorf("git add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	// Check if there's anything to commit after staging
	has, err := HasChanges(dir)
	if err != nil {
		return err
	}
	if !has {
		return nil // clean tree, nothing to commit
	}
	// Use GIT_*_NAME/EMAIL env vars so it works even without user.name/email
	// set in local/global git config.
	cmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=nestor",
		"GIT_AUTHOR_EMAIL=nestor@local",
		"GIT_COMMITTER_NAME=nestor",
		"GIT_COMMITTER_EMAIL=nestor@local",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Push pushes local commits to the named remote.
func Push(dir, remote string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	cmd := exec.Command("git", "-C", dir, "push", "-u", remote, "HEAD")
	// Connect stdin to allow credential prompts if needed
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Pull fetches and merges from the named remote.
func Pull(dir, remote string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	cmd := exec.Command("git", "-C", dir, "pull", remote, "HEAD")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
