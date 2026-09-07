package vcs

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitAvailable reports whether the host has git and skips if absent.
func gitAvailable(t *testing.T) bool {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not installed — skipping vcs tests")
		return false
	}
	return true
}

func TestHasGit(t *testing.T) {
	// Should reflect reality on this host
	got := HasGit()
	if _, err := exec.LookPath("git"); err == nil && !got {
		t.Error("HasGit() = false, expected true (git is on PATH)")
	}
	if _, err := exec.LookPath("git"); err != nil && got {
		t.Error("HasGit() = true, expected false (git not on PATH)")
	}
}

func TestInitCreatesRepo(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !IsRepo(dir) {
		t.Error("directory should be a git repo after Init")
	}

	// .gitignore should be created
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); err != nil {
		t.Errorf(".gitignore not created: %v", err)
	}

	// .gitignore content should mention snapshots
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), "snapshots/") {
		t.Errorf(".gitignore should exclude snapshots/, got: %s", data)
	}
}

func TestInitIdempotent(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	if err := Init(dir); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
}

func TestInitPreservesExistingGitignore(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	custom := "# my custom gitignore\nnode_modules/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteGitignore(dir); err != nil {
		t.Fatalf("WriteGitignore failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if string(data) != custom {
		t.Errorf("existing .gitignore was overwritten: got %q", data)
	}
}

func TestSetGetRemote(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	got := GetRemote(dir, "origin")
	if got != "" {
		t.Errorf("expected empty remote, got %q", got)
	}

	if err := SetRemote(dir, "origin", "https://github.com/user/dotfiles.git"); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}

	got = GetRemote(dir, "origin")
	if got != "https://github.com/user/dotfiles.git" {
		t.Errorf("expected remote URL, got %q", got)
	}

	if !RemoteSet(dir, "origin") {
		t.Error("RemoteSet should report origin exists")
	}
	if RemoteSet(dir, "nonexistent") {
		t.Error("RemoteSet should report nonexistent missing")
	}
}

func TestSetRemoteReplacesUrl(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if err := SetRemote(dir, "origin", "https://github.com/old/repo.git"); err != nil {
		t.Fatalf("first SetRemote failed: %v", err)
	}
	if err := SetRemote(dir, "origin", "https://github.com/new/repo.git"); err != nil {
		t.Fatalf("second SetRemote failed: %v", err)
	}

	got := GetRemote(dir, "origin")
	if got != "https://github.com/new/repo.git" {
		t.Errorf("expected updated URL, got %q", got)
	}
}

func TestStatusCleanRepo(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Initial commit to make the tree clean
	if err := os.WriteFile(filepath.Join(dir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, "initial"); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}

	s, m, u, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if s+m+u != 0 {
		t.Errorf("expected clean tree, got staged=%d modified=%d untracked=%d", s, m, u)
	}

	has, err := HasChanges(dir)
	if err != nil {
		t.Fatalf("HasChanges failed: %v", err)
	}
	if has {
		t.Error("HasChanges = true, expected false")
	}
}

func TestStatusDetectsUntracked(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := Commit(dir, "initial"); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	// add untracked file
	if err := os.WriteFile(filepath.Join(dir, "new.yml"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, m, u, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if u != 1 {
		t.Errorf("expected 1 untracked, got (staged=%d modified=%d untracked=%d)", s, m, u)
	}
}

func TestStatusDetectsModified(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, "initial"); err != nil {
		t.Fatalf("initial commit failed: %v", err)
	}

	// modify tracked file
	if err := os.WriteFile(path, []byte("version: 1\npackages:\n  common:\n    - git\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, m, u, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if m != 1 {
		t.Errorf("expected 1 modified, got (staged=%d modified=%d untracked=%d)", s, m, u)
	}
}

func TestCommitCleanTreeNoOp(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := Commit(dir, "initial"); err != nil {
		t.Fatalf("first Commit failed: %v", err)
	}

	if err := Commit(dir, "noop"); err != nil {
		t.Fatalf("second Commit on clean tree failed: %v", err)
	}
}

// initRemoteRepo creates a bare repo to use as a push/pull remote and
// returns its path. Skips the test if git is unavailable.
func initRemoteRepo(t *testing.T) string {
	t.Helper()
	if !gitAvailable(t) {
		return ""
	}
	remote := filepath.Join(t.TempDir(), "remote.git")
	out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput()
	if err != nil {
		t.Skipf("git init --bare failed: %v (%s) — skipping push/pull test", err, out)
		return ""
	}
	return remote
}

// commitFile writes a file into dir and commits it.
func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, message); err != nil {
		t.Fatalf("Commit(%q) failed: %v", message, err)
	}
}

