package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Capture current machine state into nestor.yml",
	Long: `Scans your machine for installed packages and common dotfiles,
then generates or updates nestor.yml to match the live state.
Useful for bootstrapping nestor from an existing setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSync(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func runSync(ctx context.Context) error {
	p := ui.New(os.Stdout)

	// Detect platform
	p.Header("platform")
	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	p.OK(fmt.Sprintf("os=%s arch=%s pkgmgr=%s", plat.OS, plat.Arch, plat.PackageManager))

	// Build config from live state
	cfg := &config.Config{
		Version: 1,
		Packages: config.Packages{
			Common: []string{},
		},
		Dotfiles: config.Dotfiles{
			Source:    "",
			Strategy:  "copy",
			Templates: []config.Template{},
		},
		Secrets: config.Secrets{
			Provider: "env",
			Mappings: []config.Mapping{},
		},
		Shells: config.Shells{
			Default: "bash",
			Plugins: []string{},
		},
		Profiles: map[string]config.Profile{},
	}

	// Scan packages
	p.Header("packages")
	foundPkgs := scanPackages(plat.PackageManager)
	if len(foundPkgs) > 0 {
		cfg.Packages.Common = foundPkgs
		p.OK(fmt.Sprintf("found %d packages", len(foundPkgs)))
	} else {
		p.Info("no packages detected")
	}

	// Scan dotfiles
	p.Header("dotfiles")
	home, _ := os.UserHomeDir()
	defaultSourceDir := filepath.Join(home, ".config", "nestor", "dotfiles")
	cfg.Dotfiles.Source = defaultSourceDir

	foundDots := scanDotfiles(home)
	if len(foundDots) > 0 {
		cfg.Dotfiles.Templates = foundDots
		p.OK(fmt.Sprintf("found %d common dotfiles", len(foundDots)))
	} else {
		p.Info("no common dotfiles found in home dir")
	}

	// Write config
	p.Header("config")
	outPath := configPath()

	// Load the existing config before replacing it. A load failure is fatal:
	// sync is about to overwrite this file, and silently discarding a config
	// we failed to parse would destroy hand-maintained mappings over one typo.
	existing, existingErr := loadExistingForSync(outPath)
	if existingErr != nil {
		return existingErr
	}
	if existing != nil {
		p.Warn(fmt.Sprintf("config already exists at %s — merging", outPath))
		existing.Packages.Common = mergeStrings(existing.Packages.Common, foundPkgs)
		existing.Dotfiles.Templates = mergeDotfiles(existing.Dotfiles.Templates, foundDots)
		// Preserve the freshly-computed source dir if the existing config has
		// none — otherwise the merged config points at templates with no source.
		if existing.Dotfiles.Source == "" {
			existing.Dotfiles.Source = defaultSourceDir
		}
		cfg = existing
	}

	// Materialize detected dotfiles as templates in the resolved source dir. This
	// runs AFTER the merge so it uses the effective source (existing custom or
	// default), not always the default — otherwise templates land in the wrong
	// place when the existing config has a custom source, and `nestor up` fails
	// with CheckSrcMissing for every newly-detected dotfile.
	if len(foundDots) > 0 {
		effectiveSource := cfg.Dotfiles.Source
		if effectiveSource == "" {
			effectiveSource = defaultSourceDir
		}
		if err := os.MkdirAll(effectiveSource, 0o755); err != nil {
			return fmt.Errorf("sync: create source dir: %w", err)
		}
		copied, skipped, copyErr := copyDotfileTemplates(home, effectiveSource, foundDots)
		if copyErr != nil {
			p.Warn(fmt.Sprintf("dotfile copy: %v", copyErr))
		}
		if copied > 0 {
			p.OK(fmt.Sprintf("copied %d template(s) into %s", copied, effectiveSource))
		}
		if skipped > 0 {
			p.Info(fmt.Sprintf("kept %d existing template(s) (not re-copied)", skipped))
		}
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("sync: %w", err)
	}

	dir := filepath.Dir(outPath)
	if dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0o755)
	}

	if writeErr := os.WriteFile(outPath, data, 0o644); writeErr != nil {
		return fmt.Errorf("sync: %w", writeErr)
	}
	p.OK(fmt.Sprintf("config written to %s", outPath))

	return nil
}

// loadExistingForSync returns the parsed existing config at path, or nil when
// no file exists there yet. Any other read or parse error is returned: sync
// is about to overwrite the file, so refusing to proceed is the only safe
// answer — proceeding would silently discard the user's hand-maintained config.
func loadExistingForSync(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("sync: checking existing config: %w", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("sync: existing config at %s is invalid, refusing to overwrite: %w", path, err)
	}
	return cfg, nil
}

// commonDotfiles to look for in $HOME.
var commonDotfiles = []struct {
	name string
	dest string
}{
	{".bashrc", "~/.bashrc"},
	{".zshrc", "~/.zshrc"},
	{".gitconfig", "~/.gitconfig"},
	{".tmux.conf", "~/.tmux.conf"},
	{".vimrc", "~/.vimrc"},
	{".config/nvim/init.vim", "~/.config/nvim/init.vim"},
}

// devPackageCandidates maps package managers to common dev tools to probe for.
var devPackageCandidates = map[string][]string{
	"apt":    {"git", "curl", "wget", "vim", "neovim", "tmux", "jq", "ripgrep", "fd-find", "bat", "fzf", "zsh", "build-essential"},
	"brew":   {"git", "curl", "wget", "vim", "neovim", "tmux", "jq", "ripgrep", "fd", "bat", "fzf", "zsh"},
	"dnf":    {"git", "curl", "wget", "vim", "tmux", "jq", "ripgrep", "fd-find", "bat", "fzf", "zsh"},
	"pacman": {"git", "curl", "wget", "vim", "neovim", "tmux", "jq", "ripgrep", "fd", "bat", "fzf", "zsh"},
	"snap":   {"git"},
}

func scanPackages(pkgMgr string) []string {
	candidates, ok := devPackageCandidates[pkgMgr]
	if !ok {
		return nil
	}

	found := []string{}
	for _, name := range candidates {
		if checkPkgInstalled(pkgMgr, name) {
			found = append(found, name)
		}
	}
	return found
}

func checkPkgInstalled(pkgMgr, name string) bool {
	var cmd *exec.Cmd
	switch pkgMgr {
	case "apt":
		cmd = exec.Command("dpkg", "-s", name)
	case "brew":
		cmd = exec.Command("brew", "list", "--formula", name)
	case "dnf":
		cmd = exec.Command("rpm", "-q", name)
	case "pacman":
		cmd = exec.Command("pacman", "-Q", name)
	case "snap":
		cmd = exec.Command("snap", "list", name)
	default:
		return false
	}
	return cmd.Run() == nil
}

func scanDotfiles(home string) []config.Template {
	templates := []config.Template{}
	for _, d := range commonDotfiles {
		fullPath := filepath.Join(home, d.name)
		if _, err := os.Stat(fullPath); err == nil {
			templates = append(templates, config.Template{
				Src:  d.name + ".tmpl",
				Dest: d.dest,
			})
		}
	}
	return templates
}

// copyDotfileTemplates materializes detected dotfiles as template files in the
// source dir, so the generated config points at files that actually exist. A
// template's Src (e.g. ".bashrc.tmpl") is mapped back to the detected file
// (".bashrc") in the home dir. Returns the number of files copied and skipped.
//
// Templates that already exist in the source dir are left untouched: after a
// first sync they are the user's working copies (edited, merged, secret
// templated), and the live home file drifts from them the moment either side
// changes. Re-copying would silently destroy those edits on every re-sync.
func copyDotfileTemplates(home, sourceDir string, templates []config.Template) (int, int, error) {
	copied := 0
	skipped := 0
	var firstErr error
	for _, t := range templates {
		// t.Src is like ".bashrc.tmpl" — strip the ".tmpl" suffix to recover the
		// real file name, then resolve it under home.
		srcFile := strings.TrimSuffix(t.Src, ".tmpl")
		src := filepath.Join(home, srcFile)
		dest := filepath.Join(sourceDir, t.Src)
		if _, err := os.Stat(dest); err == nil {
			skipped++
			continue
		}
		if err := copyFileSynced(src, dest); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		copied++
	}
	return copied, skipped, firstErr
}

// copyFileSynced copies src to dest, preserving file mode.
func copyFileSynced(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	info, _ := in.Stat()
	if info != nil {
		_ = out.Chmod(info.Mode())
	}
	return out.Close()
}

// mergeStrings appends items from src to dst that are not already present,
// preserving original order. Returns dst if src is empty.
func mergeStrings(dst, src []string) []string {
	if len(src) == 0 {
		return dst
	}
	seen := map[string]bool{}
	for _, s := range dst {
		seen[s] = true
	}
	for _, s := range src {
		if !seen[s] {
			dst = append(dst, s)
			seen[s] = true
		}
	}
	return dst
}

// mergeDotfiles appends templates from src to dst whose Dest is not already
// present, preserving original order. Returns dst if src is empty.
func mergeDotfiles(dst, src []config.Template) []config.Template {
	if len(src) == 0 {
		return dst
	}
	seen := map[string]bool{}
	for _, t := range dst {
		seen[t.Dest] = true
	}
	for _, t := range src {
		if !seen[t.Dest] {
			dst = append(dst, t)
			seen[t.Dest] = true
		}
	}
	return dst
}
