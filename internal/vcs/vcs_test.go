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

// TestDanglingRemoteHEADAfterMismatchedFirstPush is the session #77
// regression: a machine whose git default (masterlocal) commits offline
// and pushes first to a GitHub-style bare remote (HEAD at main) leaves
// the remote HEAD dangling — and before the fix that state was invisible
// to nestor. Detection must flag it; over path-local transports the
// symref does not resolve, so the "no advertised ref but real branches
// exist" shape is the signal.
func TestDanglingRemoteHEADAfterMismatchedFirstPush(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	// GitHub forces HEAD to main on new empty repos regardless of what
	// the pushing client calls its branch.
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD to main: %v (%s)", err, out)
	}
	setBranchNames(t, "masterlocal")

	// Healthy when empty: zero branches is the unactionable kind.
	if dangling, advertised := DanglingRemoteHEAD(t.TempDir(), remote); dangling || advertised != "" {
		t.Fatalf("empty remote reported dangling=%v advertised=%q, want false/\"\"", dangling, advertised)
	}

	// Machine A commits offline on its own default, THEN meets the remote.
	machineA := t.TempDir()
	if err := Init(machineA); err != nil {
		t.Fatalf("Init A failed: %v", err)
	}
	commitFile(t, machineA, "nestor.yml", "config: FROM-A\n", "offline commit")
	if err := SetRemote(machineA, "origin", remote); err != nil {
		t.Fatalf("SetRemote A failed: %v", err)
	}
	if err := Push(machineA, "origin"); err != nil {
		t.Fatalf("Push A failed: %v", err)
	}

	dangling, advertised := DanglingRemoteHEAD(machineA, "origin")
	if !dangling {
		t.Fatal("mismatched first push left remote HEAD dangling, but DanglingRemoteHEAD reported healthy")
	}
	if advertised != "" {
		t.Errorf("path-local transport should not resolve the symref, got advertised %q", advertised)
	}
}

// TestHealRemoteHEADFixesPathLocalRemote replays the mismatched first
// push, heals the remote directly, and proves the cure on both consumers:
// a plain git clone checks out A's work, and a nestor pull lands on the
// same branch. Before the heal, the clone checks out nothing.
func TestHealRemoteHEADFixesPathLocalRemote(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD to main: %v (%s)", err, out)
	}
	setBranchNames(t, "masterlocal")

	machineA := t.TempDir()
	if err := Init(machineA); err != nil {
		t.Fatalf("Init A failed: %v", err)
	}
	commitFile(t, machineA, "nestor.yml", "config: FROM-A\n", "offline commit")
	if err := SetRemote(machineA, "origin", remote); err != nil {
		t.Fatalf("SetRemote A failed: %v", err)
	}
	if err := Push(machineA, "origin"); err != nil {
		t.Fatalf("Push A failed: %v", err)
	}
	if dangling, _ := DanglingRemoteHEAD(machineA, "origin"); !dangling {
		t.Fatal("expected dangling remote HEAD before heal")
	}

	// Before healing, a git-native clone of the remote checks out nothing.
	cloneBefore := filepath.Join(t.TempDir(), "clone-before")
	if out, err := exec.Command("git", "clone", "-q", remote, cloneBefore).CombinedOutput(); err != nil {
		t.Fatalf("clone before heal: %v (%s)", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(cloneBefore, "nestor.yml")); !os.IsNotExist(statErr) {
		t.Errorf("clone before heal should have no worktree files, stat err = %v", statErr)
	}

	if _, healErr := HealRemoteHEAD(machineA, "origin"); healErr != nil {
		t.Fatalf("HealRemoteHEAD: %v", healErr)
	}
	if dangling, _ := DanglingRemoteHEAD(machineA, "origin"); dangling {
		t.Error("remote HEAD still dangling after heal")
	}

	// Cure, consumer 1: a fresh git clone checks out A's work.
	cloneAfter := filepath.Join(t.TempDir(), "clone-after")
	if out, err := exec.Command("git", "clone", "-q", remote, cloneAfter).CombinedOutput(); err != nil {
		t.Fatalf("clone after heal: %v (%s)", err, out)
	}
	data, err := os.ReadFile(filepath.Join(cloneAfter, "nestor.yml"))
	if err != nil {
		t.Fatalf("clone after heal missing nestor.yml: %v", err)
	}
	if strings.TrimSpace(string(data)) != "config: FROM-A" {
		t.Errorf("clone after heal holds %q", string(data))
	}

	// Cure, consumer 2: nestor pull on a fresh machine lands on A's branch.
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
	bb, err := Branch(machineB)
	if err != nil || bb != "masterlocal" {
		t.Errorf("B on branch %q (err %v), want masterlocal", bb, err)
	}
}

