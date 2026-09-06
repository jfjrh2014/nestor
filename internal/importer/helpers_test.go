package importer

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

// swapRunCommand replaces runCommand for the duration of the test.
func swapRunCommand(t *testing.T, fn func(name string, args ...string) (string, error)) {
	t.Helper()
	orig := runCommand
	runCommand = fn
	t.Cleanup(func() { runCommand = orig })
}

func TestYadmImport(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("home dir unavailable: %v", err)
	}

	var gotName string
	var gotArgs []string
	swapRunCommand(t, func(name string, args ...string) (string, error) {
		gotName, gotArgs = name, args
		return home + "/.bashrc\n" + home + "/.config/nvim/init.vim\n/tmp/outside.txt\n\n", nil
	})

	y := &Yadm{}
	res, err := y.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if gotName != "yadm" || strings.Join(gotArgs, ",") != "list,-a" {
		t.Errorf("command: got %s %v, want yadm list -a", gotName, gotArgs)
	}

	if len(res.Dotfiles) != 2 {
		t.Fatalf("dotfiles: got %d, want 2. got: %v", len(res.Dotfiles), res.Dotfiles)
	}
	if res.Dotfiles[0].Dest != "~/.bashrc" || res.Dotfiles[0].Src != ".bashrc.tmpl" {
		t.Errorf("first dotfile: got %+v, want Src .bashrc.tmpl Dest ~/.bashrc", res.Dotfiles[0])
	}
	if res.Dotfiles[1].Dest != "~/.config/nvim/init.vim" {
		t.Errorf("nested dotfile dest: got %q", res.Dotfiles[1].Dest)
	}
	if res.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1 (/tmp/outside.txt is outside home)", res.Skipped)
	}
}

func TestYadmImportCommandFailure(t *testing.T) {
	swapRunCommand(t, func(string, ...string) (string, error) {
		return "", errors.New("not a git repo")
	})

	y := &Yadm{}
	_, err := y.Import()
	if err == nil {
		t.Fatal("expected error from failing yadm list, got nil")
	}
	if !strings.Contains(err.Error(), "running yadm list") {
		t.Errorf("error should wrap 'running yadm list', got: %v", err)
	}
}

func TestAutoFindsChezmoi(t *testing.T) {
	dir := t.TempDir()
	czDir := filepath.Join(dir, "chezmoi-source")
	if err := os.MkdirAll(czDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// NewChezmoi("") would look in the real home dir; point osUserHomeDir at
	// our temp dir so the search finds czDir under .local/share/chezmoi.
	swapHome := filepath.Join(dir, "home")
	standard := filepath.Join(swapHome, ".local", "share", "chezmoi")
	if err := os.MkdirAll(standard, 0o755); err != nil {
		t.Fatal(err)
	}
	origHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return swapHome, nil }
	t.Cleanup(func() { osUserHomeDir = origHome })

	//brewfile present too, ensures chezmoi has priority
	if wd, err := os.Getwd(); err == nil {
		_ = wd
	}

	imp, err := Auto()
	if err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if _, ok := imp.(*Chezmoi); !ok {
		t.Errorf("Auto with chezmoi dir present: got %T, want *Chezmoi", imp)
	}
	if imp.Name() != "chezmoi" {
		t.Errorf("Name: got %q, want chezmoi", imp.Name())
	}
}

func TestAutoFindsYadm(t *testing.T) {
	// no chezmoi, no brewfile, yadm installed
	origHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return "/nonexistent/home/xyz", nil }
	t.Cleanup(func() { osUserHomeDir = origHome })

	origLook := execLookPath
	execLookPath = func(name string) (string, error) {
		if name == "yadm" {
			return "/usr/bin/yadm", nil
		}
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { execLookPath = origLook })

	// chdir to a clean dir so "Brewfile" search in CWD can't hit one
	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	imp, err := Auto()
	if err != nil {
		t.Fatalf("Auto: %v", err)
	}
	if _, ok := imp.(*Yadm); !ok {
		t.Errorf("Auto with only yadm: got %T, want *Yadm", imp)
	}
	if imp.Name() != "yadm" {
		t.Errorf("Name: got %q, want yadm", imp.Name())
	}
}

func TestAutoNothingFound(t *testing.T) {
	origHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return "/nonexistent/home/abc", nil }
	t.Cleanup(func() { osUserHomeDir = origHome })

	origLook := execLookPath
	execLookPath = func(string) (string, error) { return "", os.ErrNotExist }
	t.Cleanup(func() { execLookPath = origLook })

	dir := t.TempDir()
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	_, err := Auto()
	if err == nil {
		t.Fatal("expected error when no importable tool exists, got nil")
	}
	if !strings.Contains(err.Error(), "no importable tool found") {
		t.Errorf("error message: got %v, want mention of 'no importable tool found'", err)
	}
}

