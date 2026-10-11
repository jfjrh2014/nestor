package cmd

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/vcs"
)

// remoteTestDir points cfgFile at a fresh temp config dir and returns the
// dir path. The dir gets a minimal nestor.yml so it looks like a config
// dir; remote commands only consult the path, not the file's contents.
func remoteTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	if err := os.WriteFile(cfgPath, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfgFile = cfgPath
	t.Cleanup(func() { cfgFile = "" })
	return dir
}

func TestRemoteAddInitializesRepoAndSetsRemote(t *testing.T) {
	dir := remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteAddOut("file:///tmp/example.git", &out); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !vcs.IsRepo(dir) {
		t.Fatal("repo was not initialized")
	}
	if got := vcs.GetRemote(dir, remoteName); got != "file:///tmp/example.git" {
		t.Fatalf("remote url = %q, want file:///tmp/example.git", got)
	}
	if !strings.Contains(out.String(), "remote 'origin' set to file:///tmp/example.git") {
		t.Fatalf("output missing confirmation line:\n%s", out.String())
	}
}

func TestRemoteAddExistingRemoteUpdatesURL(t *testing.T) {
	dir := remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteAddOut("file:///tmp/first.git", &out); err != nil {
		t.Fatalf("first add: %v", err)
	}
	out.Reset()
	if err := runRemoteAddOut("file:///tmp/second.git", &out); err != nil {
		t.Fatalf("second add: %v", err)
	}
	// SetRemote goes through set-url on an existing remote: exactly one
	// origin, pointing at the new URL. A second `remote add` would have
	// errored ("remote origin already exists") or left the stale URL.
	urls, err := exec.Command("git", "-C", dir, "remote").Output()
	if err != nil {
		t.Fatalf("git remote: %v", err)
	}
	if got := strings.TrimSpace(string(urls)); got != "origin" {
		t.Fatalf("remotes = %q, want exactly [origin]", got)
	}
	if got := vcs.GetRemote(dir, remoteName); got != "file:///tmp/second.git" {
		t.Fatalf("remote url = %q, want file:///tmp/second.git", got)
	}
}

func TestRemoteShowPrintsConfiguredURL(t *testing.T) {
	remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteAddOut("file:///tmp/showme.git", &out); err != nil {
		t.Fatalf("add: %v", err)
	}
	out.Reset()
	if err := runRemoteShowOut(&out); err != nil {
		t.Fatalf("show: %v", err)
	}
	// ui colorizes the URL; match the content, not the escape codes.
	if !strings.Contains(out.String(), "file:///tmp/showme.git") {
		t.Fatalf("show output missing URL:\n%s", out.String())
	}
}

func TestRemoteShowWithoutRemote(t *testing.T) {
	remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteShowOut(&out); err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(out.String(), "no remote configured") {
		t.Fatalf("output missing no-remote notice:\n%s", out.String())
	}
}

func TestRemoteRemoveWithoutRemote(t *testing.T) {
	remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteRemoveOut(&out); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if !strings.Contains(out.String(), "no remote configured") {
		t.Fatalf("output missing nothing-to-remove notice:\n%s", out.String())
	}
}

func TestRemoteRemoveRemovesOrigin(t *testing.T) {
	dir := remoteTestDir(t)
	var out bytes.Buffer
	if err := runRemoteAddOut("file:///tmp/gone.git", &out); err != nil {
		t.Fatalf("add: %v", err)
	}
	out.Reset()
	if err := runRemoteRemoveOut(&out); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if vcs.RemoteSet(dir, remoteName) {
		t.Fatal("remote origin still set after remove")
	}
	if !strings.Contains(out.String(), "removed remote 'origin' (was file:///tmp/gone.git)") {
		t.Fatalf("output missing removal line:\n%s", out.String())
	}
}

// withGitlessPATH empties PATH for the test so exec.LookPath("git") fails,
// simulating a machine without git. Restored by t.Setenv's cleanup.
func withGitlessPATH(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}

func TestRemoteShowWithoutGitRefuses(t *testing.T) {
	remoteTestDir(t)
	withGitlessPATH(t)
	var out bytes.Buffer
	err := runRemoteShowOut(&out)
	if !errors.Is(err, vcs.ErrGitNotFound) {
		t.Fatalf("err = %v, want ErrGitNotFound", err)
	}
}

func TestRemoteRemoveWithoutGitRefuses(t *testing.T) {
	dir := remoteTestDir(t)
	// Remote IS configured; without git the old code let the swallowed
	// exec failure read as "no remote configured (nothing to remove)".
	if out, err := exec.Command("git", "-C", dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if err := vcs.SetRemote(dir, remoteName, "file:///tmp/nogit.git"); err != nil {
		t.Fatalf("set remote: %v", err)
	}
	// Manual PATH swap (not t.Setenv) so it can be restored before the
	// surviving-remote check: vcs.RemoteSet shells out to git and reads
	// false under a gitless PATH.
	origPATH := os.Getenv("PATH")
	os.Setenv("PATH", t.TempDir())
	defer os.Setenv("PATH", origPATH)
	var out bytes.Buffer
	err := runRemoteRemoveOut(&out)
	os.Setenv("PATH", origPATH)
	if !errors.Is(err, vcs.ErrGitNotFound) {
		t.Fatalf("err = %v, want ErrGitNotFound", err)
	}
	if strings.Contains(out.String(), "no remote configured") {
		t.Fatalf("output must not claim no-remote when git is missing:\n%s", out.String())
	}
	if !vcs.RemoteSet(dir, remoteName) {
		t.Fatal("remote was removed despite gitless refusal")
	}
}

func TestRemoteAddWithoutGitRefuses(t *testing.T) {
	remoteTestDir(t)
	withGitlessPATH(t)
	var out bytes.Buffer
	err := runRemoteAddOut("file:///tmp/x.git", &out)
	if !errors.Is(err, vcs.ErrGitNotFound) {
		t.Fatalf("err = %v, want ErrGitNotFound", err)
	}
}
