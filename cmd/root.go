package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "nestor",
	Short: "Your dev environment, from zero to coding.",
	Long: `nestor manages your full developer environment lifecycle:
packages, dotfiles, secrets, shell config — one config, one command.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute(ctx context.Context) error {
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "", "config file (default ~/.config/nestor/nestor.yml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func configPath() string {
	if cfgFile != "" {
		return cfgFile
	}
	// prefer local nestor.yml if present
	if _, err := os.Stat("nestor.yml"); err == nil {
		return "nestor.yml"
	}
	home, _ := os.UserHomeDir()
	return fmt.Sprintf("%s/.config/nestor/nestor.yml", home)
}