// TestIsLocalRemotePathScpSyntax pins the session #86 regression: the
// classifier missed git's scp-like syntax when it carries no user part.
// isLocalRemotePath("example.com:foo/bar.git") returned true (no "://",
// no "@"), so HealRemoteHEAD on a dangling scp remote skipped the
// advisory branch and ran `git -C example.com:foo/bar.git symbolic-ref`
// — a hard error, violating HealRemoteHEAD's "warn, never fail the push"
// contract. The table ports git's own rule (connect.c
// url_is_local_not_ssh): local = no scheme AND no colon before the first
// slash, with Windows drive letters the sanctioned exception.
func TestIsLocalRemotePathScpSyntax(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		// Plain local paths: local, on any platform.
		{"/tmp/remote.git", true},
		{"remote.git", true},
		{"relative/dir/repo", true},
		// Schemes: never local.
		{"https://github.com/u/r.git", false},
		{"file:///tmp/remote.git", false},
		{"ssh://git@host/tmp/remote.git", false},
		// user@host scp-like: remote (was already correct).
		{"git@github.com:u/r.git", false},
		{"user@192.168.1.5:/srv/repo.git", false},
		// user-LESS scp-like: remote. These are the regression.
		{"example.com:foo/bar.git", false},
		{"example.com:foo", false},
		{"foo:bar", false},
		// Bracketed IPv6 without a user part: remote.
		{"[::1]:2222/x.git", false},
		{"[2001:db8::1]:repo.git", false},
		// Windows drive letters: local despite the leading colon shape.
		{"C:/Users/me/repo", true},
		{"c:\\Users\\me\\repo", true},
		// Pathological but decisive: a colon AFTER the first slash is
		// an ordinary filename on a local path, not a host separator.
		{"/tmp/wei:rd/repo", true},
		{"./a:b", true},
	}
	for _, tc := range cases {
		if got := isLocalRemotePath(tc.url); got != tc.want {
			t.Errorf("isLocalRemotePath(%q) = %v, want %v", tc.url, got, tc.want)
		}
	}
}

// TestHeadVerdict pins the HEAD classification contract across all five
// states, including the zero-branch-with-advertised-symref shape only a
// real server can produce (offline transports never advertise an unborn
// symref — probed over path and file://): the documented "still empty —
// nothing to clone yet, nothing to heal" line, which the pre-fix code
// violated by reporting a bare remote with an unborn symref as dangling.
func TestHeadVerdict(t *testing.T) {
	cases := []struct {
		name       string
		advertised string
		names      []string
		wantDangl  bool
		wantAdv    string
	}{
		{"empty-remote-no-symref", "", nil, false, ""},
		{"zero-branches-advertised-unborn", "main", nil, false, "main"},
		{"advertised-branch-exists", "main", []string{"main"}, false, "main"},
		{"dangling-advertised-over-branches", "main", []string{"work"}, true, "main"},
		{"path-local-dangling", "", []string{"masterlocal"}, true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dangling, advertised := headVerdict(tc.advertised, tc.names)
			if dangling != tc.wantDangl || advertised != tc.wantAdv {
				t.Errorf("headVerdict(%q, %v) = (%v, %q), want (%v, %q)",
					tc.advertised, tc.names, dangling, advertised, tc.wantDangl, tc.wantAdv)
			}
		})
	}
}

