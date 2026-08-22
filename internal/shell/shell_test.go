package shell

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePlugin(t *testing.T) {
	tests := []struct {
		raw       string
		wantType  PluginType
		wantOwner string
		wantRepo  string
	}{
		{"zsh-users/zsh-autosuggestions", PluginGitHub, "zsh-users", "zsh-autosuggestions"},
		{"starship", PluginNamed, "", ""},
		{"  spaced/spaced-repo  ", PluginGitHub, "spaced", "spaced-repo"},
		{"/leading-slash", PluginNamed, "", ""},
		{"trailing-slash/", PluginNamed, "", ""},
		{"", PluginNamed, "", ""},
	}

	for _, tt := range tests {
		p := ParsePlugin(tt.raw)
		if p.Type != tt.wantType {
			t.Errorf("ParsePlugin(%q).Type = %d, want %d", tt.raw, p.Type, tt.wantType)
		}
		if p.Owner != tt.wantOwner {
			t.Errorf("ParsePlugin(%q).Owner = %q, want %q", tt.raw, p.Owner, tt.wantOwner)
		}
		if p.Repo != tt.wantRepo {
			t.Errorf("ParsePlugin(%q).Repo = %q, want %q", tt.raw, p.Repo, tt.wantRepo)
		}
	}
}

func TestRCFile(t *testing.T) {
	home := os.Getenv("HOME")
	defer os.Setenv("HOME", home)

	tests := []struct {
		shell string
		want  string
	}{
		{"zsh", filepath.Join(home, ".zshrc")},
		{"bash", filepath.Join(home, ".bashrc")},
		{"fish", ""}, // unsupported
		{"", ""},
	}

	for _, tt := range tests {
		got := RCFile(tt.shell)
		if got != tt.want {
			t.Errorf("RCFile(%q) = %q, want %q", tt.shell, got, tt.want)
		}
	}
}

func TestSourceLines(t *testing.T) {
	results := []PluginResult{
		{Plugin: Plugin{Raw: "zsh-users/zsh-autosuggestions", Type: PluginGitHub, Owner: "zsh-users", Repo: "zsh-autosuggestions"}, Status: StatusInstalled, Path: "/fake/zsh-autosuggestions"},
		{Plugin: Plugin{Raw: "starship", Type: PluginNamed}, Status: StatusSkipped},
		{Plugin: Plugin{Raw: "zsh-users/zsh-syntax-highlighting", Type: PluginGitHub, Owner: "zsh-users", Repo: "zsh-syntax-highlighting"}, Status: StatusInstalled, Path: "/fake/zsh-syntax-highlighting"},
		{Plugin: Plugin{Raw: "failed/plugin", Type: PluginGitHub, Owner: "failed", Repo: "plugin"}, Status: StatusError},
	}

	lines := SourceLines(results)
	if len(lines) != 2 {
		t.Fatalf("expected 2 source lines, got %d: %v", len(lines), lines)
	}

	// First should be autosuggestions
	want := "source /fake/zsh-autosuggestions/zsh-autosuggestions.zsh"
	if lines[0] != want {
		t.Errorf("lines[0] = %q, want %q", lines[0], want)
	}

	// Dedup check
	dupResults := append(results, results[0])
	dupLines := SourceLines(dupResults)
	if len(dupLines) != 2 {
		t.Errorf("dedup failed: got %d lines, expected 2", len(dupLines))
	}
}

func TestWriteSourceBlock_NewFile(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".zshrc")

	lines := []string{"source /fake/zsh-autosuggestions/zsh-autosuggestions.zsh"}

	if err := WriteSourceBlock(rcPath, lines); err != nil {
		t.Fatalf("WriteSourceBlock: %v", err)
	}

	data, _ := os.ReadFile(rcPath)
	content := string(data)

	if !contains(content, markerBegin) || !contains(content, markerEnd) {
		t.Error("missing markers in new block")
	}
	if !contains(content, lines[0]) {
		t.Error("missing source line in new block")
	}
}

func TestWriteSourceBlock_Idempotent(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".zshrc")

	// Write existing user content
	existing := "# my config\nexport EDITOR=vim\n"
	os.WriteFile(rcPath, []byte(existing), 0644)

	lines1 := []string{"source /fake/plugin1/plugin1.zsh"}
	WriteSourceBlock(rcPath, lines1)

	// Now update with different lines
	lines2 := []string{"source /fake/plugin2/plugin2.zsh"}
	WriteSourceBlock(rcPath, lines2)

	data, _ := os.ReadFile(rcPath)
	content := string(data)

	if !contains(content, "export EDITOR=vim") {
		t.Error("existing user content was clobbered")
	}
	// Should only have ONE begin marker
	if countStr(content, markerBegin) != 1 {
		t.Errorf("expected 1 begin marker, got %d", countStr(content, markerBegin))
	}
	// New plugin should be present, old should be gone
	if !contains(content, "plugin2/plugin2.zsh") {
		t.Error("new source line missing after update")
	}
	if contains(content, "plugin1/plugin1.zsh") {
		t.Error("old source line should have been replaced")
	}
}

func TestWriteSourceBlock_EmptyLines(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".bashrc")

	existing := "export PATH=/usr/local/bin:$PATH\n"

	os.WriteFile(rcPath, []byte(existing), 0644)

	// First write with lines
	WriteSourceBlock(rcPath, []string{"source /fake/plugin/plugin.zsh"})
	// Second write with empty — should clear the block content but keep markers
	WriteSourceBlock(rcPath, []string{})

	data, _ := os.ReadFile(rcPath)
	content := string(data)

	if !contains(content, "export PATH") {
		t.Error("existing user content clobbered on empty update")
	}
	if !contains(content, markerBegin) {
		t.Error("markers should still be present even with empty lines")
	}
	if contains(content, "source /fake") {
		t.Error("old source line should be cleared when empty")
	}
}

