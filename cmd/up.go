package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/jfjrh2014/nestor/internal/secrets"
	"github.com/jfjrh2014/nestor/internal/snapshot"
	"github.com/jfjrh2014/nestor/internal/ui"
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
	p := ui.New(os.Stdout)

	path := configPath()
	p.Info(fmt.Sprintf("loading config from %s", path))

	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("up: %w", err)
	}

	p.OK(fmt.Sprintf("config v%d loaded", cfg.Version))
	p.Detail("dotfiles strategy", cfg.Dotfiles.Strategy)

	// Step 1: detect platform
	p.Header("platform")
	plat, err := platform.Detect()
	if err != nil {
		p.Warn(err.Error())
	} else {
		p.OK("platform detected")
	}
	p.Detail("os", plat.OS)
	p.Detail("arch", plat.Arch)
	p.Detail("package manager", plat.PackageManager)

	// Step 2: install packages
	p.Header("packages")
	resolver := packages.Resolver{
		Common: cfg.Packages.Common,
		Lists: map[string][]string{
			"macos": cfg.Packages.MacOS,
			"linux": cfg.Packages.Linux,
			"wsl":   cfg.Packages.WSL,
		},
	}
	specs := make([]packages.Spec, 0, len(resolver.Resolve(plat.OS)))
	for _, raw := range resolver.Resolve(plat.OS) {
		specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
	}

	if len(specs) == 0 {
		p.Info("no packages declared")
	} else {
		p.Info(fmt.Sprintf("installing %d packages", len(specs)))
		results := packages.InstallAll(specs, plat.PackageManager)
		installed, skipped, failed := 0, 0, 0
		for _, r := range results {
			switch r.Status {
			case packages.StatusInstalled:
				installed++
				p.OK(r.Spec.Name)
			case packages.StatusAlreadyInstalled:
				skipped++
			case packages.StatusError:
				failed++
				p.Error(fmt.Sprintf("%s: %v", r.Spec.Name, r.Err))
			}
		}
		p.Info(fmt.Sprintf("%d installed, %d already present, %d failed", installed, skipped, failed))
	}

	// Step 3: snapshot existing dotfiles before overwriting
	if len(cfg.Dotfiles.Templates) > 0 {
		destPaths := make([]string, 0, len(cfg.Dotfiles.Templates))
		for _, t := range cfg.Dotfiles.Templates {
			destPaths = append(destPaths, t.Dest)
		}
		snap, err := snapshot.Create(destPaths)
		if err != nil {
			p.Warn(fmt.Sprintf("snapshot failed (non-fatal): %v", err))
		} else if len(snap.Files) > 0 {
			p.OK(fmt.Sprintf("snapshot created (%d files backed up)", len(snap.Files)))
		}
	}

	// Step 4: deploy dotfiles
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
			source = filepath.Join(home, ".config", "nestor", "dotfiles")
		}

		temps := make([]dotfiles.Template, 0, len(cfg.Dotfiles.Templates))
		for _, t := range cfg.Dotfiles.Templates {
			temps = append(temps, dotfiles.Template{Src: t.Src, Dest: t.Dest})
		}

		p.Info(fmt.Sprintf("deploying %d templates (%s strategy)", len(temps), strategy))

		deployer := dotfiles.Deployer{Strategy: strategy, Source: source}
		results := deployer.DeployAll(temps)

		deployed, failed := 0, 0
		for _, r := range results {
			switch r.Status {
			case dotfiles.StatusDeployed:
				deployed++
				p.OK(r.Template.Dest)
			case dotfiles.StatusError:
				failed++
				p.Error(fmt.Sprintf("%s: %v", r.Template.Dest, r.Err))
			}
		}
		p.Info(fmt.Sprintf("%d deployed, %d failed", deployed, failed))
	}

	// Step 5: inject secrets
	p.Header("secrets")
	if len(cfg.Secrets.Mappings) == 0 || cfg.Secrets.Provider == "" {
		p.Info("no secrets declared")
	} else {
		prov, err := secrets.NewProvider(cfg.Secrets.Provider)
		if err != nil {
			p.Error(fmt.Sprintf("provider %s: %v", cfg.Secrets.Provider, err))
		} else {
			p.Info(fmt.Sprintf("resolving %d secrets via %s", len(cfg.Secrets.Mappings), prov.Name()))
			sMappings := make([]secrets.Mapping, 0, len(cfg.Secrets.Mappings))
			for _, m := range cfg.Secrets.Mappings {
				sMappings = append(sMappings, secrets.Mapping{Key: m.Key, Inject: m.Inject})
			}
			vals, err := secrets.ResolveAll(prov, sMappings)
			if err != nil {
				p.Error(fmt.Sprintf("resolve: %v", err))
			} else {
				results := secrets.InjectAll(vals, sMappings)
				injected, failed := 0, 0
				for _, r := range results {
					switch r.Status {
					case secrets.StatusInjected:
						injected++
						p.OK(r.Dest)
					case secrets.StatusError:
						failed++
						p.Error(fmt.Sprintf("%s: %v", r.Dest, r.Err))
					}
				}
				p.Info(fmt.Sprintf("%d injected, %d failed", injected, failed))
			}
		}
	}

	// TODO: configure shell from cfg.Shells

	p.Header("result")
	p.OK("environment is up")

	return nil
}
