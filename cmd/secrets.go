package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/secrets"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secret injection",
	Long: `Resolve and inject secrets from external providers
(1Password, Bitwarden, HashiCorp Vault, env vars) into your dotfiles.

Use 'nestor secrets check' for a dry run that verifies all secrets are reachable.`,
}

var secretsInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Resolve and inject all configured secrets",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSecretsInject(cmd.Context())
	},
}

var secretsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify all secrets are accessible (dry run, no writes)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runSecretsCheck(cmd.Context())
	},
}

func init() {
	secretsCmd.AddCommand(secretsInjectCmd)
	secretsCmd.AddCommand(secretsCheckCmd)
	rootCmd.AddCommand(secretsCmd)
}

// buildSecretMappings converts config mappings to secrets package mappings.
func buildSecretMappings(cfgMappings []config.Mapping) []secrets.Mapping {
	out := make([]secrets.Mapping, 0, len(cfgMappings))
	for _, m := range cfgMappings {
		out = append(out, secrets.Mapping{Key: m.Key, Inject: m.Inject})
	}
	return out
}

func runSecretsInject(ctx context.Context) error {
	p := ui.New(os.Stdout)

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("secrets inject: %w", err)
	}

	p.Header("secrets")
	if cfg.Secrets.Provider == "" || len(cfg.Secrets.Mappings) == 0 {
		p.Info("no secrets declared")
		return nil
	}

	prov, err := secrets.NewProvider(cfg.Secrets.Provider)
	if err != nil {
		p.Error(fmt.Sprintf("provider %s: %v", cfg.Secrets.Provider, err))
		return err
	}

	providerCLI := secretCLI(cfg.Secrets.Provider)
	if providerCLI != "env" {
		if _, lookErr := exec.LookPath(providerCLI); lookErr != nil {
			p.Error(fmt.Sprintf("provider CLI '%s' not found in PATH", providerCLI))
			return fmt.Errorf("provider CLI '%s' not found", providerCLI)
		}
	}

	mappings := buildSecretMappings(cfg.Secrets.Mappings)
	p.Info(fmt.Sprintf("resolving %d secrets via %s", len(mappings), prov.Name()))

	vals, err := secrets.ResolveAll(prov, mappings)
	if err != nil {
		p.Error(fmt.Sprintf("resolve: %v", err))
		return err
	}

	results := secrets.InjectAll(vals, mappings)
	injected, failedCount := 0, 0
	for _, r := range results {
		switch r.Status {
		case secrets.StatusInjected:
			injected++
			p.OK(r.Dest)
		case secrets.StatusError:
			failedCount++
			p.Error(fmt.Sprintf("%s: %v", r.Dest, r.Err))
		}
	}

	p.Header("result")
	if failedCount > 0 {
		p.Warn(fmt.Sprintf("%d injected, %d failed", injected, failedCount))
	} else {
		p.OK(fmt.Sprintf("%d secrets injected", injected))
	}
	return nil
}

func runSecretsCheck(ctx context.Context) error {
	p := ui.New(os.Stdout)
	issues := 0

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("secrets check: %w", err)
	}

	p.Header("secrets")
	if cfg.Secrets.Provider == "" || len(cfg.Secrets.Mappings) == 0 {
		p.Info("no secrets declared")
		return nil
	}

	// Provider
	p.Info(fmt.Sprintf("provider: %s", cfg.Secrets.Provider))
	providerCLI := secretCLI(cfg.Secrets.Provider)
	if providerCLI == "" {
		p.Error(fmt.Sprintf("unknown provider: %s", cfg.Secrets.Provider))
		issues++
	} else if providerCLI != "env" {
		if _, lookErr := exec.LookPath(providerCLI); lookErr != nil {
			p.Error(fmt.Sprintf("CLI '%s' not found in PATH", providerCLI))
			issues++
		} else {
			p.OK(fmt.Sprintf("CLI '%s' available", providerCLI))
		}
	} else {
		p.OK("env provider (always available)")
	}

	// Resolve each key (dry run — no files written)
	mappings := buildSecretMappings(cfg.Secrets.Mappings)
	prov, err := secrets.NewProvider(cfg.Secrets.Provider)
	if err != nil {
		p.Error(fmt.Sprintf("init provider: %v", err))
		issues++
	} else {
		resolved, failedCount := 0, 0
		for _, m := range mappings {
			v, err := prov.Resolve(m.Key)
			if err != nil {
				failedCount++
				issues++
				p.Error(fmt.Sprintf("%s: %v", m.Key, err))
				continue
			}
			if v == "" {
				failedCount++
				issues++
				p.Warn(fmt.Sprintf("%s: resolved to empty value", m.Key))
			} else {
				resolved++
				p.OK(fmt.Sprintf("%s reachable", m.Key))
			}
		}
	}

	// Injection targets
	p.Header("inject targets")
	totalTargets := 0
	for _, m := range cfg.Secrets.Mappings {
		for dest := range m.Inject {
			totalTargets++
			if _, statErr := os.Stat(dest); statErr == nil {
				p.OK(dest)
			} else {
				p.Info(fmt.Sprintf("%s (will be created)", dest))
			}
		}
	}
	if totalTargets == 0 {
		p.Info("no injection targets declared")
	}

	// Summary
	p.Header("diagnosis")
	if issues == 0 {
		p.OK("all secrets reachable, ready to inject")
	} else {
		p.Warn(fmt.Sprintf("%d issue(s) found", issues))
	}
	return nil
}
