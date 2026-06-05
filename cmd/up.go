package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up",
	Short: "Bootstrap your dev environment from nestor.yml",
	Long: `Detects your OS, installs packages, deploys dotfiles,
injects secrets, and configures your shell. One command, full setup.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runUp(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(ctx context.Context) error {
	p := ui.New(os.Stdout)

	path := configPath()
	p.Info(fmt.Sprintf("loading config from %s", path))

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}

	p.OK(fmt.Sprintf("config v%d loaded", cfg.Version))
	p.Detail("dotfiles strategy", cfg.Dotfiles.Strategy)

	// Step 1: detect platform
	p.Header("platform")
	plat, err := platform.Detect()
	if err != nil {
		p.Warn(err.Error())
	} else {
		p.OK("platform detected")
	}
	p.Detail("os", plat.OS)
	p.Detail("arch", plat.Arch)
	p.Detail("package manager", plat.PackageManager)

	// Step 2: install packages
	p.Header("packages")
	resolver := packages.Resolver{
		Common: cfg.Packages.Common,
		Lists: map[string][]string{
			"macos": cfg.Packages.MacOS,
			"linux": cfg.Packages.Linux,
			"wsl":   cfg.Packages.WSL,
		},
	}
	specs := make([]packages.Spec, 0, len(resolver.Resolve(plat.OS)))
	for _, raw := range resolver.Resolve(plat.OS) {
		specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
	}

	if len(specs) == 0 {
		p.Info("no packages declared")
	} else {
		p.Info(fmt.Sprintf("installing %d packages", len(specs)))
		results := packages.InstallAll(specs, plat.PackageManager)
		installed, skipped, failed := 0, 0, 0
		for _, r := range results {
			switch r.Status {
			case packages.StatusInstalled:
				installed++
				p.OK(r.Spec.Name)
			case packages.StatusAlreadyInstalled:
				skipped++
			case packages.StatusError:
				failed++
				p.Error(fmt.Sprintf("%s: %v", r.Spec.Name, r.Err))
			}
		}
		p.Info(fmt.Sprintf("%d installed, %d already present, %d failed", installed, skipped, failed))
	}

	// TODO: deploy dotfiles from cfg.Dotfiles
	// TODO: inject secrets from cfg.Secrets
	// TODO: configure shell from cfg.Shells

	p.Header("result")
	p.OK("environment is up")

	return nil
}
