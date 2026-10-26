package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jfjrh2014/nestor/internal/secrets"
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
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Hand the live config path to the engine layers that guard against
		// writing into nestor's own config. Without this, a guard whose
		// default only knows the XDG location lets `nestor up -c /work/my.yml`
		// with a self-referential inject target rewrite the config driving
		// the run on every invocation (session #128). Idempotent and cheap:
		// a plain string setter, run once per command.
		secrets.SetConfigPathFn(configPath)
	},
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