func TestPushToBareRemote(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	commitFile(t, dir, "nestor.yml", "version: 1\n", "initial")

	if err := SetRemote(dir, "origin", remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Push(dir, "origin"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	// The bare remote should now contain our commit message.
	out, err := exec.Command("git", "-C", remote, "log", "--oneline", "HEAD").Output()
	if err != nil {
		t.Fatalf("git log on remote failed: %v", err)
	}
	if !strings.Contains(string(out), "initial") {
		t.Errorf("expected 'initial' commit on remote, got: %s", out)
	}
}

func TestPullFromRemote(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	// Seed the remote with a commit via a first clone.
	seed := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, seed).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}
	commitFile(t, seed, "shared.yml", "first\n", "seed commit")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("seed Push failed: %v", err)
	}

	// Second clone should already have the seed commit.
	other := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}
	if _, err := os.Stat(filepath.Join(other, "shared.yml")); err != nil {
		t.Fatalf("clone should contain shared.yml: %v", err)
	}

	// Push a new commit from the seed clone, then Pull it into the other.
	commitFile(t, seed, "second.yml", "second\n", "second commit")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("second Push failed: %v", err)
	}
	if err := Pull(other, "origin"); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, "second.yml")); err != nil {
		t.Errorf("second.yml missing after Pull: %v", err)
	}
}

func TestStatusErrorOutsideRepo(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	plain := t.TempDir() // no git init
	if _, _, _, err := Status(plain); err == nil {
		t.Error("Status on non-repo should fail")
	}
}

func TestInitMkdirFailure(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	// A regular file blocks MkdirAll under it.
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	under := filepath.Join(blocker, "repo")
	if err := Init(under); err == nil {
		t.Error("Init under a file-blocker should fail")
	}
}

func TestInitGitFailure(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	// `git init` on a path whose .git is a regular file (not a valid gitfile) fails.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(dir); err == nil {
		t.Error("Init with a file at .git should fail")
	}
}

func TestCommitAddFailure(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, "doomed"); err == nil {
		t.Error("Commit on a broken repo should fail")
	}
}

func TestCommitCreatesHistory(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	path := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Commit(dir, "initial"); err != nil {
		t.Fatalf("initial Commit failed: %v", err)
	}

	// Verify a commit exists
	out, err := exec.Command("git", "-C", dir, "log", "--oneline").Output()
	if err != nil {
		t.Fatalf("git log failed: %v", err)
	}
	if !strings.Contains(string(out), "initial") {
		t.Errorf("expected commit 'initial' in log, got: %s", out)
	}
}

// setBranchNames sets the branch name a fresh `git init` gets on this host,
// so tests can simulate the master-vs-main mismatch between machines.
func setBranchNames(t *testing.T, name string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if out, err := exec.Command("git", "config", "--global", "init.defaultBranch", name).CombinedOutput(); err != nil {
		t.Fatalf("setting init.defaultBranch: %v (%s)", err, out)
	}
}

// TestBranchUnbornAndBorn pins the Branch helper contract on both sides of
// the first commit: before it, HEAD names the default branch (often master
// locally while GitHub remotes use main); after it, HEAD names the branch
// that was committed to. Non-repos error.
func TestBranchUnbornAndBorn(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	setBranchNames(t, "mastervalue")

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	got, err := Branch(dir)
	if err != nil {
		t.Fatalf("Branch on unborn repo: %v", err)
	}
	if got != "mastervalue" {
		t.Errorf("Branch(unborn) = %q, want %q", got, "mastervalue")
	}

	commitFile(t, dir, "nestor.yml", "version: 1\n", "initial")
	got, err = Branch(dir)
	if err != nil {
		t.Fatalf("Branch on born repo: %v", err)
	}
	if got != "mastervalue" {
		t.Errorf("Branch(born) = %q, want %q", got, "mastervalue")
	}

	if _, err := Branch(filepath.Join(dir, "missing")); err == nil {
		t.Error("Branch on a non-repo should fail")
	}
}

