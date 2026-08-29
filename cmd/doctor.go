package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	secpkg "github.com/jfjrh2014/nestor/internal/secrets"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Health check for your nestor environment",
	Long: `Checks that your nestor environment is healthy:
config valid, packages installed, dotfiles present,
secrets provider available. Like 'brew doctor' for everything.
Use --profile to check against a named profile's packages and
dotfiles (mirrors 'nestor up --profile', as 'nestor diff' does).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor(cmd.Context())
	},
}

var doctorProfileFlag string

func init() {
	doctorCmd.Flags().StringVarP(&doctorProfileFlag, "profile", "p", "", "check against a named profile (extra packages/dotfiles)")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(ctx context.Context) error {
	profileName, _ := ctx.Value(profileKey{}).(string)
	if profileName == "" {
		profileName = doctorProfileFlag
	}
	return runDoctorProfileOut(ctx, profileName, os.Stdout)
}

func runDoctorOut(ctx context.Context, w io.Writer) error {
	return runDoctorProfileOut(ctx, "", w)
}

func runDoctorProfileOut(ctx context.Context, profileName string, w io.Writer) error {
	p := ui.New(w)
	issues := 0

	// 1. Config
	p.Header("config")
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		p.Error(fmt.Sprintf("config error: %v", err))
		return fmt.Errorf("cannot proceed without valid config")
	}
	p.OK(fmt.Sprintf("valid config at %s", path))
	if profileName != "" {
		if !cfg.ValidProfile(profileName) {
			return fmt.Errorf("unknown profile: %s", profileName)
		}
		p.OK(fmt.Sprintf("profile: %s", profileName))
	}

	// 2. Platform
	p.Header("platform")
	plat, platErr := platform.Detect()
	if platErr != nil {
		p.Error(fmt.Sprintf("platform detection failed: %v", platErr))
		issues++
	} else {
		p.OK(fmt.Sprintf("os=%s arch=%s pkgmgr=%s", plat.OS, plat.Arch, plat.PackageManager))
		if _, lookErr := exec.LookPath(plat.PackageManager); lookErr != nil {
			p.Error(fmt.Sprintf("package manager '%s' not found in PATH", plat.PackageManager))
			issues++
		} else {
			p.OK(fmt.Sprintf("package manager '%s' available", plat.PackageManager))
		}
	}

	// 3. Packages
	p.Header("packages")
	resolver := packages.Resolver{
		Common: cfg.Packages.Common,
		Lists: map[string][]string{
			"macos": cfg.Packages.MacOS,
			"linux": cfg.Packages.Linux,
			"wsl":   cfg.Packages.WSL,
		},
	}
	if profileName != "" {
		resolver.Common = append(resolver.Common, cfg.ProfilePackages(profileName)...)
	}
	var specs []packages.Spec
	if platErr == nil {
		for _, raw := range resolver.Resolve(plat.OS) {
			specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
		}
	}

	if len(specs) == 0 {
		p.Info("no packages declared")
	} else {
		installed, missing := 0, 0
		for _, s := range specs {
			mgr, mgrErr := packages.NewManager(s.Manager)
			if mgrErr != nil {
				p.Error(fmt.Sprintf("%s: unknown package manager '%s'", s.Name, s.Manager))
				issues++
				missing++
				continue
			}
			ok, _ := mgr.IsInstalled(s)
			if ok {
				installed++
			} else {
				missing++
				issues++
				p.Warn(fmt.Sprintf("missing: %s", s.Name))
			}
		}
		if missing == 0 {
			p.OK(fmt.Sprintf("all %d packages installed", installed))
		} else {
			p.Info(fmt.Sprintf("%d installed, %d missing", installed, missing))
		}
	}

	// 4. Dotfiles
	templates := cfg.Dotfiles.Templates
	if profileName != "" {
		templates = append(templates, cfg.ProfileDotfiles(profileName)...)
	}
	p.Header("dotfiles")
	if len(templates) == 0 {
		p.Info("no dotfiles declared")
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

		if _, statErr := os.Stat(source); statErr != nil {
			p.Error(fmt.Sprintf("dotfiles source dir missing: %s", source))
			issues++
		} else {
			p.OK(fmt.Sprintf("source dir: %s", source))
		}

		present, drifted, absent := 0, 0, 0
		for _, t := range templates {
			deployer := dotfiles.Deployer{Strategy: strategy, Source: source}
			status := deployer.Check(t.Src, t.Dest)
			switch status {
			case dotfiles.CheckPresent:
				present++
			case dotfiles.CheckDrifted:
				drifted++
				issues++
				p.Warn(fmt.Sprintf("drifted: %s", t.Dest))
			case dotfiles.CheckAbsent:
				absent++
				issues++
				p.Warn(fmt.Sprintf("not deployed: %s", t.Dest))
			case dotfiles.CheckSrcMissing:
				absent++
				issues++
				p.Error(fmt.Sprintf("source missing: %s", t.Src))
			}
		}
		if drifted+absent == 0 {
			p.OK(fmt.Sprintf("all %d dotfiles present and up to date", present))
		} else {
			p.Info(fmt.Sprintf("%d present, %d drifted, %d missing", present, drifted, absent))
		}
	}

	// 5. Secrets
	p.Header("secrets")
	if len(cfg.Secrets.Mappings) == 0 {
		p.Info("no secrets declared")
	} else {
		// An empty provider is valid: NewProvider("") returns the env default.
		// Resolve it so we report on the real provider, not the config literal.
		provName := cfg.Secrets.Provider
		if prov, provErr := secpkg.NewProvider(cfg.Secrets.Provider); provErr == nil {
			provName = prov.Name()
		}
		cli := secretCLI(provName)
		if cli == "" {
			p.Error(fmt.Sprintf("unknown secrets provider: %s", provName))
			issues++
		} else if _, lookErr := exec.LookPath(cli); lookErr != nil {
			p.Error(fmt.Sprintf("secrets provider CLI '%s' not found in PATH", cli))
			p.Info(fmt.Sprintf("install %s to enable secret injection", cli))
			issues++
		} else {
			p.OK(fmt.Sprintf("provider '%s' (CLI: %s) available", provName, cli))
		}
		p.Info(fmt.Sprintf("%d secret mapping(s) configured", len(cfg.Secrets.Mappings)))
	}

	// Summary
	p.Header("diagnosis")
	if issues == 0 {
		p.OK("your nestor environment is healthy")
	} else {
		p.Warn(fmt.Sprintf("%d issue(s) found — run 'nestor up' to fix", issues))
	}

	return nil
}

// secretCLI maps a provider name to its required CLI binary.
func secretCLI(provider string) string {
	switch provider {
	case "env":
		return "env" // always available
	case "1password":
		return "op"
	case "bitwarden":
		return "bw"
	case "vault":
		return "vault"
	default:
		return ""
	}
}
