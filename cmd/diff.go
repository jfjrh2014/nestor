package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show drift between live machine state and nestor.yml",
	Long: `Compares your current machine against nestor.yml:
which packages are missing, extra, or which dotfiles have drifted
from their template source.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff(cmd.Context())
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func runDiff(ctx context.Context) error {
	p := ui.New(os.Stdout)

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	p.OK(fmt.Sprintf("config loaded from %s", path))

	plat, err := platform.Detect()
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}

	driftCount := 0

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
	specs := make([]packages.Spec, 0)
	for _, raw := range resolver.Resolve(plat.OS) {
		specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
	}

	missing, extra := 0, 0
	for _, s := range specs {
		mgr, err := packages.NewManager(s.Manager)
		if err != nil {
			p.Warn(fmt.Sprintf("%s: unknown manager %s", s.Name, s.Manager))
			driftCount++
			continue
		}
		ok, _ := mgr.IsInstalled(s)
		if !ok {
			missing++
			driftCount++
			p.Warn(fmt.Sprintf("missing: %s", s.Name))
		}
	}

	if missing == 0 && len(specs) > 0 {
		p.OK(fmt.Sprintf("all %d packages installed", len(specs)))
	} else if len(specs) == 0 {
		p.Info("no packages declared")
	}
	if extra > 0 {
		p.Info(fmt.Sprintf("%d extra packages not tracked", extra))
	}

	// --- dotfiles ---
	p.Header("dotfiles")
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
			source = fmt.Sprintf("%s/.config/nestor/dotfiles", home)
		}

		present, drifted, absent := 0, 0, 0
		for _, t := range cfg.Dotfiles.Templates {
			deployer := dotfiles.Deployer{Strategy: strategy, Source: source}
			status := deployer.Check(t.Src, t.Dest)
			switch status {
			case dotfiles.CheckPresent:
				present++
				p.OK(t.Dest)
			case dotfiles.CheckDrifted:
				drifted++
				driftCount++
				p.Warn(fmt.Sprintf("drifted: %s", t.Dest))
			case dotfiles.CheckAbsent:
				absent++
				driftCount++
				p.Warn(fmt.Sprintf("missing: %s", t.Dest))
			case dotfiles.CheckSrcMissing:
				absent++
				driftCount++
				p.Warn(fmt.Sprintf("src missing: %s", t.Src))
			}
		}
		p.Info(fmt.Sprintf("%d present, %d drifted, %d missing", present, drifted, absent))
	}

	// --- summary ---
	p.Header("summary")
	if driftCount == 0 {
		p.OK("no drift detected — environment matches config")
	} else {
		p.Warn(fmt.Sprintf("%d drift(s) found — run 'nestor up' to fix", driftCount))
	}

	return nil
}