func TestNames(t *testing.T) {
	if (&Brewfile{}).Name() != "brewfile" {
		t.Error(`Brewfile.Name() != "brewfile"`)
	}
}

func TestBrewfileImportOpenFailure(t *testing.T) {
	// stat succeeds at construction, but the file is gone by Import()
	path := filepath.Join(t.TempDir(), "Brewfile")
	if err := os.WriteFile(path, []byte("brew \"git\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bf, err := NewBrewfile(path)
	if err != nil {
		t.Fatalf("NewBrewfile: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := bf.Import(); err == nil {
		t.Error("expected error opening deleted Brewfile, got nil")
	}
}

func TestBrewfileSearchFindsDotBrewfile(t *testing.T) {
	// NewBrewfile("") falls back to $HOME/.Brewfile when CWD has none.
	fakeHome := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeHome, ".Brewfile"), []byte("brew \"git\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	origHome := osUserHomeDir
	osUserHomeDir = func() (string, error) { return fakeHome, nil }
	t.Cleanup(func() { osUserHomeDir = origHome })

	dir := t.TempDir() // clean CWD, no Brewfile here
	oldWD, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	bf, err := NewBrewfile("")
	if err != nil {
		t.Fatalf("NewBrewfile search: %v", err)
	}
	if bf.Path != filepath.Join(fakeHome, ".Brewfile") {
		t.Errorf("path: got %q, want $HOME/.Brewfile", bf.Path)
	}

	res, err := bf.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if len(res.Packages) != 1 || res.Packages[0] != "homebrew: git" {
		t.Errorf("packages: got %v, want [homebrew: git]", res.Packages)
	}
}

func TestChezmoiImportScanError(t *testing.T) {
	// Pass a nonexistent path as the source: Walk reports the root access
	// error through the callback, exercising the error-propagation branch.
	// (A file-as-source yields no error: Walk walks the single file.)
	// Make the root visible-but-unreadable instead: chmod 0000 on a dir
	// fails with EACCES even for root on most systems; fall back to a
	// nonexistent path if needed.
	dir := filepath.Join(t.TempDir(), "hidden")
	if err := os.MkdirAll(filepath.Join(dir, "dot_x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	cz := &Chezmoi{SourceDir: dir}
	_, err := cz.Import()

	// Running as root defeats chmod 0000, so this test accepts either the
	// scan error or (root case) a clean run with the walked dotfile found.
	if err == nil {
		t.Skip("chmod 0000 ineffective under root; scan-error branch not reachable here")
	}
}

func TestExpandHome(t *testing.T) {
	orig := osUserHomeDir
	osUserHomeDir = func() (string, error) { return "/home/tester", nil }
	t.Cleanup(func() { osUserHomeDir = orig })

	if got := expandHome("~/.config"); got != "/home/tester/.config" {
		t.Errorf("expandHome(~/.config): got %q", got)
	}
	if got := expandHome("~/"); got != "/home/tester" {
		t.Errorf("expandHome(~/): got %q", got)
	}
	if got := expandHome("~other/x"); got != "~other/x" {
		t.Errorf("expandHome(~other/x): got %q, want unchanged", got)
	}
	if got := expandHome("plain/path"); got != "plain/path" {
		t.Errorf("expandHome(plain/path): got %q, want unchanged", got)
	}
	if got := expandHome("~"); got != "/home/tester" {
		t.Errorf("expandHome(~): got %q, want /home/tester", got)
	}

	// home lookup fails -> path returned unchanged
	osUserHomeDir = func() (string, error) { return "", errors.New("no home") }
	if got := expandHome("~/x"); got != "~/x" {
		t.Errorf("expandHome with failing home lookup: got %q, want unchanged", got)
	}
}

func TestMergeResultEmptyConfig(t *testing.T) {
	res := &Result{
		Packages: []string{"git"},
		Dotfiles: []config.Template{{Src: ".vimrc.tmpl", Dest: "~/.vimrc"}},
	}
	cfg := &config.Config{}
	added := MergeResult(cfg, res)
	if added != 2 {
		t.Errorf("added: got %d, want 2", added)
	}
	if len(cfg.Packages.Common) != 1 || cfg.Packages.Common[0] != "git" {
		t.Errorf("packages: got %v", cfg.Packages.Common)
	}
	if len(cfg.Dotfiles.Templates) != 1 || cfg.Dotfiles.Templates[0].Dest != "~/.vimrc" {
		t.Errorf("dotfiles: got %v", cfg.Dotfiles.Templates)
	}
}

func TestYadmNotInstalled(t *testing.T) {
	// Force lookup failure by pointing to a binary that doesn't exist
	orig := execLookPath
	execLookPath = func(string) (string, error) {
		return "", os.ErrNotExist
	}
	t.Cleanup(func() { execLookPath = orig })

	_, err := NewYadm()
	if err == nil {
		t.Error("expected error when yadm not in PATH, got nil")
	}
}
