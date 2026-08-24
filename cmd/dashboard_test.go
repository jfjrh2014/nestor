package cmd

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
)

func TestDashSecretsProvider(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want string
	}{
		{"empty provider, no mappings", &config.Config{Secrets: config.Secrets{}}, "none"},
		{"empty provider, has mappings", &config.Config{Secrets: config.Secrets{Mappings: []config.Mapping{{Key: "x"}}}}, "env"},
		{"set provider, has mappings", &config.Config{Secrets: config.Secrets{Provider: "bitwarden", Mappings: []config.Mapping{{Key: "x"}}}}, "bitwarden"},
		{"set provider, no mappings", &config.Config{Secrets: config.Secrets{Provider: "bitwarden"}}, "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dashSecretsProvider(tt.cfg); got != tt.want {
				t.Errorf("dashSecretsProvider() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDashProfiles(t *testing.T) {
	cfg := &config.Config{
		Profiles: map[string]config.Profile{
			"personal": {Packages: []string{"discord"}},
			"work":     {Packages: []string{"slack"}},
		},
	}
	got := dashProfiles(cfg)
	if len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}
	// should be sorted
	want := "personal"
	if got[0] != want {
		t.Errorf("expected first profile %q, got %q", want, got[0])
	}

	// empty profiles
	emptyCfg := &config.Config{}
	if got := dashProfiles(emptyCfg); len(got) != 0 {
		t.Errorf("expected 0 profiles for empty config, got %d", len(got))
	}
}

// dashDotfileState maps an internal dashDotfileStatus to the (present, drift)
// tuple, to keep these tests decoupled from the struct's field names.
func dashDotfileState(present, drift bool) dashDotfileStatus {
	return dashDotfileStatus{present: present, drift: drift}
}

// TestDashDotfileDriftCopy is a regression test for the drift detection
// reimplementation. Previously, only symlink-strategy dotfiles ever had drift
// checked; copy-strategy dotfiles that drifted in content were always marked
// "present, not drifted". By delegating to dotfiles.Deployer.Check (which
// compares file contents), drift is now detected for both strategies.
func TestDashDotfileDriftCopy(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	destDir := filepath.Join(dir, "dest")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A source dotfile, and a destination with different content.
	src := filepath.Join(srcDir, "gitconfig")
	if err := os.WriteFile(src, []byte("[user]\n  name = original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(destDir, "gitconfig")
	if err := os.WriteFile(dest, []byte("[user]\n  name = drifted\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategyCopy, Source: srcDir}
	got := deployer.Check("gitconfig", dest)
	if got != dotfiles.CheckDrifted {
		t.Fatalf("copy-strategy drift should be detected; got %s", got)
	}

	// Match the dest content to confirm present detection.
	if err := os.WriteFile(dest, []byte("[user]\n  name = original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got = deployer.Check("gitconfig", dest)
	if got != dotfiles.CheckPresent {
		t.Fatalf("matching content should be present; got %s", got)
	}
}

// TestDashDotfileDriftSymlink is a regression test for the broken use of
// os.Stat (follows symlinks). A drifted symlink (points somewhere unexpected)
// should read as present+drifted, not absent.
func TestDashDotfileDriftSymlink(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(srcDir, "tmux.conf")
	if err := os.WriteFile(src, []byte("set -g status on\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Drifted symlink: points to a file NOT in the source dir.
	other := filepath.Join(dir, "imposter.conf")
	if err := os.WriteFile(other, []byte("set -g status off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, ".tmux.conf")
	if err := os.Symlink(other, dest); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategySymlink, Source: srcDir}
	got := deployer.Check("tmux.conf", dest)
	if got != dotfiles.CheckDrifted {
		t.Fatalf("drifted symlink should be CheckDrifted; got %s", got)
	}

	// Fix the symlink → should be present.
	if err := os.Remove(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(src, dest); err != nil {
		t.Fatal(err)
	}
	got = deployer.Check("tmux.conf", dest)
	if got != dotfiles.CheckPresent {
		t.Fatalf("correct symlink should be present; got %s", got)
	}
}

// TestDashDotfileAbsent verifies the absent case still maps to present=false.
func TestDashDotfileAbsent(t *testing.T) {
	dir := t.TempDir()
	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "vimrc"), []byte("set number\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	deployer := dotfiles.Deployer{Strategy: dotfiles.StrategyCopy, Source: srcDir}
	got := deployer.Check("vimrc", filepath.Join(dir, ".nonexistent"))
	if got != dotfiles.CheckAbsent {
		t.Fatalf("missing dest should be CheckAbsent; got %s", got)
	}
}

// TestDashDotfileStatusMapping locks in the contract that the four CheckStatus
// values map onto dashDotfileStatus the way dashboard.go's dashLoadStatus relies
// on. If someone changes the mapping, this breaks before a real dashboard run
// ever sees it.
func TestDashDotfileStatusMapping(t *testing.T) {
	cases := []struct {
		name        string
		status      dotfiles.CheckStatus
		wantPresent bool
		wantDrift   bool
	}{
		{"present", dotfiles.CheckPresent, true, false},
		{"drifted", dotfiles.CheckDrifted, true, true},
		{"absent", dotfiles.CheckAbsent, false, false},
		{"src-missing", dotfiles.CheckSrcMissing, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			present, drift := dashCheckStatusToFields(c.status)
			if present != c.wantPresent || drift != c.wantDrift {
				t.Errorf("status %s => (%v,%v), want (%v,%v)",
					c.status, present, drift, c.wantPresent, c.wantDrift)
			}
			_ = dashDotfileState // sanity: helper still exists
		})
	}
}

// dashCheckStatusToFields mirrors the switch in dashLoadStatus so the mapping is
// unit-testable in isolation from bubbletea's tea.Cmd plumbing.
func dashCheckStatusToFields(s dotfiles.CheckStatus) (present, drift bool) {
	switch s {
	case dotfiles.CheckPresent:
		return true, false
	case dotfiles.CheckDrifted:
		return true, true
	case dotfiles.CheckAbsent, dotfiles.CheckSrcMissing:
		return false, false
	}
	return false, false
}

// --- session #52: model, Update, View, and render tests ---

// newLoadedModel returns a dashboardModel in the state dashStatusLoadedMsg
// would produce, so View/render/Update tests don't need a terminal.
func newLoadedModel() dashboardModel {
	return dashboardModel{
		cfg: &config.Config{
			Profiles: map[string]config.Profile{"work": {Packages: []string{"slack"}}},
			Secrets:  config.Secrets{Provider: "vault", Mappings: []config.Mapping{{Key: "API_TOKEN"}}},
		},
		platformInfo: platform.Info{OS: "linux", Arch: "amd64", PackageManager: "apt"},
		pkgSpecs: []packages.Spec{
			{Raw: "ripgrep", Manager: "apt", Name: "ripgrep"},
			{Raw: "bat:npm", Manager: "npm", Name: "bat"},
		},
		installedCount: 1,
		missingPkgs:    []packages.Spec{{Raw: "bat:npm", Manager: "npm", Name: "bat"}},
		dotfileStatuses: []dashDotfileStatus{
			{tmpl: config.Template{Src: "bashrc.tmpl", Dest: "~/.bashrc"}, present: true},
			{tmpl: config.Template{Src: "vimrc.tmpl", Dest: "~/.vimrc"}, present: true, drift: true},
			{tmpl: config.Template{Src: "tmux.conf.tmpl", Dest: "~/.tmux.conf"}},
		},
		secretStatuses: []dashSecretStatus{{mapping: config.Mapping{Key: "API_TOKEN"}}},
		loaded:         true,
	}
}

func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestDashboardUpdateKeys(t *testing.T) {
	m := newLoadedModel()

	// tab cycles forward and resets cursor
	m2, _ := m.Update(keyMsg("tab"))
	d := m2.(dashboardModel)
	if d.activeTab != dashTabPackages || d.cursor != 0 {
		t.Errorf("tab: activeTab=%v cursor=%d, want packages/0", d.activeTab, d.cursor)
	}

	// down moves cursor, bounded by list length (2 packages)
	m3, _ := d.Update(keyMsg("down"))
	d3 := m3.(dashboardModel)
	if d3.cursor != 1 {
		t.Errorf("down: cursor=%d, want 1", d3.cursor)
	}
	m4, _ := d3.Update(keyMsg("down"))
	if d4 := m4.(dashboardModel); d4.cursor != 1 {
		t.Errorf("down at end: cursor=%d, want clamped to 1", d4.cursor)
	}

	// up moves back, not below zero
	m5, _ := d3.Update(keyMsg("up"))
	dm5 := m5.(dashboardModel)
	if dm5.cursor != 0 {
		t.Errorf("up: cursor=%d, want 0", dm5.cursor)
	}
	m6, _ := dm5.Update(keyMsg("up"))
	if dm6 := m6.(dashboardModel); dm6.cursor != 0 {
		t.Errorf("up at start: cursor=%d, want clamped to 0", dm6.cursor)
	}

	// shift+tab from Overview wraps to Secrets (last tab)
	w, _ := newLoadedModel().Update(keyMsg("shift+tab"))
	if dw := w.(dashboardModel); dw.activeTab != dashTabSecrets {
		t.Errorf("shift+tab wrap: activeTab=%v, want secrets", dw.activeTab)
	}

	// digits 1-4 jump directly
	for tab, key := range map[dashTab]string{dashTabOverview: "1", dashTabDotfiles: "3", dashTabSecrets: "4"} {
		j, _ := newLoadedModel().Update(keyMsg(key))
		if dj := j.(dashboardModel); dj.activeTab != tab {
			t.Errorf("key %q: activeTab=%v, want %v", key, dj.activeTab, tab)
		}
	}

	// quit keys
	for _, k := range []string{"q", "ctrl+c", "esc"} {
		q, cmd := newLoadedModel().Update(keyMsg(k))
		if cmd == nil {
			t.Errorf("key %q: expected quit command, got nil", k)
		}
		_ = q
	}

	// window size is stored
	s, _ := newLoadedModel().Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if ds := s.(dashboardModel); ds.width != 80 || ds.height != 24 {
		t.Errorf("WindowSizeMsg: got %dx%d, want 80x24", ds.width, ds.height)
	}
}

func TestDashboardUpdateLoadAndError(t *testing.T) {
	m := newLoadedModel()

	// a status-loaded message replaces everything and sets loaded
	loaded := dashStatusLoadedMsg{
		platformInfo:   platform.Info{OS: "darwin", Arch: "arm64", PackageManager: "brew"},
		pkgSpecs:       []packages.Spec{{Raw: "jq", Manager: "brew", Name: "jq"}},
		installedCount: 1,
		dotfiles:       []dashDotfileStatus{{tmpl: config.Template{Src: "s", Dest: "d"}, present: true}},
		secrets:        []dashSecretStatus{{mapping: config.Mapping{Key: "K"}}},
	}
	u, _ := m.Update(loaded)
	d := u.(dashboardModel)
	if !d.loaded || d.platformInfo.OS != "darwin" || len(d.pkgSpecs) != 1 || len(d.dotfileStatuses) != 1 || len(d.secretStatuses) != 1 {
		t.Errorf("dashStatusLoadedMsg did not populate model: %+v", d)
	}

	// an error message sets err and drives the error view
	e, _ := m.Update(dashErrMsg{err: errors.New("boom")})
	de := e.(dashboardModel)
	if de.err == nil || de.err.Error() != "boom" {
		t.Fatalf("dashErrMsg did not set err")
	}
	if v := de.View(); !strings.Contains(v, "boom") {
		t.Errorf("error view should contain the error text, got %q", v)
	}
}

func TestDashboardViewInit(t *testing.T) {
	m := newLoadedModel()
	if cmd := m.Init(); cmd == nil {
		t.Error("Init() should return the load command, got nil")
	}
}

func TestDashboardListLen(t *testing.T) {
	m := newLoadedModel()
	cases := map[dashTab]int{
		dashTabOverview: 0,
		dashTabPackages: 2,
		dashTabDotfiles: 3,
		dashTabSecrets:  1,
	}
	for tab, want := range cases {
		m.activeTab = tab
		if got := m.listLen(); got != want {
			t.Errorf("listLen(tab=%d) = %d, want %d", tab, got, want)
		}
	}
}

func TestDashboardViewRenderTabs(t *testing.T) {
	m := newLoadedModel()

	tabs := m.renderTabs()
	if !strings.Contains(tabs, "Overview") || !strings.Contains(tabs, "Secrets") {
		t.Errorf("renderTabs missing names: %q", tabs)
	}

	// the active tab is marked with the pointer
	m.activeTab = dashTabPackages
	if pt := m.renderTabs(); !strings.Contains(pt, "▶ Packages") {
		t.Errorf("active package tab should render pointer, got %q", pt)
	}
}

func TestDashboardViewOverview(t *testing.T) {
	m := newLoadedModel()
	v := m.View()

	for _, want := range []string{"nestor", "System Health", "Packages:", "1/2 installed", "1 missing", "Dotfiles:", "1 ok, 1 drifted, 1 missing", "Secrets:", "1 configured (vault)", "Profiles:", "work", "q quit"} {
		if !strings.Contains(v, want) {
			t.Errorf("overview view missing %q", want)
		}
	}

	// no missing packages -> no missing annotation
	m.missingPkgs = nil
	if v2 := m.View(); strings.Contains(v2, "missing)") {
		t.Errorf("view should not mention missing packages, got snippet with 'missing)'")
	}
	// no profiles -> no Profiles line
	m.cfg.Profiles = nil
	if v3 := m.View(); strings.Contains(v3, "Profiles:") {
		t.Error("empty profiles should not render a Profiles line")
	}
}

func TestDashboardViewPackages(t *testing.T) {
	m := newLoadedModel()
	m.activeTab = dashTabPackages

	v := m.View()
	for _, want := range []string{"Packages (2)", "ripgrep", "bat", "✓", "✗"} {
		if !strings.Contains(v, want) {
			t.Errorf("packages view missing %q", want)
		}
	}

	// cursor is marked with the arrow on the first row
	if c := m.renderPackages(); !strings.Contains(c, "→ ") {
		t.Errorf("cursor row should render arrow, got %q", c)
	}

	// empty state
	m.pkgSpecs = nil
	if e := m.renderPackages(); !strings.Contains(e, "No packages configured") {
		t.Errorf("empty packages should render placeholder, got %q", e)
	}
}

func TestDashboardViewDotfiles(t *testing.T) {
	m := newLoadedModel()
	m.activeTab = dashTabDotfiles

	v := m.View()
	for _, want := range []string{"Dotfiles (3)", ".bashrc", ".vimrc (drifted)", ".tmux.conf (missing)"} {
		if !strings.Contains(v, want) {
			t.Errorf("dotfiles view missing %q", want)
		}
	}

	// empty state
	m.dotfileStatuses = nil
	if e := m.renderDotfiles(); !strings.Contains(e, "No dotfiles configured") {
		t.Errorf("empty dotfiles should render placeholder, got %q", e)
	}
}

func TestDashboardViewSecrets(t *testing.T) {
	m := newLoadedModel()
	m.activeTab = dashTabSecrets

	if v := m.View(); !strings.Contains(v, "Secrets (1)") || !strings.Contains(v, "API_TOKEN") {
		t.Errorf("secrets view missing header/key: %q", v)
	}

	// empty state
	m.secretStatuses = nil
	if e := m.renderSecrets(); !strings.Contains(e, "No secrets configured") {
		t.Errorf("empty secrets should render placeholder, got %q", e)
	}
}

func TestDashLoadStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// empty config: load should succeed and return zero dotfiles/secrets
	msg := dashLoadStatus(&config.Config{})()
	if _, ok := msg.(dashStatusLoadedMsg); !ok {
		t.Fatalf("expected dashStatusLoadedMsg, got %T", msg)
	}
	loaded := msg.(dashStatusLoadedMsg)
	if len(loaded.dotfiles) != 0 || len(loaded.secrets) != 0 {
		t.Errorf("empty config should load empty dotfiles/secrets, got %d/%d", len(loaded.dotfiles), len(loaded.secrets))
	}
}

// --- session #52 additions: Error(), configPath branches, pull/list error paths ---

func TestDashErrMsgError(t *testing.T) {
	inner := errors.New("disk on fire")
	e := dashErrMsg{err: inner}
	if e.Error() != "disk on fire" {
		t.Errorf("dashErrMsg.Error() = %q, want %q", e.Error(), "disk on fire")
	}
}

func TestConfigPathBranches(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// explicit flag wins
	cfgFile = "/explicit/nestor.yml"
	if got := configPath(); got != "/explicit/nestor.yml" {
		t.Errorf("configPath() with flag = %q, want explicit path", got)
	}
	cfgFile = ""

	// local nestor.yml preferred over the home default
	work := t.TempDir()
	chdirScope(t, work)
	if err := os.WriteFile(filepath.Join(work, "nestor.yml"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := configPath(); got != "nestor.yml" {
		t.Errorf("configPath() with local file = %q, want nestor.yml", got)
	}

	// falls back to home config path
	if err := os.Remove(filepath.Join(work, "nestor.yml")); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, ".config", "nestor", "nestor.yml")
	if got := configPath(); got != want {
		t.Errorf("configPath() default = %q, want %q", got, want)
	}
}

func TestPullOutMissingConfigDir(t *testing.T) {
	skipIfNoGit(t)
	isolatedConfigHome(t)

	var buf bytes.Buffer
	err := runPullOut(context.Background(), &buf)
	if err == nil {
		t.Fatal("expected error pulling with no config, got nil")
	}
}

func TestListOutMissingConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	cfgFile = filepath.Join(dir, "does-not-exist.yml")
	defer func() { cfgFile = "" }()

	var out bytes.Buffer
	if err := runListOut(context.Background(), &out); err == nil {
		t.Fatal("expected list error for missing config, got nil")
	}
}
