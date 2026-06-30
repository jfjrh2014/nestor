package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/jfjrh2014/nestor/internal/vcs"
	"github.com/spf13/cobra"
)

var pullRemoteURL string

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull config changes from a git remote",
	Long: `Fetch and merge config updates from your git remote.

Use this on a second machine to pull the latest nestor.yml and dotfile
templates that you pushed from elsewhere.

Examples:
  nestor pull --remote https://github.com/you/dotfiles.git
  nestor pull            # uses previously configured remote`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runPull(cmd.Context())
	},
}

func init() {
	pullCmd.Flags().StringVar(&pullRemoteURL, "remote", "", "git remote URL to pull from")
	rootCmd.AddCommand(pullCmd)
}

func runPull(ctx context.Context) error {
	return runPullOut(ctx, os.Stdout)
}

func runPullOut(ctx context.Context, w io.Writer) error {
	p := ui.New(w)

	if !vcs.HasGit() {
		return fmt.Errorf("git not found on PATH — install git to use config sync")
	}

	dir, err := nestorConfigDir()
	if err != nil {
		return fmt.Errorf("finding config dir: %w", err)
	}

	// Initialize repo if needed (so pull can register the remote)
	isNew := !vcs.IsRepo(dir)
	if err := vcs.Init(dir); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	if isNew {
		p.OK(fmt.Sprintf("initialized git repo in %s", dir))
	}

	// Configure remote if a URL was provided
	if pullRemoteURL != "" {
		if err := vcs.SetRemote(dir, remoteName, pullRemoteURL); err != nil {
			return fmt.Errorf("setting remote: %w", err)
		}
		p.OK(fmt.Sprintf("remote 'origin' set to %s", pullRemoteURL))
	}

	if !vcs.RemoteSet(dir, remoteName) {
		return fmt.Errorf("no remote configured — set one with --remote or 'nestor remote add <url>'")
	}

	// Warn about uncommitted local changes before pulling
	has, err := vcs.HasChanges(dir)
	if err != nil {
		return fmt.Errorf("checking status: %w", err)
	}
	if has {
		p.Warn("you have uncommitted local changes — commit them first ('nestor push') to avoid conflicts")
	}

	p.Info(fmt.Sprintf("pulling from %s ...", vcs.GetRemote(dir, remoteName)))
	if err := vcs.Pull(dir, remoteName); err != nil {
		return fmt.Errorf("pull: %w", err)
	}

	p.OK("pulled latest config from remote")
	p.Info("run 'nestor up' to apply any changes")
	return nil
}
