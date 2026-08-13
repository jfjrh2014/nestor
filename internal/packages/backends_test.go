package packages

import (
	"errors"
	"testing"
)

// mockRunner records every command it was asked to run. By setting
// fail=true it simulates a command that exits non-zero, which the backends
// interpret as "not installed" (for IsInstalled) or "install failed" (for
// Install). The recorded calls let us assert exact command construction
// without hitting real package managers.
type mockRunner struct {
	calls []cmdCall
	fail  bool
}

type cmdCall struct {
	name string
	args []string
}

func (m *mockRunner) Run(name string, args ...string) error {
	m.calls = append(m.calls, cmdCall{name, append([]string{}, args...)})
	if m.fail {
		return errors.New("exit status 1")
	}
	return nil
}

// swapRunner replaces the package-level runner for the duration of a test
// and returns a restore func. All backend tests must use this so they don't
// accidentally shell out to real system tools.
func swapRunner(r cmdRunner) func() {
	old := runner
	runner = r
	return func() { runner = old }
}

// --- brew ---

func TestBrew_IsInstalled_Formula(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	b := brewMgr{}
	ok, _ := b.IsInstalled(Spec{Name: "git", Sub: ""})
	if !ok {
		t.Error("expected installed=true when runner succeeds")
	}
	want := cmdCall{"brew", []string{"list", "--formula", "git"}}
	if len(mr.calls) != 1 || mr.calls[0].name != want.name {
		t.Fatalf("got %v, want %v", mr.calls, want)
	}
	assertArgs(t, mr.calls[0].args, want.args)
}

func TestBrew_IsInstalled_Cask(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	b := brewMgr{}
	_, _ = b.IsInstalled(Spec{Name: "firefox", Sub: "cask"})
	want := cmdCall{"brew", []string{"list", "--cask", "firefox"}}
	assertArgs(t, mr.calls[0].args, want.args)
}

func TestBrew_IsInstalled_NotInstalled(t *testing.T) {
	mr := &mockRunner{fail: true}
	defer swapRunner(mr)()

	b := brewMgr{}
	ok, _ := b.IsInstalled(Spec{Name: "sl"})
	if ok {
		t.Error("expected installed=false when runner fails")
	}
}

