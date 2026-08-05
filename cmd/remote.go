package cmd

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/jfjrh2014/nestor/internal/vcs"
	"github.com/spf13/cobra"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Manage the git remote for your config",
	Long: `Configure and inspect the git remote used by 'nestor push' and 'nestor pull'.

Subcommands:
  nestor remote add <url>     Set the origin remote URL
  nestor remote show          Print the configured remote URL
  nestor remote remove        Remove the origin remote`,
	// bare 'nestor remote' with no subcommand → show
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoteShow()
	},
}

func init() {
	remoteCmd.AddCommand(remoteAddCmd)
	remoteCmd.AddCommand(remoteShowCmd)
	remoteCmd.AddCommand(remoteRemoveCmd)
	rootCmd.AddCommand(remoteCmd)
}

var remoteAddCmd = &cobra.Command{
	Use:   "add <url>",
	Short: "Set the git remote URL for config sync",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoteAdd(args[0])
	},
}

var remoteShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the configured remote URL",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoteShow()
	},
}

var remoteRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove the configured git remote",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runRemoteRemove()
	},
}

func runRemoteAdd(url string) error {
	return runRemoteAddOut(url, os.Stdout)
}

func runRemoteAddOut(url string, w io.Writer) error {
	p := ui.New(w)
	if !vcs.HasGit() {
		return fmt.Errorf("git not found on PATH")
	}
	dir, err := nestorConfigDir()
	if err != nil {
		return fmt.Errorf("finding config dir: %w", err)
	}
	if err := vcs.Init(dir); err != nil {
		return fmt.Errorf("init repo: %w", err)
	}
	if err := vcs.SetRemote(dir, remoteName, url); err != nil {
		return fmt.Errorf("setting remote: %w", err)
	}
	p.OK(fmt.Sprintf("remote 'origin' set to %s", url))
	return nil
}

func runRemoteShow() error {
	return runRemoteShowOut(os.Stdout)
}

func runRemoteShowOut(w io.Writer) error {
	p := ui.New(w)
	dir, err := nestorConfigDir()
	if err != nil {
		return fmt.Errorf("finding config dir: %w", err)
	}
	url := vcs.GetRemote(dir, remoteName)
	if url == "" {
		p.Info("no remote configured (run 'nestor remote add <url>' to set one)")
		return nil
	}
	p.Info(url)
	return nil
}

func runRemoteRemove() error {
	return runRemoteRemoveOut(os.Stdout)
}

func runRemoteRemoveOut(w io.Writer) error {
	p := ui.New(w)
	dir, err := nestorConfigDir()
	if err != nil {
		return fmt.Errorf("finding config dir: %w", err)
	}
	url := vcs.GetRemote(dir, remoteName)
	if url == "" {
		p.Info("no remote configured (nothing to remove)")
		return nil
	}
	out, err := exec.Command("git", "-C", dir, "remote", "remove", remoteName).CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing remote: %w: %s", err, strings.TrimSpace(string(out)))
	}
	p.OK(fmt.Sprintf("removed remote 'origin' (was %s)", url))
	return nil
}
