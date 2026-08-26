package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var diffProfileFlag string

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show drift between live machine state and nestor.yml",
	Long: `Compares your current machine against nestor.yml:
which packages are missing, extra, or which dotfiles have drifted
from their template source.
Use --profile to include a named profile's packages and dotfiles
in the comparison (mirrors 'nestor up --profile').`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDiff(cmd.Context())
	},
}

func init() {
	diffCmd.Flags().StringVarP(&diffProfileFlag, "profile", "p", "", "compare against a named profile (extra packages/dotfiles)")
	rootCmd.AddCommand(diffCmd)
}

func runDiff(ctx context.Context) error {
	profileName, _ := ctx.Value(profileKey{}).(string)
	if profileName == "" {
		profileName = diffProfileFlag
	}
	return runDiffOut(ctx, profileName, os.Stdout)
}

// runDiffOut is the writer-injected core of 'nestor diff' (pattern from
// sessions #18-#43). It compares live machine state against the config,
// optionally layered with a named profile exactly as 'nestor up' does —
// otherwise packages installed by 'up --profile X' would be reported as
// untracked drift by 'diff'.
func runDiffOut(ctx context.Context, profileName string, w io.Writer) error {
	p := ui.New(w)

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("diff: %w", err)
	}
	p.OK(fmt.Sprintf("config loaded from %s", path))

	if profileName != "" {
		if !cfg.ValidProfile(profileName) {
			return fmt.Errorf("unknown profile: %s", profileName)
		}
		p.OK(fmt.Sprintf("profile: %s", profileName))
	}

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
	resolved := resolver.Resolve(plat.OS)
	if profileName != "" {
		resolved = append(resolved, cfg.ProfilePackages(profileName)...)
	}

	specs := make([]packages.Spec, 0, len(resolved))
	for _, raw := range resolved {
		specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
	}

	// Names tracked in config (+ active profile), for untracked-package detection.
	configured := make(map[string]bool, len(specs))
	for _, s := range specs {
		configured[s.Name] = true
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

	// Installed dev packages not declared in config (or profile) = drift ("extra").
	untracked := untrackedPackages(configured, scanPackages(plat.PackageManager))
	for _, name := range untracked {
		extra++
		driftCount++
		p.Info(fmt.Sprintf("extra (not tracked): %s", name))
	}

	if missing == 0 && extra == 0 && len(specs) > 0 {
		p.OK(fmt.Sprintf("all %d packages installed", len(specs)))
	} else if len(specs) == 0 && extra == 0 {
		p.Info("no packages declared")
	}
	if extra > 0 {
		p.Info(fmt.Sprintf("%d extra package(s) not tracked — run 'nestor sync' to capture", extra))
	}

	// --- dotfiles ---
	p.Header("dotfiles")
	templates := cfg.Dotfiles.Templates
	if profileName != "" {
		templates = append(templates, cfg.ProfileDotfiles(profileName)...)
	}
	if len(templates) == 0 {
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
		for _, t := range templates {
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

// untrackedPackages returns installed package names not present in configured,
// sorted for deterministic output. It is a pure function of its inputs so the
// drift unit test can exercise it without invoking any real package manager.
func untrackedPackages(configured map[string]bool, installed []string) []string {
	seen := make(map[string]bool, len(installed))
	out := make([]string, 0, len(installed))
	for _, name := range installed {
		if configured[name] || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	// Deterministic output order.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1] > out[j]; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
