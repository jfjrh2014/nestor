// Package importer converts external tool configs into nestor config entries.
package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfjrh2014/nestor/internal/config"
)

// Result holds what was imported.
type Result struct {
	Packages []string
	Dotfiles []config.Template
	Source   string
	Skipped  int
}

// Importer is the common interface for all import sources.
type Importer interface {
	Import() (*Result, error)
	Name() string
}

// Auto tries to detect which tool is in use and returns the right importer.
// Checks chezmoi first, then yadm, then Brewfile.
func Auto() (Importer, error) {
	if c, err := NewChezmoi(""); err == nil {
		return c, nil
	}
	if y, err := NewYadm(); err == nil {
		return y, nil
	}
	if b, err := NewBrewfile(""); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("no importable tool found (looked for chezmoi, yadm, Brewfile)")
}

// --- Brewfile ---

// Brewfile parses a Brewfile and extracts tap, brew, and cask entries.
type Brewfile struct {
	Path string
}

func NewBrewfile(path string) (*Brewfile, error) {
	if path == "" {
		// search CWD and ~
		for _, candidate := range []string{"Brewfile", "~/Brewfile"} {
			expanded := expandHome(candidate)
			if _, err := os.Stat(expanded); err == nil {
				path = expanded
				break
			}
		}
	}
	if path == "" {
		// try $HOME/.Brewfile as last resort
		if home, err := os.UserHomeDir(); err == nil {
			candidate := filepath.Join(home, ".Brewfile")
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	if path == "" {
		return nil, fmt.Errorf("no Brewfile found")
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("Brewfile not found: %w", err)
	}
	return &Brewfile{Path: path}, nil
}

func (b *Brewfile) Name() string { return "brewfile" }

func (b *Brewfile) Import() (*Result, error) {
	f, err := os.Open(b.Path)
	if err != nil {
		return nil, fmt.Errorf("opening Brewfile: %w", err)
	}
	defer f.Close()

	res := &Result{Source: b.Path}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == '#' {
			continue
		}

		// parse "brew 'name'", "tap 'user/repo'", "cask 'name'"
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		kind := parts[0]
		name := strings.Trim(parts[1], `"'`)

		switch kind {
		case "tap":
			res.Packages = append(res.Packages, "homebrew/tap: "+name)
		case "brew":
			res.Packages = append(res.Packages, "homebrew: "+name)
		case "cask":
			res.Packages = append(res.Packages, "homebrew/cask: "+name)
		case "mas":
			// Mac App Store - skip
			res.Skipped++
		}
	}
	return res, scanner.Err()
}

// --- chezmoi ---

// Chezmoi scans a chezmoi source directory for managed dotfiles.
type Chezmoi struct {
	SourceDir string
}

func NewChezmoi(sourceDir string) (*Chezmoi, error) {
	if sourceDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		sourceDir = filepath.Join(home, ".local", "share", "chezmoi")
	}
	if _, err := os.Stat(sourceDir); err != nil {
		return nil, fmt.Errorf("chezmoi source dir not found: %w", err)
	}
	return &Chezmoi{SourceDir: sourceDir}, nil
}

func (c *Chezmoi) Name() string { return "chezmoi" }

func (c *Chezmoi) Import() (*Result, error) {
	res := &Result{Source: c.SourceDir}

	err := filepath.Walk(c.SourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		rel, _ := filepath.Rel(c.SourceDir, path)
		// chezmoi encodes dotfiles with a leading "dot_" prefix
		// e.g. dot_bashrc -> .bashrc, dot_config/nvim/init.toml -> .config/nvim/init.toml
		dest := decodeChezmoiPath(rel)
		if dest == "" || dest == "__skip__" {
			res.Skipped++
			return nil
		}

		name := filepath.Base(dest)
		res.Dotfiles = append(res.Dotfiles, config.Template{
			Src:  name + ".tmpl",
			Dest: "~/" + dest,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scanning chezmoi dir: %w", err)
	}
	return res, nil
}

// chezmoi attribute prefixes that stack on top of dot_/exact_.
// Order matters: strip these first, then decode the core prefix.
var chezmoiAttrPrefixes = []string{"private_", "executable_", "readonly_", "create_"}

// decodeChezmoiPath converts a chezmoi source path to its target home-dir path.
// Returns "" if not importable, "__skip__" for intentionally skipped files.
func decodeChezmoiPath(rel string) string {
	parts := strings.Split(rel, string(filepath.Separator))
	var decoded []string
	for _, part := range parts {
		// chezmoi attribute prefixes stack: private_executable_dot_foo -> .foo
		// strip attribute prefixes first, then handle dot_ / exact_
		p := part
		for {
			stripped := false
			for _, prefix := range chezmoiAttrPrefixes {
				if strings.HasPrefix(p, prefix) {
					p = strings.TrimPrefix(p, prefix)
					stripped = true
					break
				}
			}
			if !stripped {
				break
			}
		}

		switch {
		case strings.HasPrefix(p, "dot_"):
			p = "." + strings.TrimPrefix(p, "dot_")
		case strings.HasPrefix(p, "exact_"):
			return "__skip__"
		case p == ".git" || strings.HasPrefix(p, ".git"):
			return "__skip__"
		}
		decoded = append(decoded, p)
	}

	// Only import dotfiles (things starting with .) - skip arbitrary files
	if len(decoded) == 0 {
		return ""
	}
	joined := filepath.Join(decoded...)
	if !strings.HasPrefix(decoded[0], ".") {
		return ""
	}
	return joined
}

// --- yadm ---

// Yadm uses `yadm list -a` to find managed files.
type Yadm struct{}

func NewYadm() (*Yadm, error) {
	// check if yadm is installed
	if _, err := execLookPath("yadm"); err != nil {
		return nil, fmt.Errorf("yadm not found in PATH")
	}
	return &Yadm{}, nil
}

func (y *Yadm) Name() string { return "yadm" }

func (y *Yadm) Import() (*Result, error) {
	// yadm list -a outputs the full paths of all managed files
	out, err := runCommand("yadm", "list", "-a")
	if err != nil {
		return nil, fmt.Errorf("running yadm list: %w", err)
	}

	res := &Result{Source: "yadm"}
	home, _ := os.UserHomeDir()

	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// convert absolute path to ~/relative
		rel := strings.TrimPrefix(line, home+"/")
		if rel == line {
			// outside home dir, skip
			res.Skipped++
			continue
		}

		name := filepath.Base(rel)
		res.Dotfiles = append(res.Dotfiles, config.Template{
			Src:  name + ".tmpl",
			Dest: "~/" + rel,
		})
	}
	return res, nil
}

// MergeResult merges an import result into a config.
// Deduplicates packages and dotfiles that already exist.
// Returns the count of new items added.
func MergeResult(cfg *config.Config, res *Result) int {
	added := 0

	// packages
	existingPkgs := map[string]bool{}
	for _, p := range cfg.Packages.Common {
		existingPkgs[p] = true
	}
	for _, p := range res.Packages {
		if !existingPkgs[p] {
			cfg.Packages.Common = append(cfg.Packages.Common, p)
			existingPkgs[p] = true
			added++
		}
	}

	// dotfiles
	existingDots := map[string]bool{}
	for _, d := range cfg.Dotfiles.Templates {
		existingDots[d.Dest] = true
	}
	for _, d := range res.Dotfiles {
		if !existingDots[d.Dest] {
			cfg.Dotfiles.Templates = append(cfg.Dotfiles.Templates, d)
			existingDots[d.Dest] = true
			added++
		}
	}

	return added
}