func TestDetect(t *testing.T) {
	// Save and restore
	origShell := os.Getenv("SHELL")
	defer os.Setenv("SHELL", origShell)

	os.Setenv("SHELL", "/usr/bin/zsh")
	shellName, err := Detect()
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if shellName != "zsh" {
		t.Errorf("Detect() = %q, want %q", shellName, "zsh")
	}

	os.Setenv("SHELL", "")
	_, err = Detect()
	if err == nil {
		t.Error("expected error when SHELL unset")
	}
}

func TestInstallPlugins_NamedOnly(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	results := InstallPlugins([]string{"starship", "eza"})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Status != StatusSkipped {
			t.Errorf("named plugin %q should be skipped, got status %d", r.Plugin.Raw, r.Status)
		}
	}
}

// fixtureRepo creates a local git repo with one commit and returns its path.
func fixtureRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	git := func(args ...string) []string {
		return append([]string{"-C", dir, "-c", "user.email=t@t", "-c", "user.name=t"}, args...)
	}
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run(git("init", "-q")...)
	run(git("commit", "--allow-empty", "-m", "init")...)
	return dir
}

// TestInstallPlugins_GitHubClone exercises the real git round-trip against a
// local fixture repo, via the cloneURLFn seam (0% -> covered without network).
func TestInstallPlugins_GitHubClone(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := fixtureRepo(t)
	orig := cloneURLFn
	cloneURLFn = func(owner, repo string) string { return src }
	t.Cleanup(func() { cloneURLFn = orig })

	results := InstallPlugins([]string{"acme/plugin-one", "acme/plugin-two", "starship"})
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	r1 := results[0]
	if r1.Status != StatusInstalled {
		t.Fatalf("plugin-one: status=%d, err=%v", r1.Status, r1.Err)
	}
	if r1.Path == "" || !strings.HasSuffix(r1.Path, "plugin-one") {
		t.Errorf("plugin-one path = %q, want suffix plugin-one", r1.Path)
	}
	// The .git dir proves a real clone happened.
	if fi, err := os.Stat(filepath.Join(r1.Path, ".git")); err != nil || !fi.IsDir() {
		t.Errorf("expected %s/.git to be a dir, err=%v", r1.Path, err)
	}

	if results[2].Status != StatusSkipped {
		t.Errorf("starship should stay skipped, got %d", results[2].Status)
	}
}

// TestInstallPlugins_GitHubPull covers the already-cloned branch: second run
// must pull --ff-only, not try to re-clone.
func TestInstallPlugins_GitHubPull(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	src := fixtureRepo(t)
	orig := cloneURLFn
	cloneURLFn = func(owner, repo string) string { return src }
	t.Cleanup(func() { cloneURLFn = orig })

	raw := "acme/plugin-one"
	if r := InstallPlugins([]string{raw})[0]; r.Status != StatusInstalled {
		t.Fatalf("first install: status=%d, err=%v", r.Status, r.Err)
	}

	// New commit on the source; second run takes the pull branch.
	if out, err := exec.Command("git", "-C", src, "-c", "user.email=t@t", "-c", "user.name=t",
		"commit", "--allow-empty", "-m", "second").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}

	results := InstallPlugins([]string{raw})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != StatusInstalled {
		t.Errorf("second install (pull path): status=%d, err=%v", results[0].Status, results[0].Err)
	}
}

// TestInstallPlugins_Clonerror covers the failure branch: cloneURLFn points at
// a path that isn't a repo, git clone fails, result carries StatusError.
func TestInstallPlugins_Clonerror(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	empty := t.TempDir()
	orig := cloneURLFn
	cloneURLFn = func(owner, repo string) string { return empty }
	t.Cleanup(func() { cloneURLFn = orig })

	results := InstallPlugins([]string{"acme/broken"})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.Status != StatusError {
		t.Fatalf("expected StatusError, got %d", r.Status)
	}
	if r.Err == nil {
		t.Error("expected non-nil Err on clone failure")
	}
}

func TestPluginsPath(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)

	got, err := PluginsPath()
	if err != nil {
		t.Fatalf("PluginsPath: %v", err)
	}
	want := filepath.Join(cfg, "nestor", "plugins")
	if got != want {
		t.Errorf("PluginsPath() = %q, want %q", got, want)
	}
}

func TestWriteSourceBlock_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	rcPath := filepath.Join(dir, ".zshrc")

	// Existing content WITHOUT trailing newline — the append branch must add one
	// before the block so the marker starts on its own line.
	os.WriteFile(rcPath, []byte("export EDITOR=vim"), 0644)

	if err := WriteSourceBlock(rcPath, []string{"source /fake/p/p.zsh"}); err != nil {
		t.Fatalf("WriteSourceBlock: %v", err)
	}

	data, _ := os.ReadFile(rcPath)
	content := string(data)
	if !contains(content, "export EDITOR=vim\n") {
		t.Errorf("expected newline inserted between content and block:\n%q", content)
	}
	if !contains(content, markerBegin) {
		t.Error("missing begin marker")
	}
}

// --- helpers ---

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && indexOf(s, substr) >= 0)
}

func indexOf(s, sep string) int {
	for i := 0; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

func countStr(s, substr string) int {
	n := 0
	for {
		idx := indexOf(s, substr)
		if idx < 0 {
			return n
		}
		n++
		s = s[idx+len(substr):]
	}
}