// TestDanglingRemoteHEADHealthyFileTransport exercises the symref-resolving
// branch end-to-end: file:// is the only offline scheme:// transport, and it
// resolves HEAD when the remote's default branch exists (probed) — the
// "hosted" parsing path, previously uncovered.
func TestDanglingRemoteHEADHealthyFileTransport(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	// Healthy means HEAD names the branch the push will create — set the
	// remote's symref to the pushing machine's branch name up front (a
	// first pusher wins scenario over a symref-resolving transport).
	setBranchNames(t, "masterlocal")
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/masterlocal").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD to masterlocal: %v (%s)", err, out)
	}

	machineA := t.TempDir()
	if err := Init(machineA); err != nil {
		t.Fatalf("Init A failed: %v", err)
	}
	commitFile(t, machineA, "nestor.yml", "config: FROM-A\n", "initial")
	if err := SetRemote(machineA, "origin", "file://"+remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Push(machineA, "origin"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	dangling, advertised := DanglingRemoteHEAD(machineA, "origin")
	if dangling {
		t.Error("file:// remote with HEAD at an existing branch reported dangling")
	}
	if advertised != "masterlocal" {
		t.Errorf("file:// transport should resolve the symref once HEAD exists, advertised = %q, want masterlocal", advertised)
	}
	// HealRemoteHEAD on a healthy remote is a silent no-op, on any transport.
	if advisory, err := HealRemoteHEAD(machineA, "origin"); err != nil || advisory != "" {
		t.Errorf("heal on healthy remote = (%q, %v), want (\"\", nil)", advisory, err)
	}
}

// TestHealRemoteHEADFileTransportDanglingAdvisory covers the advisory
// branch for a non-path URL: over file:// (the only offline scheme://
// transport) a dangling HEAD does NOT resolve its symref (probed — the
// ref: line appears only when HEAD's target exists), so the state reads
// as dangling with advertised="". HealRemoteHEAD must return the
// settings-change advisory — never an error, never an attempt to repair
// a URL it cannot reach as a path. The advertised-name advisory shape
// itself needs a real server and is pinned at headVerdict level instead.
func TestHealRemoteHEADFileTransportDanglingAdvisory(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD to main: %v (%s)", err, out)
	}
	setBranchNames(t, "masterlocal")

	machineA := t.TempDir()
	if err := Init(machineA); err != nil {
		t.Fatalf("Init A failed: %v", err)
	}
	commitFile(t, machineA, "nestor.yml", "config: FROM-A\n", "initial")
	if err := SetRemote(machineA, "origin", "file://"+remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Push(machineA, "origin"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}

	dangling, advertised := DanglingRemoteHEAD(machineA, "origin")
	if !dangling || advertised != "" {
		t.Fatalf("expected dangling with empty advertised over file://, got (%v, %q)", dangling, advertised)
	}

	advisory, err := HealRemoteHEAD(machineA, "origin")
	if err != nil {
		t.Fatalf("file:// dangling must not error: %v", err)
	}
	want := "remote HEAD is dangling (names a branch that does not exist)"
	if !strings.HasPrefix(advisory, want) {
		t.Errorf("advisory = %q, want prefix %q", advisory, want)
	}
	// The cure is a host setting, not something nestor can do from here:
	// the remote HEAD must still be dangling after the call.
	if dangling2, _ := DanglingRemoteHEAD(machineA, "origin"); !dangling2 {
		t.Error("remote HEAD unexpectedly healed by an advisory-only call")
	}
}

// TestAdvertisedSymref pins the ls-remote --symref parse shared by
// RemoteDefaultBranch and DanglingRemoteHEAD. The zero-oid row is the
// session #91 regression: a hosted remote advertises its default with an
// all-zero object id while the repo is still empty (probed — path-local
// transports advertise nothing when HEAD dangles). The pre-fix parser
// treated such a name as resolved, so RemoteDefaultBranch returned a
// dangling default over a real sole branch and alignUnbornWithRemote
// repointed the unborn local at it — the first push then forked history
// across two branches, the exact disease the alignment exists to prevent.
func TestAdvertisedSymref(t *testing.T) {
	zero := strings.Repeat("0", 40)
	oid := strings.Repeat("a", 40)
	cases := []struct {
		name       string
		out        string
		wantBranch string
		wantUnborn bool
	}{
		{"resolved", "ref: refs/heads/main\tHEAD\n" + oid + "\tHEAD\n", "main", false},
		{"unborn-zero-oid", "ref: refs/heads/main\tHEAD\n" + zero + "\tHEAD\n", "main", true},
		{"no-symref-line", oid + "\tHEAD\n", "", false},
		{"empty-output", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			branch, unborn := advertisedSymref(tc.out)
			if branch != tc.wantBranch || unborn != tc.wantUnborn {
				t.Errorf("advertisedSymref = (%q, %v), want (%q, %v)",
					branch, unborn, tc.wantBranch, tc.wantUnborn)
			}
		})
	}
}

