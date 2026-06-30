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
