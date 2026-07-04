package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jfjrh2014/nestor/internal/config"
)

func TestDoctorValidConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nestor.yml")
	cfgContent := `version: 1
packages:
  common:
    - git
dotfiles:
  source: ` + dir + `/dotfiles
  strategy: copy
  templates: []
secrets:
  provider: env
  mappings: []
shells:
  default: bash
`
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "dotfiles"), 0o755); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config load: %v", err)
	}
	if cfg.Version != 1 {
		t.Errorf("expected version 1, got %d", cfg.Version)
	}
	if len(cfg.Packages.Common) != 1 || cfg.Packages.Common[0] != "git" {
		t.Errorf("expected [git], got %v", cfg.Packages.Common)
	}
}

func TestSecretCLI(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"env", "env"},
		{"1password", "op"},
		{"bitwarden", "bw"},
		{"vault", "vault"},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := secretCLI(tt.provider)
		if got != tt.want {
			t.Errorf("secretCLI(%q) = %q, want %q", tt.provider, got, tt.want)
		}
	}
}

func TestScanDotfiles(t *testing.T) {
	dir := t.TempDir()

	// Create fake dotfiles
	for _, name := range []string{".bashrc", ".gitconfig"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("# test\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	templates := scanDotfiles(dir)
	if len(templates) != 2 {
		t.Fatalf("expected 2 dotfiles, got %d", len(templates))
	}

	found := map[string]bool{}
	for _, t := range templates {
		found[t.Src] = true
	}
	if !found[".bashrc.tmpl"] {
		t.Error("expected .bashrc.tmpl")
	}
	if !found[".gitconfig.tmpl"] {
		t.Error("expected .gitconfig.tmpl")
	}
}

func TestScanPackages(t *testing.T) {
	// On this machine git is installed via apt
	found := scanPackages("apt")
	if len(found) == 0 {
		t.Error("expected at least some packages to be found via apt")
	}

	hasGit := false
	for _, p := range found {
		if p == "git" {
			hasGit = true
		}
	}
	if !hasGit {
		t.Error("expected git to be detected via apt")
	}
}

func TestCheckPkgInstalled(t *testing.T) {
	// git is definitely installed
	if !checkPkgInstalled("apt", "git") {
		t.Error("git should be installed")
	}
	// Something that definitely isn't
	if checkPkgInstalled("apt", "nonexistent-pkg-xyz-12345") {
		t.Error("nonexistent package should not be installed")
	}
}

func TestMergeStrings(t *testing.T) {
	tests := []struct {
		name string
		dst  []string
		src  []string
		want []string
	}{
		{
			name: "appends new items preserving order",
			dst:  []string{"git", "curl"},
			src:  []string{"vim", "jq"},
			want: []string{"git", "curl", "vim", "jq"},
		},
		{
			name: "skips duplicates",
			dst:  []string{"git", "curl"},
			src:  []string{"git", "vim"},
			want: []string{"git", "curl", "vim"},
		},
		{
			name: "empty src returns dst unchanged",
			dst:  []string{"git"},
			src:  nil,
			want: []string{"git"},
		},
		{
			name: "empty dst absorbs all of src",
			dst:  nil,
			src:  []string{"git", "curl"},
			want: []string{"git", "curl"},
		},
		{
			name: "dedupes within src too",
			dst:  []string{"git"},
			src:  []string{"curl", "curl", "curl"},
			want: []string{"git", "curl"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeStrings(tt.dst, tt.src)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMergeDotfiles(t *testing.T) {
	dotfile := func(dest string) config.Template {
		return config.Template{Src: dest + ".tmpl", Dest: dest}
	}
	tests := []struct {
		name string
		dst  []config.Template
		src  []config.Template
		want []string // dest paths, in order
	}{
		{
			name: "appends new dests preserving order",
			dst:  []config.Template{dotfile("~/.bashrc")},
			src:  []config.Template{dotfile("~/.gitconfig"), dotfile("~/.vimrc")},
			want: []string{"~/.bashrc", "~/.gitconfig", "~/.vimrc"},
		},
		{
			name: "skips duplicate dest",
			dst:  []config.Template{dotfile("~/.bashrc")},
			src:  []config.Template{dotfile("~/.bashrc"), dotfile("~/.vimrc")},
			want: []string{"~/.bashrc", "~/.vimrc"},
		},
		{
			name: "empty src returns dst unchanged",
			dst:  []config.Template{dotfile("~/.bashrc")},
			src:  nil,
			want: []string{"~/.bashrc"},
		},
		{
			name: "empty dst absorbs all of src",
			dst:  nil,
			src:  []config.Template{dotfile("~/.bashrc"), dotfile("~/.gitconfig")},
			want: []string{"~/.bashrc", "~/.gitconfig"},
		},
		{
			name: "dedupes within src too",
			dst:  []config.Template{dotfile("~/.bashrc")},
			src:  []config.Template{dotfile("~/.vimrc"), dotfile("~/.vimrc")},
			want: []string{"~/.bashrc", "~/.vimrc"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeDotfiles(tt.dst, tt.src)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i].Dest != tt.want[i] {
					t.Errorf("index %d dest = %q, want %q", i, got[i].Dest, tt.want[i])
				}
			}
		})
	}
}
