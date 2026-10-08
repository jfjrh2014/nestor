package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/jfjrh2014/nestor/internal/fsutil"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter nestor.yml in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInit(os.Stdout)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

const starterConfig = `version: 1

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

func runInit(w io.Writer) error {
	target := "nestor.yml"
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists, not overwriting", target)
	}

	if err := fsutil.WriteFileSync(target, []byte(starterConfig), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}

	fmt.Fprintf(w, "nestor: created %s — edit it and run nestor up\n", target)
	return nil
}
