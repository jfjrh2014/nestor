package cmd

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipIfNoGit skips the test when git isn't installed.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git not installed — skipping")
	}
}

// isolatedConfigHome redirects HOME to a temp dir and creates a local
// nestor.yml there, ensuring the push/pull commands act on an isolated
// config dir rather than the real ~/.config/nestor.
func isolatedConfigHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	customHome := filepath.Join(dir, "home")
	if err := os.MkdirAll(customHome, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", customHome)
	t.Setenv("USERPROFILE", customHome) // Windows
	return dir
}

func TestPushLocalCommitNoRemote(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	// Create a nestor config dir with a config file
	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Pre-set a non-interactive git identity
	if err := os.WriteFile(filepath.Join(cfgDir, "test"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var buf bytes.Buffer
	err := runPushOut(ctx, &buf)
	if err != nil {
		// Should fail gracefully with no remote
		t.Logf("push returned: %v (expected graceful failure or success)", err)
	}

	out := buf.String()
	if !strings.Contains(out, "initialized git repo") && !strings.Contains(out, "committed locally") {
		// The repo might already exist on machines with a parent repo — check at least one indicator.
		if !strings.Contains(out, "no remote") && !strings.Contains(out, "nothing to commit") {
			t.Errorf("unexpected push output:\n%s", out)
		}
	}
}

func TestRemoteAddShow(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := runRemoteAddOut("https://github.com/example/dotfiles.git", os.Stdout); err != nil {
		t.Fatalf("runRemoteAddOut: %v", err)
	}

	got := remoteURLForTest(t, cfgDir)
	if got != "https://github.com/example/dotfiles.git" {
		t.Errorf("expected remote URL, got %q", got)
	}
}

func TestRemoteShowEmpty(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runRemoteShowOut(os.Stdout); err != nil {
		t.Fatalf("runRemoteShowOut: %v", err)
	}
}

func TestRemoteRemove(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// add then remove
	if err := runRemoteAddOut("https://github.com/example/dotfiles.git", os.Stdout); err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if err := runRemoteRemoveOut(os.Stdout); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	got := remoteURLForTest(t, cfgDir)
	if got != "" {
		t.Errorf("expected empty remote URL after remove, got %q", got)
	}
}

func TestNestorConfigDirFromEnv(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	// With HOME set and no local nestor.yml, should point to ~/.config/nestor
	got, err := nestorConfigDir()
	if err != nil {
		t.Fatalf("nestorConfigDir: %v", err)
	}
	want := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if got != want {
		t.Errorf("nestorConfigDir = %q, want %q", got, want)
	}
}

func TestNestorConfigDirLocalNestorYml(t *testing.T) {
	skipIfNoGit(t)
	dir := t.TempDir()
	chdirScope(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := nestorConfigDir()
	if err != nil {
		t.Fatalf("nestorConfigDir: %v", err)
	}
	abs, _ := filepath.Abs(dir)
	if got != abs {
		t.Errorf("nestorConfigDir = %q, want %q", got, abs)
	}
}

// remoteURLForTest reads the origin remote URL directly via git.
func remoteURLForTest(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// brokenHome returns a HOME value under which ~/.config/nestor cannot be
// created — the parent is a regular file, so MkdirAll fails. Used to force
// an error out of the remote subcommands to prove error propagation.
func brokenHome(t *testing.T) {
	t.Helper()
	blocker := filepath.Join(t.TempDir(), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", filepath.Join(blocker, "home"))
}

// TestRemoteAddErrorPropagates proves the RunE fix from session #38:
// runRemoteAddOut must return its errors, not swallow them. Before the fix
// every error path printed to stderr and returned nil, so a CI script calling
// `nestor remote add` exited 0 on total failure.
func TestRemoteAddErrorPropagates(t *testing.T) {
	skipIfNoGit(t)
	brokenHome(t)

	err := runRemoteAddOut("https://github.com/example/dotfiles.git", os.Stdout)
	if err == nil {
		t.Fatal("expected error from runRemoteAddOut with a broken config dir, got nil")
	}
}

// TestRemoteShowEmptyStillTrivial confirms the show path still returns nil
// (not an error) when there is genuinely nothing to report — i.e. the error-
// propagation fix didn't make "no remote configured" into a failure.
func TestRemoteShowEmptyStillTrivial(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runRemoteShowOut(os.Stdout); err != nil {
		t.Fatalf("runRemoteShowOut on empty repo should be nil, got: %v", err)
	}
}

// TestRemoteRemoveNoOpStillTrivial confirms the remove path still returns nil
// when there is nothing to remove — i.e. the error-propagation fix didn't
// turn "nothing to remove" into a failure.
func TestRemoteRemoveNoOpStillTrivial(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := runRemoteRemoveOut(os.Stdout); err != nil {
		t.Fatalf("runRemoteRemoveOut on empty repo should be nil, got: %v", err)
	}
}

// chdirScope changes to dir for the duration of the test.
func chdirScope(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

// setInitDefaultBranch overrides the branch name a fresh `git init` gets,
// to simulate a machine whose default (masterlocal) differs from the
// remote's (main, as GitHub forces on new empty repos).
func setInitDefaultBranch(t *testing.T, name string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "gitconfig"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	if out, err := exec.Command("git", "config", "--global", "init.defaultBranch", name).CombinedOutput(); err != nil {
		t.Fatalf("setting init.defaultBranch: %v (%s)", err, out)
	}
}

// bareRemoteWithHead returns the path of a fresh bare remote whose HEAD
// points at main, the way GitHub creates empty repos.
func bareRemoteWithHead(t *testing.T) string {
	t.Helper()
	remote := filepath.Join(t.TempDir(), "remote.git")
	if out, err := exec.Command("git", "init", "--bare", remote).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v (%s)", err, out)
	}
	if out, err := exec.Command("git", "-C", remote, "symbolic-ref", "HEAD", "refs/heads/main").CombinedOutput(); err != nil {
		t.Fatalf("setting remote HEAD: %v (%s)", err, out)
	}
	return remote
}

// remoteBranchExists reports whether a branch with commits exists on a
// bare remote.
func remoteBranchExists(t *testing.T, remote, branch string) bool {
	t.Helper()
	return exec.Command("git", "-C", remote, "show", branch).Run() == nil
}

// TestPushPullCrossMachineBranchAlignment is the end-to-end session #71
// regression through the real command entrypoints: push from a master-
// default machine into an empty remote, then pull on a fresh second
// machine. Before the fix the push silently created a differently-named
// remote branch and the second machine's `nestor pull` failed outright
// with "couldn't find remote ref HEAD". After the fix: first pusher wins
// the branch name, every follower adopts it, and content syncs both ways.
func TestPushPullCrossMachineBranchAlignment(t *testing.T) {
	skipIfNoGit(t)
	setInitDefaultBranch(t, "masterlocal")
	remote := bareRemoteWithHead(t)

	ctx := context.Background()

	// Machine A pushes.
	isolatedConfigHome(t)
	if err := os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".config", "nestor"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(os.Getenv("HOME"), ".config", "nestor", "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("config: FROM-A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pushRemoteURL = remote
	defer func() { pushRemoteURL = "" }()
	var bufA bytes.Buffer
	if err := runPushOut(ctx, &bufA); err != nil {
		t.Fatalf("push from machine A: %v\n%s", err, bufA.String())
	}
	// First pusher wins: the remote holds exactly one branch, masterlocal.
	if out, err := exec.Command("git", "-C", remote, "show", "masterlocal:nestor.yml").Output(); err != nil || strings.TrimSpace(string(out)) != "config: FROM-A" {
		t.Errorf("remote masterlocal should hold A's config (got %q, err %v)", string(out), err)
	}
	// ...and no phantom second branch: the silent-fork symptom.
	if remoteBranchExists(t, remote, "main") {
		t.Error("remote gained a phantom main branch; push forked the sync")
	}

	// Machine B pulls into a fresh HOME.
	isolatedConfigHome(t)
	bCfgPath := filepath.Join(os.Getenv("HOME"), ".config", "nestor", "nestor.yml")
	pullRemoteURL = remote
	defer func() { pullRemoteURL = "" }()
	var bufB bytes.Buffer
	if err := runPullOut(ctx, &bufB); err != nil {
		t.Fatalf("pull on machine B: %v\n%s", err, bufB.String())
	}
	if !strings.Contains(bufB.String(), "pulled latest config") {
		t.Errorf("pull output missing success line:\n%s", bufB.String())
	}
	data, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "nestor", "nestor.yml"))
	if err != nil {
		t.Fatalf("pulled config missing on B: %v", err)
	}
	if strings.TrimSpace(string(data)) != "config: FROM-A" {
		t.Errorf("B pulled %q, want A's config", string(data))
	}
	if ref, err := exec.Command("git", "-C", filepath.Join(os.Getenv("HOME"), ".config", "nestor"), "symbolic-ref", "--short", "HEAD").Output(); err != nil || strings.TrimSpace(string(ref)) != "masterlocal" {
		t.Errorf("B on branch %q (err %v), want masterlocal", string(ref), err)
	}

	// B edits and pushes; then the remote must hold B's edit (round trip).
	if err := os.WriteFile(bCfgPath, []byte("config: FROM-B\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pushRemoteURL = remote
	var bufB2 bytes.Buffer
	if err := runPushOut(ctx, &bufB2); err != nil {
		t.Fatalf("push from machine B: %v\n%s", err, bufB2.String())
	}
	if out, err := exec.Command("git", "-C", remote, "show", "masterlocal:nestor.yml").Output(); err != nil || strings.TrimSpace(string(out)) != "config: FROM-B" {
		t.Errorf("remote masterlocal should hold B's edit (got %q, err %v)", string(out), err)
	}
}
