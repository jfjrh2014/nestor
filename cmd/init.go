package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter nestor.yml in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit()
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit() error {
	target := "nestor.yml"
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists, not overwriting", target)
	}

	starter := `version: 1

packages:
  common:
    - git
    - neovim
    - ripgrep
    - fd
    - bat
    - fzf
    - tmux
    - jq

dotfiles:
  source: ~/.config/nestor/dotfiles
  strategy: copy
  templates: []

secrets:
  provider: env
  mappings: []

shells:
  default: zsh
  plugins:
    - zsh-users/zsh-autosuggestions
    - zsh-users/zsh-syntax-highlighting
    - starship

profiles: {}
`

	if err := os.WriteFile(target, []byte(starter), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}

	fmt.Printf("nestor: created %s — edit it and run nestor up\n", target)
	return nil
}
