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

// Branch returns the name of the branch HEAD points at. Works on repos with
// no commits yet (unborn HEAD), where rev-parse cannot resolve. Returns ""
// (nil error) when HEAD is detached; an error when dir is not a git repo.
func Branch(dir string) (string, error) {
	if !HasGit() {
		return "", ErrGitNotFound
	}
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "--short", "HEAD").Output()
	if err != nil {
		if !IsRepo(dir) {
			return "", fmt.Errorf("not a git repository: %s", dir)
		}
		return "", nil // detached HEAD
	}
	return strings.TrimSpace(string(out)), nil
}

// unborn reports whether HEAD names a branch that has no commits yet.
func unborn(dir string) bool {
	return exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "HEAD").Run() != nil
}

// IsUnborn reports whether HEAD names a branch that has no commits yet —
// the state of a freshly-initialized repo before its first commit.
func IsUnborn(dir string) bool {
	return unborn(dir)
}

// RemoteDefaultBranch returns the branch remote pulls should read from and
// unborn pushes should adopt: the remote's advertised HEAD branch when it
// resolves, else the remote's only branch — a dangling HEAD over exactly
// one real branch is unambiguous, and is the shape of a repo whose default
// was never moved off main after a client pushed differently-named work,
// and of fresh local bare clones. Returns "" when the remote is empty,
// unreachable, or genuinely ambiguous.
func RemoteDefaultBranch(dir, remote string) string {
	out, err := exec.Command("git", "-C", dir, "ls-remote", "--symref", remote, "HEAD").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "ref:" && strings.HasPrefix(fields[1], "refs/heads/") {
				return strings.TrimPrefix(fields[1], "refs/heads/")
			}
		}
	}
	heads, err := exec.Command("git", "-C", dir, "ls-remote", "--heads", remote).Output()
	if err != nil {
		return ""
	}
	var names []string
	for _, line := range strings.Split(string(heads), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.HasPrefix(fields[1], "refs/heads/") {
			names = append(names, strings.TrimPrefix(fields[1], "refs/heads/"))
		}
	}
	if len(names) == 1 {
		return names[0]
	}
	return ""
}

// alignUnbornWithRemote points an unborn local branch at the remote's
// default branch name before the first pull or push. Both machines must
// work on the same branch: git inits default to different names (master
// locally vs main on GitHub), and a bare HEAD refspec then splits the sync
// across two branches — pulls fail while pushes silently fork the history.
// Repos with commits are left untouched: their branch is the user's choice.
func alignUnbornWithRemote(dir, localBranch, remote string) error {
	if !unborn(dir) {
		return nil
	}
	remoteBranch := RemoteDefaultBranch(dir, remote)
	if remoteBranch == "" || remoteBranch == localBranch {
		return nil
	}
	// HEAD is unborn, so re-pointing it moves nothing.
	out, err := exec.Command("git", "-C", dir, "symbolic-ref", "HEAD", "refs/heads/"+remoteBranch).CombinedOutput()
	if err != nil {
		return fmt.Errorf("aligning with remote branch %q: %w (%s)", remoteBranch, err, strings.TrimSpace(string(out)))
	}
	return nil
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

// EnsureUnbornAligned points an unborn local branch at the remote's
// default branch name before the first commit/push/pull. Both machines must
// work on the same branch: git inits default to different names (master
// locally vs main on GitHub), and a bare HEAD refspec then splits the sync
// across two branches — pulls fail while pushes silently fork the history.
// Repos with commits are left untouched: their branch is the user's choice.
// Call this after the remote is configured but before committing or
// syncing; by the time a first commit exists, the branch name is already
// baked in.
func EnsureUnbornAligned(dir, remote string) error {
	branch, err := Branch(dir)
	if err != nil {
		return err
	}
	return alignUnbornWithRemote(dir, branch, remote)
}

// resolveBranch returns the branch push/pull should act on, aligning an
// unborn local branch with the remote's default branch name first.
func resolveBranch(dir, remote string) (string, error) {
	if err := EnsureUnbornAligned(dir, remote); err != nil {
		return "", err
	}
	branch, err := Branch(dir)
	if err != nil {
		return "", err
	}
	if branch == "" {
		return "", fmt.Errorf("HEAD is detached — checkout a branch before syncing config")
	}
	return branch, nil
}

// Push pushes the current branch to the named remote and sets upstream.
// Before the first push, an unborn local branch adopts the remote's default
// branch name so both ends of the sync work on the same branch.
func Push(dir, remote string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	branch, err := resolveBranch(dir, remote)
	if err != nil {
		return err
	}
	cmd := exec.Command("git", "-C", dir, "push", "-u", remote, branch)
	// Connect stdin to allow credential prompts if needed
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Pull fetches and merges the current branch from the named remote. Before
// the first pull, an unborn local branch adopts the remote's default branch
// name so the fetched history lands on the branch both machines use, and
// gets an empty baseline commit so the merge has a common ancestor. The
// merge allows unrelated histories: nestor owns both ends of the config
// repo, so a machine's first pull is unrelated by definition, never a
// user mistake worth blocking on.
func Pull(dir, remote string) error {
	if !HasGit() {
		return ErrGitNotFound
	}
	branch, err := resolveBranch(dir, remote)
	if err != nil {
		return err
	}
	if unborn(dir) {
		if err := Commit(dir, "nestor: baseline before first pull"); err != nil {
			return fmt.Errorf("baseline commit before first pull: %w", err)
		}
	}
	// The merge may need a commit (first pull, divergent edits); give it the
	// same identity fallback Commit uses so virgin machines don't die on
	// "please tell me who you are".
	cmd := exec.Command("git", "-C", dir, "pull", "--allow-unrelated-histories", "--no-rebase", remote, branch)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=nestor",
		"GIT_AUTHOR_EMAIL=nestor@local",
		"GIT_COMMITTER_NAME=nestor",
		"GIT_COMMITTER_EMAIL=nestor@local",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