// TestDefaultBranchDecision pins the full resolution contract, including
// the regression this session fixes: an unborn advertisement (zero-oid,
// hosted-only per the probes) over a still-empty remote is the only name
// the remote offers, so it must be adopted — the pre-fix fall-through
// discarded it, the align feature no-op'd on the exact master-vs-main
// shape it exists for, and the re-point branch ran in no test.
func TestDefaultBranchDecision(t *testing.T) {
	cases := []struct {
		name   string
		adv    string
		advUnb bool
		names  []string
		want   string
	}{
		{"resolved-advertisement-wins", "main", false, []string{"other"}, "main"},
		{"unborn-advertisement-over-empty-remote", "main", true, nil, "main"},
		{"unborn-advertisement-yields-to-sole-branch", "main", true, []string{"work"}, "work"},
		{"unborn-advertisement-ambiguous-over-two-branches", "main", true, []string{"work", "play"}, ""},
		{"no-advertisement-empty-remote", "", false, nil, ""},
		{"no-advertisement-sole-branch", "", false, []string{"work"}, "work"},
		{"no-advertisement-two-branches", "", false, []string{"work", "play"}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := defaultBranchDecision(tc.adv, tc.advUnb, tc.names); got != tc.want {
				t.Errorf("defaultBranchDecision(%q, %v, %v) = %q, want %q", tc.adv, tc.advUnb, tc.names, got, tc.want)
			}
		})
	}
}

// TestPointHeadAtRepointsUnbornHead pins the extracted re-point primitive:
// HEAD moves to the target branch without creating any ref, so the repo
// stays unborn and the first commit lands on the adopted name.
func TestPointHeadAtRepointsUnbornHead(t *testing.T) {
	setBranchNames(t, "masterlocal")
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if err := pointHeadAt(dir, "main"); err != nil {
		t.Fatalf("pointHeadAt failed: %v", err)
	}
	branch, err := Branch(dir)
	if err != nil || branch != "main" {
		t.Fatalf("after re-point Branch = %q (err %v), want main", branch, err)
	}
	// Still unborn: no ref was created, only HEAD moved.
	if out, err := exec.Command("git", "-C", dir, "rev-parse", "--verify", "--quiet", "HEAD").CombinedOutput(); err == nil {
		t.Fatalf("re-point created a ref; HEAD resolves to %s", strings.TrimSpace(string(out)))
	}
	commitFile(t, dir, "nestor.yml", "config: X\n", "first")
	branch, err = Branch(dir)
	if err != nil || branch != "main" {
		t.Fatalf("first commit after re-point landed on %q (err %v), want main", branch, err)
	}
}