// TestPushPullAlignUnbornBranchWithRemote is the session #71 regression:
// machine A pushes a master-branch repo into an empty remote whose HEAD
// says main. Under the old HEAD refspec, machine B's `nestor pull` failed
// outright ("couldn't find remote ref HEAD") and pushes silently forked
// the sync. Now B adopts the remote's only branch before its first pull
// (a dangling remote HEAD over one branch is unambiguous), so both
// machines work the same branch whichever name the first pusher used.
func TestPushPullAlignUnbornBranchWithRemote(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	setBranchNames(t, "masterlocal")
	// GitHub forces HEAD to main on new empty repos regardless of what the
	// pushing client calls its branch — model that explicitly, since the
	// bare init above just inherited the same default as the machines.
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD to main: %v (%s)", err, out)
	}
	// Machine A: the exact composition the push command uses — align the
	// unborn branch right after SetRemote, BEFORE the first commit bakes
	// the branch name in.
	machineA := t.TempDir()
	if err := Init(machineA); err != nil {
		t.Fatalf("Init A failed: %v", err)
	}
	if err := SetRemote(machineA, "origin", remote); err != nil {
		t.Fatalf("SetRemote A failed: %v", err)
	}
	if err := EnsureUnbornAligned(machineA, "origin"); err != nil {
		t.Fatalf("EnsureUnbornAligned A failed: %v", err)
	}
	commitFile(t, machineA, "nestor.yml", "config: FROM-A\n", "initial")
	if err := Push(machineA, "origin"); err != nil {
		t.Fatalf("Push A failed: %v", err)
	}

	// First pusher wins: the remote holds exactly one branch, A's default.
	remoteBranch := RemoteDefaultBranch(machineA, remote)
	if remoteBranch != "masterlocal" {
		t.Fatalf("remote default branch = %q, want masterlocal", remoteBranch)
	}
	out, err := exec.Command("git", "-C", remote, "show", remoteBranch+":nestor.yml").Output()
	if err != nil || strings.TrimSpace(string(out)) != "config: FROM-A" {
		t.Errorf("remote %s should hold A's config (got %q, err %v)", remoteBranch, string(out), err)
	}

	// Machine B: fresh repo, adopts the remote's name, finds A's commit.
	machineB := t.TempDir()
	if err := Init(machineB); err != nil {
		t.Fatalf("Init B failed: %v", err)
	}
	if err := SetRemote(machineB, "origin", remote); err != nil {
		t.Fatalf("SetRemote B failed: %v", err)
	}
	if err := Pull(machineB, "origin"); err != nil {
		t.Fatalf("Pull B failed: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(machineB, "nestor.yml"))
	if err != nil {
		t.Fatalf("pulled config missing on B: %v", err)
	}
	if strings.TrimSpace(string(data)) != "config: FROM-A" {
		t.Errorf("B pulled %q, want A's config", string(data))
	}
	bb, err := Branch(machineB)
	if err != nil || bb != remoteBranch {
		t.Errorf("B on branch %q (err %v), want remote default %q", bb, err, remoteBranch)
	}

	// And the sync stays intact: B pushes a commit, A pulls it.
	commitFile(t, machineB, "second.yml", "second\n", "second commit")
	if err := Push(machineB, "origin"); err != nil {
		t.Fatalf("Push B failed: %v", err)
	}
	if err := Pull(machineA, "origin"); err != nil {
		t.Fatalf("Pull A failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(machineA, "second.yml")); err != nil {
		t.Errorf("A missing second.yml after pull: %v", err)
	}
}

// TestPullUnbornEmptyRemoteStillFails pins the honest-error path: with
// nothing on the remote there is no branch to adopt and no history to
// merge, so the pull must still fail loudly rather than claim success.
func TestPullUnbornEmptyRemoteStillFails(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := SetRemote(dir, "origin", remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Pull(dir, "origin"); err == nil {
		t.Error("Pull from an empty remote should fail")
	}
}
