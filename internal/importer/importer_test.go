package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestBrewfileImport(t *testing.T) {
	dir := t.TempDir()
	brewfile := `# my brewfile
brew "git"
brew "ripgrep"
cask "visual-studio-code"
tap "hashicorp/tap"
mas "Xcode", id: 497799835
brew "neovim"
`
	path := filepath.Join(dir, "Brewfile")
	if err := os.WriteFile(path, []byte(brewfile), 0o644); err != nil {
		t.Fatal(err)
	}

	bf, err := NewBrewfile(path)
	if err != nil {
		t.Fatalf("NewBrewfile: %v", err)
	}

	res, err := bf.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	// git, ripgrep, neovim (brew) + visual-studio-code (cask) + hashicorp/tap (tap) = 5
	wantPkgs := 5
	if got := len(res.Packages); got != wantPkgs {
		t.Errorf("packages: got %d (%v), want %d", got, res.Packages, wantPkgs)
	}

	// mas entry should be skipped
	if res.Skipped != 1 {
		t.Errorf("skipped: got %d, want 1 (mas entry)", res.Skipped)
	}

	// verify homebrew: prefix on brew entries
	for _, p := range res.Packages {
		switch {
		case strings.Contains(p, "git") && !strings.Contains(p, "tap"):
			if p != "homebrew: git" {
				t.Errorf("git package: got %q, want %q", p, "homebrew: git")
			}
		case strings.Contains(p, "visual-studio-code"):
			if p != "homebrew/cask: visual-studio-code" {
				t.Errorf("cask: got %q", p)
			}
		}
	}
}

func TestBrewfileMerge(t *testing.T) {
	dir := t.TempDir()
	brewfile := `brew "git"
brew "neovim"
`
	path := filepath.Join(dir, "Brewfile")
	os.WriteFile(path, []byte(brewfile), 0o644)

	bf, _ := NewBrewfile(path)
	res, _ := bf.Import()

	// config already has "homebrew: git" (same format the brewfile importer produces)
	cfg := &config.Config{
		Packages: config.Packages{
			Common: []string{"homebrew: git", "homebrew: curl"},
		},
	}

	added := MergeResult(cfg, res)
	if added != 1 {
		t.Errorf("added: got %d, want 1 (neovim is new, git already exists)", added)
	}

	// verify neovim was added
	found := false
	for _, p := range cfg.Packages.Common {
		if p == "homebrew: neovim" {
			found = true
		}
	}
	if !found {
		t.Errorf("neovim not found in config after merge, got: %v", cfg.Packages.Common)
	}
}

func TestChezmoiImport(t *testing.T) {
	dir := t.TempDir()
	chezmoiDir := filepath.Join(dir, "chezmoi")
	files := map[string]string{
		"dot_bashrc":               "# bash config\n",
		"dot_gitconfig":            "[user]\n",
		"dot_vimrc":                "\" vim\n",
		"dot_config/nvim/init.vim": "\" nvim\n",
		"private_dot_ssh/config":   "Host github\n",
		".git/info/exclude":        "",
		"README.md":                "# readme\n",
	}
	for relPath, content := range files {
		full := filepath.Join(chezmoiDir, relPath)
		os.MkdirAll(filepath.Dir(full), 0o755)
		os.WriteFile(full, []byte(content), 0o644)
	}

	cz, err := NewChezmoi(chezmoiDir)
	if err != nil {
		t.Fatalf("NewChezmoi: %v", err)
	}

	res, err := cz.Import()
	if err != nil {
		t.Fatalf("Import: %v", err)
	}

	if len(res.Dotfiles) != 5 {
		t.Errorf("dotfiles: got %d, want 5 (bashrc, gitconfig, vimrc, nvim/init.vim, ssh/config). got: %v",
			len(res.Dotfiles), res.Dotfiles)
	}

	// verify a decode
	foundBashrc := false
	for _, d := range res.Dotfiles {
		if d.Dest == "~/.bashrc" {
			foundBashrc = true
			if d.Src != ".bashrc.tmpl" {
				t.Errorf("bashrc src: got %q, want %q", d.Src, ".bashrc.tmpl")
			}
		}
	}
	if !foundBashrc {
		t.Errorf("~/.bashrc not found in imports: %v", res.Dotfiles)
	}
}

func TestChezmoiPathDecode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"dot_bashrc", ".bashrc"},
		{"dot_config/nvim/init.vim", ".config/nvim/init.vim"},
		{"private_dot_ssh/config", ".ssh/config"},
		{"exact_dot_config/fish/config.fish", "__skip__"},
	}
	for _, tt := range tests {
		got := decodeChezmoiPath(tt.input)
		if got != tt.want {
			t.Errorf("decodeChezmoiPath(%q): got %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMergeResultDedup(t *testing.T) {
	res := &Result{
		Packages: []string{"git", "neovim"},
		Dotfiles: []config.Template{
			{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
			{Src: ".zshrc.tmpl", Dest: "~/.zshrc"},
		},
	}

	cfg := &config.Config{
		Packages: config.Packages{
			Common: []string{"git"},
		},
		Dotfiles: config.Dotfiles{
			Templates: []config.Template{
				{Src: ".bashrc.tmpl", Dest: "~/.bashrc"},
			},
		},
	}

	added := MergeResult(cfg, res)
	if added != 2 { // neovim (pkg) + zshrc (dotfile)
		t.Errorf("added: got %d, want 2", added)
	}

	if len(cfg.Packages.Common) != 2 {
		t.Errorf("packages after merge: got %d, want 2", len(cfg.Packages.Common))
	}
	if len(cfg.Dotfiles.Templates) != 2 {
		t.Errorf("dotfiles after merge: got %d, want 2", len(cfg.Dotfiles.Templates))
	}
}

func TestBrewfileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := NewBrewfile(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Error("expected error for nonexistent Brewfile, got nil")
	}
}

func TestChezmoiNotFound(t *testing.T) {
	_, err := NewChezmoi("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for nonexistent chezmoi dir, got nil")
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