func TestBrew_Install_Formula(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	b := brewMgr{}
	if err := b.Install(Spec{Name: "git"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"install", "git"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestBrew_Install_Cask(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	b := brewMgr{}
	if err := b.Install(Spec{Name: "firefox", Sub: "cask"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"install", "--cask", "firefox"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestBrew_Install_Fails(t *testing.T) {
	mr := &mockRunner{fail: true}
	defer swapRunner(mr)()

	b := brewMgr{}
	if err := b.Install(Spec{Name: "git"}); err == nil {
		t.Error("expected error when runner fails")
	}
}

// --- apt ---

func TestApt_IsInstalled(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	a := aptMgr{}
	ok, _ := a.IsInstalled(Spec{Name: "curl"})
	if !ok {
		t.Error("expected installed=true")
	}
	want := []string{"-s", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestApt_Install(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	a := aptMgr{}
	if err := a.Install(Spec{Name: "curl"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"install", "-y", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

// --- dnf ---

func TestDnf_IsInstalled(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	d := dnfMgr{}
	ok, _ := d.IsInstalled(Spec{Name: "curl"})
	if !ok {
		t.Error("expected installed=true")
	}
	want := []string{"-q", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestDnf_Install(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	d := dnfMgr{}
	if err := d.Install(Spec{Name: "curl"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"install", "-y", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

// --- pacman ---

func TestPacman_IsInstalled(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	p := pacmanMgr{}
	ok, _ := p.IsInstalled(Spec{Name: "curl"})
	if !ok {
		t.Error("expected installed=true")
	}
	want := []string{"-Q", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestPacman_Install(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	p := pacmanMgr{}
	if err := p.Install(Spec{Name: "curl"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"-S", "--noconfirm", "curl"}
	assertArgs(t, mr.calls[0].args, want)
}

// --- snap ---

func TestSnap_IsInstalled(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	s := snapMgr{}
	ok, _ := s.IsInstalled(Spec{Name: "code"})
	if !ok {
		t.Error("expected installed=true")
	}
	want := []string{"list", "code"}
	assertArgs(t, mr.calls[0].args, want)
}

func TestSnap_Install(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	s := snapMgr{}
	if err := s.Install(Spec{Name: "code"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"install", "code"}
	assertArgs(t, mr.calls[0].args, want)
}

// --- InstallAll orchestration ---

func TestInstallAll_AllPaths(t *testing.T) {
	// runner reports git as installed (dpkg -s succeeds), curl as missing
	// (dpkg -s fails, apt-get install succeeds), and the third spec uses
	// an unknown manager to trigger the error path without hitting the runner.
	mr := &binaryRunner{installed: map[string]bool{
		"git": true,
	}}
	defer swapRunner(mr)()

	specs := []Spec{
		{Raw: "git", Manager: "apt", Name: "git"},
		{Raw: "curl", Manager: "apt", Name: "curl"},
		{Raw: "weird", Manager: "unknown-mgr", Name: "weird"},
	}

	results := InstallAll(specs, "apt")
	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}

	// git: already installed
	if results[0].Status != StatusAlreadyInstalled {
		t.Errorf("spec 0: got %s, want %s", results[0].Status, StatusAlreadyInstalled)
	}
	// curl: installed
	if results[1].Status != StatusInstalled {
		t.Errorf("spec 1: got %s, want %s", results[1].Status, StatusInstalled)
	}
	// weird: error (unknown manager)
	if results[2].Status != StatusError {
		t.Errorf("spec 2: got %s, want %s", results[2].Status, StatusError)
	}
}

func TestInstallAll_Empty(t *testing.T) {
	defer swapRunner(&mockRunner{})

	results := InstallAll(nil, "apt")
	if len(results) != 0 {
		t.Errorf("got %d results for nil input, want 0", len(results))
	}
}

func TestInstallAll_ReusesManagerAcrossSpecs(t *testing.T) {
	mr := &mockRunner{}
	defer swapRunner(mr)()

	// Two specs with the same manager. IsInstalled fails for both (mockRunner
	// always succeeds, so IsInstalled succeeds, and both return already-installed).
	// That's 2 IsInstalled calls total.
	specs := []Spec{
		{Raw: "curl", Manager: "apt", Name: "curl"},
		{Raw: "wget", Manager: "apt", Name: "wget"},
	}
	results := InstallAll(specs, "apt")
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for i, r := range results {
		if r.Status != StatusAlreadyInstalled {
			t.Errorf("spec %d: got %s, want %s", i, r.Status, StatusAlreadyInstalled)
		}
	}
	// 2 IsInstalled calls (both succeed so Install is not called)
	if len(mr.calls) != 2 {
		t.Errorf("expected 2 runner calls, got %d", len(mr.calls))
	}
}

// --- helpers ---

// binaryRunner simulates a more realistic package backend: dpkg -s succeeds
// if the package is in the installed map, fails otherwise. All other commands
// (e.g. apt-get install) succeed unconditionally.
type binaryRunner struct {
	installed map[string]bool
	calls     []cmdCall
}

func (b *binaryRunner) Run(name string, args ...string) error {
	b.calls = append(b.calls, cmdCall{name, append([]string{}, args...)})
	if name == "dpkg" && len(args) >= 2 {
		pkg := args[len(args)-1]
		if b.installed[pkg] {
			return nil
		}
		return errors.New("not installed")
	}
	return nil
}

func assertArgs(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("arg count: got %d (%v), want %d (%v)", len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("arg %d: got %q, want %q", i, got[i], want[i])
		}
	}
}
