package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jfjrh2014/nestor/internal/config"
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

	// TODO: install packages based on platform.PackageManager
	// TODO: deploy dotfiles from cfg.Dotfiles
	// TODO: inject secrets from cfg.Secrets
	// TODO: configure shell from cfg.Shells

	p.Header("result")
	p.OK("environment is up")

	return nil
}
