package cmd

import (
	"context"
	"fmt"
	"io"
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

var secretsProfileFlag string

var secretsInjectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Resolve and inject all configured secrets",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Context().Value(profileKey{}).(string)
		if profileName == "" {
			profileName = secretsProfileFlag
		}
		return runSecretsInjectProfileOut(cmd.Context(), profileName, os.Stdout)
	},
}

var secretsCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify all secrets are accessible (dry run, no writes)",
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName, _ := cmd.Context().Value(profileKey{}).(string)
		if profileName == "" {
			profileName = secretsProfileFlag
		}
		return runSecretsCheckProfileOut(cmd.Context(), profileName, os.Stdout)
	},
}

func init() {
	secretsInjectCmd.Flags().StringVarP(&secretsProfileFlag, "profile", "p", "", "include a named profile's extra secrets (mirrors 'nestor up --profile')")
	secretsCheckCmd.Flags().StringVarP(&secretsProfileFlag, "profile", "p", "", "include a named profile's extra secrets (mirrors 'nestor up --profile')")
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

// effectiveSecretMappings returns the secrets to act on: base mappings plus
// the named profile's extra layers (” = base only). It mirrors the profile
// resolution in 'up' (up.go Step 5) so inject/check see the same set that
// 'up --profile' would inject. A profile with no secret mappings means the
// base set unchanged; an unknown profile is a hard error.
func effectiveSecretMappings(cfg *config.Config, profileName string) (mappings, profileSecs []config.Mapping, err error) {
	mappings = cfg.Secrets.Mappings
	if profileName == "" {
		return mappings, nil, nil
	}
	if !cfg.ValidProfile(profileName) {
		return nil, nil, fmt.Errorf("unknown profile: %s", profileName)
	}
	profileSecs = cfg.ProfileSecretMappings(profileName)
	if len(profileSecs) == 0 {
		return mappings, nil, nil
	}
	merged := make([]config.Mapping, 0, len(mappings)+len(profileSecs))
	merged = append(merged, mappings...)
	merged = append(merged, profileSecs...)
	return merged, profileSecs, nil
}

func runSecretsInject(ctx context.Context, w io.Writer) error {
	return runSecretsInjectProfileOut(ctx, "", w)
}

func runSecretsInjectProfileOut(ctx context.Context, profileName string, w io.Writer) error {
	p := ui.New(w)

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("secrets inject: %w", err)
	}

	mappings, extraSecs, err := effectiveSecretMappings(cfg, profileName)
	if err != nil {
		return err
	}

	p.Header("secrets")
	if len(mappings) == 0 {
		p.Info("no secrets declared")
		return nil
	}
	if len(extraSecs) > 0 {
		p.OK(fmt.Sprintf("profile %s: %d extra secrets", profileName, len(extraSecs)))
	}

	// An empty provider is valid: NewProvider("") returns the env default.
	// Resolve the provider once so a malformed provider name is caught before
	// we enter the per-key loop or the CLI existence check.
	prov, err := secrets.NewProvider(cfg.Secrets.Provider)
	if err != nil {
		p.Error(fmt.Sprintf("provider %s: %v", cfg.Secrets.Provider, err))
		return err
	}

	providerCLI := secretCLI(prov.Name())
	if providerCLI != "env" {
		if _, lookErr := exec.LookPath(providerCLI); lookErr != nil {
			p.Error(fmt.Sprintf("provider CLI '%s' not found in PATH", providerCLI))
			return fmt.Errorf("provider CLI '%s' not found", providerCLI)
		}
	}

	secMappings := buildSecretMappings(mappings)
	p.Info(fmt.Sprintf("resolving %d secrets via %s", len(secMappings), prov.Name()))

	vals, err := secrets.ResolveAll(prov, secMappings)
	if err != nil {
		p.Error(fmt.Sprintf("resolve: %v", err))
		return err
	}

	results := secrets.InjectAll(vals, secMappings)
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

func runSecretsCheck(ctx context.Context, w io.Writer) error {
	return runSecretsCheckProfileOut(ctx, "", w)
}

func runSecretsCheckProfileOut(ctx context.Context, profileName string, w io.Writer) error {
	p := ui.New(w)
	issues := 0

	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("secrets check: %w", err)
	}

	mappings, extraSecs, err := effectiveSecretMappings(cfg, profileName)
	if err != nil {
		return err
	}

	p.Header("secrets")
	if len(mappings) == 0 {
		p.Info("no secrets declared")
		return nil
	}
	if len(extraSecs) > 0 {
		p.OK(fmt.Sprintf("profile %s: %d extra secrets", profileName, len(extraSecs)))
	}

	// Provider. An empty provider is valid: NewProvider("") returns the env
	// default. Resolve it once so we check the real CLI, and so the
	// per-key loop below uses the same provider instance.
	prov, provErr := secrets.NewProvider(cfg.Secrets.Provider)
	resolvedName := cfg.Secrets.Provider
	if provErr != nil {
		p.Error(fmt.Sprintf("provider %s: %v", cfg.Secrets.Provider, provErr))
		issues++
	} else {
		resolvedName = prov.Name()
	}
	p.Info(fmt.Sprintf("provider: %s", resolvedName))

	providerCLI := secretCLI(resolvedName)
	if providerCLI == "" {
		p.Error(fmt.Sprintf("unknown provider: %s", resolvedName))
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
	secMappings := buildSecretMappings(mappings)
	if provErr != nil {
		// Provider setup failed; surface the per-key outcome as failures so
		// the diagnosis line still reflects every secret we could not check.
		for _, m := range secMappings {
			p.Error(fmt.Sprintf("%s: %v", m.Key, provErr))
		}
		p.Warn(fmt.Sprintf("0 resolved, %d failed", len(secMappings)))
		issues += len(secMappings)
	} else {
		resolved, failedCount := 0, 0
		for _, m := range secMappings {
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
		if failedCount > 0 {
			p.Warn(fmt.Sprintf("%d resolved, %d failed", resolved, failedCount))
		} else {
			p.OK(fmt.Sprintf("%d secret(s) resolved", resolved))
		}
	}

	// Injection targets
	p.Header("inject targets")
	totalTargets := 0
	for _, m := range mappings {
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
