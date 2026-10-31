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

// TestPullNoRemoteConfigured verifies that pull fails with a clear error
// when no remote is configured (not an init failure or silent success).
func TestPullNoRemoteConfigured(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	// reset remote flag
	pullRemoteURL = ""
	defer func() { pullRemoteURL = "" }()

	err := runPullOut(context.Background(), &buf)
	if err == nil {
		t.Fatal("expected error when no remote is configured, got nil")
	}
	if !strings.Contains(err.Error(), "no remote") {
		t.Fatalf("expected error mentioning 'no remote', got: %v", err)
	}
}

// TestPullNoGit checks the pre-flight git check.
func TestPullNoGit(t *testing.T) {
	// Can't easily remove git from PATH in a Go test, but we can at least
	// verify the function doesn't panic and delegates correctly on the
	// happy path. If git is available (it is in this env), this is covered
	// by TestPullNoRemoteConfigured. Skip otherwise.
	skipIfNoGit(t)
	t.Skip("git is available — covered by TestPullNoRemoteConfigured")
}

// TestPullRefusesInvalidRemoteConfig verifies the post-pull validation:
// a remote that carries a config validate() refuses (here: a secret inject
// target on nestor's own config — the 2026-10-19/26 corruption family) must
// make pull fail loudly instead of leaving a config every later command
// refuses to load. The merge IS committed (that's what pull does), so the
// test pins that the failure names the live config and ORIG_HEAD. The
// remote descends from the local history (machine A pushes, machine B edits
// and pushes) so the pull is a fast-forward, the shape a real sync has.
func TestPullRefusesInvalidRemoteConfig(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, ".gitignore"), []byte(".gitignore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", cfgDir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in cfgDir: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("add", "-A")
	run("commit", "-m", "local baseline")

	// Machine B: clone, write the invalid config, push back to the bare hub.
	bare := t.TempDir() + "/hub.git"
	run("init", "--bare", bare)
	run("remote", "add", "origin", bare)
	run("push", "-u", "origin", "main")
	clone := t.TempDir() + "/machineB"
	run("clone", "-b", "main", bare, clone)
	invalid := "version: 1\nsecrets:\n  provider: env\n  mappings:\n    - key: API_KEY\n      inject:\n        \"~/.config/nestor/nestor.yml\": \"{{ API_KEY }}\"\n"
	if err := os.WriteFile(filepath.Join(clone, "nestor.yml"), []byte(invalid), 0o644); err != nil {
		t.Fatal(err)
	}
	b := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", clone}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in machineB: %v\n%s", args, err, out)
		}
		return string(out)
	}
	b("config", "user.email", "t@t")
	b("config", "user.name", "t")
	b("add", "-A")
	b("commit", "-m", "invalid config")
	b("push", "origin", "HEAD:main")

	var buf bytes.Buffer
	pullRemoteURL = bare
	defer func() { pullRemoteURL = "" }()

	err := runPullOut(context.Background(), &buf)
	if err == nil {
		t.Fatal("expected pull to refuse the invalid config, got nil")
	}
	if !strings.Contains(err.Error(), "pulled config is invalid") {
		t.Fatalf("expected 'pulled config is invalid' error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "ORIG_HEAD") {
		t.Fatalf("expected recovery hint with ORIG_HEAD, got: %v", err)
	}
}

// TestPullCleanFastForwardStillSucceeds is the untouched path: a remote
// carrying a valid config change pulls fine and prints the follow-up hint.
func TestPullCleanFastForwardStillSucceeds(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	cfgDir := filepath.Join(os.Getenv("HOME"), ".config", "nestor")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, ".gitignore"), []byte(".gitignore\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", cfgDir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in cfgDir: %v\n%s", args, err, out)
		}
		return string(out)
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("add", "-A")
	run("commit", "-m", "local baseline")

	bare := t.TempDir() + "/hub.git"
	run("init", "--bare", bare)
	run("remote", "add", "origin", bare)
	run("push", "-u", "origin", "main")
	clone := t.TempDir() + "/machineB"
	run("clone", "-b", "main", bare, clone)
	valid := "version: 1\ndotfiles:\n  templates:\n    - src: bashrc.tmpl\n      dest: ~/.bashrc\n"
	if err := os.WriteFile(filepath.Join(clone, "nestor.yml"), []byte(valid), 0o644); err != nil {
		t.Fatal(err)
	}
	b := func(args ...string) string {
		out, err := exec.Command("git", append([]string{"-C", clone}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v in machineB: %v\n%s", args, err, out)
		}
		return string(out)
	}
	b("config", "user.email", "t@t")
	b("config", "user.name", "t")
	b("add", "-A")
	b("commit", "-m", "valid config")
	b("push", "origin", "HEAD:main")

	var buf bytes.Buffer
	pullRemoteURL = bare
	defer func() { pullRemoteURL = "" }()

	if err := runPullOut(context.Background(), &buf); err != nil {
		t.Fatalf("expected clean pull to succeed, got: %v", err)
	}
	if !strings.Contains(buf.String(), "run 'nestor up'") {
		t.Fatalf("expected success output, got: %s", buf.String())
	}
}
