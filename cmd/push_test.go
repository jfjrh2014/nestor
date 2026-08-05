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
