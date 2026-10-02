package cmd

import (
	"bytes"
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