// TestAlignAdoptsSoleRemoteBranch exercises alignUnbornWithRemote's re-point
// branch end-to-end over a path-local remote (the only offline shape that
// resolves a name for an unborn repo): the remote's HEAD dangles over its
// single real branch, the unborn machine adopts that name, and the sync
// lands on the branch the remote actually holds. Coverage map note: the
// re-point branch had 0% coverage since #91 degenerated the empty-remote
// align scenario into a no-op.
func TestAlignAdoptsSoleRemoteBranch(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	setBranchNames(t, "masterlocal")
	// Seed the remote with one real branch named differently from the
	// machines' default, and leave HEAD dangling over a never-created main.
	seed := t.TempDir()
	if err := Init(seed); err != nil {
		t.Fatalf("Init seed failed: %v", err)
	}
	commitFile(t, seed, "nestor.yml", "config: SEED\n", "seed")
	// Land the seed commit directly on refs/heads/work: pushing the branch
	// normally would ALSO create masterlocal, and a two-branch remote is
	// genuinely ambiguous — RemoteDefaultBranch would correctly return ""
	// (first draft did exactly that, and the test caught its own fixture).
	rev, err := exec.Command("git", "-C", seed, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("seed rev-parse: %v", err)
	}
	out, err := exec.Command("git", "-C", seed, "push", remote, strings.TrimSpace(string(rev))+":refs/heads/work").CombinedOutput()
	if err != nil {
		t.Fatalf("creating work branch: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("dangling remote HEAD: %v (%s)", err, out)
	}

	machine := t.TempDir()
	if err := Init(machine); err != nil {
		t.Fatalf("Init machine failed: %v", err)
	}
	if err := SetRemote(machine, "origin", remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := EnsureUnbornAligned(machine, "origin"); err != nil {
		t.Fatalf("EnsureUnbornAligned failed: %v", err)
	}
	branch, err := Branch(machine)
	if err != nil || branch != "work" {
		t.Fatalf("machine on %q (err %v), want adopted remote branch work", branch, err)
	}
	// The real flow after adoption: pull merges the remote's history
	// (unrelated first pull is allowed), then the machine's own commit
	// pushes fast-forward onto the branch it adopted.
	if err := Pull(machine, "origin"); err != nil {
		t.Fatalf("Pull after align failed: %v", err)
	}
	commitFile(t, machine, "local.yml", "local\n", "on adopted branch")
	if err := Push(machine, "origin"); err != nil {
		t.Fatalf("Push failed: %v", err)
	}
	if out, err := exec.Command("git", "-C", remote, "show", "work:local.yml").Output(); err != nil || strings.TrimSpace(string(out)) != "local" {
		t.Errorf("remote work branch missing machine's commit (got %q, err %v)", string(out), err)
	}
}

func TestPullConflictAbortsMerge(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	// Seed the remote: machine A commits its version of shared.yml.
	seed := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, seed).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}
	commitFile(t, seed, "shared.yml", "from machine A\n", "machine A config")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("seed Push failed: %v", err)
	}

	// Machine B: fresh repo, unborn HEAD, its own version of shared.yml —
	// the documented first-pull-on-a-second-machine shape.
	other := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", other).CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v (%s)", err, out)
	}
	if err := SetRemote(other, "origin", remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(other, "shared.yml"), []byte("from machine B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := Pull(other, "origin")
	if err == nil {
		t.Fatal("conflicting first pull should fail")
	}
	if !strings.Contains(err.Error(), "merge was aborted") {
		t.Errorf("error should say the merge was aborted, got: %v", err)
	}

	// The abort must have removed the conflict markers from the worktree:
	// the file is exactly what machine B wrote, byte for byte.
	got, readErr := os.ReadFile(filepath.Join(other, "shared.yml"))
	if readErr != nil {
		t.Fatalf("shared.yml missing after abort: %v", readErr)
	}
	if string(got) != "from machine B\n" {
		t.Errorf("conflict markers left in worktree, got: %q", got)
	}
	// ...and no merge left in progress.
	if mergeInProgress(other) {
		t.Error("merge still in progress after pull failure")
	}
	// The baseline commit nestor made before pulling still exists, so the
	// user can resolve and merge by hand.
	if unborn(other) {
		t.Error("baseline commit lost — HEAD is unborn again")
	}
}

func TestPullCleanFastForward(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	seed := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, seed).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}
	commitFile(t, seed, "a.yml", "a\n", "first")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("seed Push failed: %v", err)
	}

	// Machine B: fresh empty repo (no local files) — pull fast-forwards.
	other := t.TempDir()
	if out, err := exec.Command("git", "init", "-b", "main", other).CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v (%s)", err, out)
	}
	if err := SetRemote(other, "origin", remote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Pull(other, "origin"); err != nil {
		t.Fatalf("Pull failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(other, "a.yml")); err != nil {
		t.Fatalf("a.yml missing after Pull: %v", err)
	}
}

func TestPullLeavesUserMergeInProgress(t *testing.T) {
	remote := initRemoteRepo(t)
	if remote == "" {
		return
	}
	seed := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, seed).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}
	commitFile(t, seed, "a.yml", "a\n", "first")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("seed Push failed: %v", err)
	}

	other := t.TempDir()
	if out, err := exec.Command("git", "clone", remote, other).CombinedOutput(); err != nil {
		t.Fatalf("clone failed: %v (%s)", err, out)
	}

	// Start a merge the user owns, and leave it unfinished. Diverge first:
	// a --no-commit merge of an up-to-date branch is a no-op and creates no
	// MERGE_HEAD, so the remote must advance AND the clone must hold a local
	// commit before the test merge is real. The branch name is whatever the
	// clone checked out (initRemoteRepo does not pin one).
	branch, brErr := Branch(other)
	if brErr != nil || branch == "" {
		t.Fatalf("clone branch unknown: %v (%q)", brErr, branch)
	}
	commitFile(t, seed, "b.yml", "b\n", "advance remote")
	if err := Push(seed, "origin"); err != nil {
		t.Fatalf("advance Push failed: %v", err)
	}
	commitFile(t, other, "c.yml", "c\n", "diverge locally")
	if out, err := exec.Command("git", "-C", other, "fetch", "origin").CombinedOutput(); err != nil {
		t.Fatalf("fetch failed: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", other, "merge", "--no-commit", "--no-ff", "origin/"+branch).CombinedOutput(); err != nil {
		t.Fatalf("setup merge failed: %v (%s)", err, out)
	}
	if !mergeInProgress(other) {
		t.Fatal("fixture broken: no merge in progress after setup")
	}

	// A pull that fails for an unrelated reason must NOT abort the user's
	// merge. Use an unreachable second remote so the pull errors before any
	// merge is attempted.
	otherRemote := filepath.Join(t.TempDir(), "gone.git")
	if err := SetRemote(other, "broken", otherRemote); err != nil {
		t.Fatalf("SetRemote failed: %v", err)
	}
	if err := Pull(other, "broken"); err == nil {
		t.Fatal("pull from unreachable remote should fail")
	}
	if !mergeInProgress(other) {
		t.Error("user's in-progress merge was aborted by a failing pull")
	}
}

