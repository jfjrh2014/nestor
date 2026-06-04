package cmd

import (
	"context"
	"fmt"

	"github.com/jfjrh2014/nestor/internal/config"
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
	path := configPath()

	fmt.Printf("nestor: loading config from %s\n", path)

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}

	fmt.Printf("nestor: config v%d loaded (dotfiles strategy: %s)\n", cfg.Version, cfg.Dotfiles.Strategy)

	// TODO: Phase 1 — implement each step
	fmt.Println("nestor: detecting OS...")
	fmt.Println("nestor: installing packages...")
	fmt.Println("nestor: deploying dotfiles...")
	fmt.Println("nestor: injecting secrets...")
	fmt.Println("nestor: configuring shell...")
	fmt.Println("nestor: ✓ environment is up")

	return nil
}
