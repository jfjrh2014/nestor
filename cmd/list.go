package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all managed packages, dotfiles, and secrets with status",
	Long: `Shows everything nestor manages and whether each item is
installed/present or missing. Useful quick health check.`,
	Aliases: []string{"ls"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return runList(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(ctx context.Context) error {
	return runListOut(ctx, os.Stdout)
}

func runListOut(_ context.Context, w io.Writer) error {
	p := ui.New(w)

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}

	total, ok, missing := 0, 0, 0

	// --- packages ---
	p.Header("packages")
	resolver := packages.Resolver{
		Common: cfg.Packages.Common,
		Lists: map[string][]string{
			"macos": cfg.Packages.MacOS,
			"linux": cfg.Packages.Linux,
			"wsl":   cfg.Packages.WSL,
		},
	}

	for _, raw := range resolver.Resolve(plat.OS) {
		spec := packages.ParseSpec(raw, plat.PackageManager)
		total++
		mgr, err := packages.NewManager(spec.Manager)
		if err != nil {
			missing++
			p.Error(fmt.Sprintf("%-30s  unknown manager: %s", spec.Name, spec.Manager))
			continue
		}
		installed, _ := mgr.IsInstalled(spec)
		if installed {
			ok++
			p.OK(fmt.Sprintf("%-30s  installed", spec.Name))
		} else {
			missing++
			p.Warn(fmt.Sprintf("%-30s  missing", spec.Name))
		}
	}
	if total == 0 {
		p.Info("no packages declared")
	}

	// --- dotfiles ---
	p.Header("dotfiles")
	dotTotal, dotOk, dotMissing := 0, 0, 0
	if len(cfg.Dotfiles.Templates) == 0 {
		p.Info("no templates declared")
	} else {
		strategy := dotfiles.Strategy(cfg.Dotfiles.Strategy)
		if strategy == "" {
			strategy = dotfiles.StrategyCopy
		}
		source := cfg.Dotfiles.Source
		if source == "" {
			home, _ := os.UserHomeDir()
			source = filepath.Join(home, ".config", "nestor", "dotfiles")
		}

		for _, t := range cfg.Dotfiles.Templates {
			dotTotal++
			deployer := dotfiles.Deployer{Strategy: strategy, Source: source}
			status := deployer.Check(t.Src, t.Dest)
			switch status {
			case dotfiles.CheckPresent:
				dotOk++
				p.OK(fmt.Sprintf("%-30s  present", t.Dest))
			case dotfiles.CheckDrifted:
				dotMissing++
				p.Warn(fmt.Sprintf("%-30s  drifted", t.Dest))
			case dotfiles.CheckAbsent:
				dotMissing++
				p.Warn(fmt.Sprintf("%-30s  missing", t.Dest))
			case dotfiles.CheckSrcMissing:
				dotMissing++
				p.Warn(fmt.Sprintf("%-30s  src missing", t.Src))
			}
		}
	}
	total += dotTotal
	ok += dotOk
	missing += dotMissing

	// --- secrets ---
	p.Header("secrets")
	secTotal := len(cfg.Secrets.Mappings)
	if secTotal == 0 {
		p.Info("no secrets declared")
	} else {
		for _, m := range cfg.Secrets.Mappings {
			// We don't resolve secrets in list — just show the mapping exists.
			// Secrets aren't status-checked, so we don't add them to total/ok/missing
			// (the summary counts verified items only).
			dests := make([]string, 0, len(m.Inject))
			for d := range m.Inject {
				dests = append(dests, d)
			}
			p.Info(fmt.Sprintf("%-30s  → %d target(s)", m.Key, len(dests)))
		}
	}

	// --- summary ---
	p.Header("summary")
	p.Info(fmt.Sprintf("%d managed (%d ok, %d need attention)", total, ok, missing))

	return nil
}