// TestIsUnbornStates pins IsUnborn on all three states: an unborn repo
// (fresh init, no commits), a born repo (after the first commit), and a
// directory that is not a git repository at all. Before the fix, IsUnborn
// delegated to a bare rev-parse failure, so every non-repo reported true.
func TestIsUnbornStates(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	setBranchNames(t, "mastervalue")

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if !IsUnborn(dir) {
		t.Error("IsUnborn(fresh init) = false, want true")
	}

	commitFile(t, dir, "nestor.yml", "version: 1\n", "initial")
	if IsUnborn(dir) {
		t.Error("IsUnborn(committed repo) = true, want false")
	}

	if IsUnborn(filepath.Join(dir, "missing")) {
		t.Error("IsUnborn(non-repo directory) = true, want false")
	}
}

// TestIsUnbornDetached pins the detached-HEAD state: HEAD resolves to a
// commit but names no branch, so the repo is not unborn.
func TestIsUnbornDetached(t *testing.T) {
	if !gitAvailable(t) {
		return
	}
	setBranchNames(t, "mastervalue")

	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	commitFile(t, dir, "nestor.yml", "version: 1\n", "initial")
	if out, err := exec.Command("git", "-C", dir, "checkout", "--detach", "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("detach failed: %v (%s)", err, out)
	}
	if IsUnborn(dir) {
		t.Error("IsUnborn(detached HEAD) = true, want false")
	}
}
