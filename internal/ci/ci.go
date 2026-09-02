// Package ci validates a nestor.yml config in a static, side-effect-free way.
// It is safe to run in CI (no package installs, no file writes, no network).
package ci

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/secrets"
)

// Severity classifies a validation finding.
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	}
	return "unknown"
}

// Finding is a single validation result.
type Finding struct {
	Severity Severity
	Category string // config, packages, dotfiles, secrets, profiles
	Message  string
}

// Report is the full validation output.
type Report struct {
	Findings []Finding
	Passed   bool
}

// HasErrors returns true if any finding is SeverityError.
func (r Report) HasErrors() bool {
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

// ErrorCount returns the number of error-severity findings.
func (r Report) ErrorCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			n++
		}
	}
	return n
}

// WarnCount returns the number of warning-severity findings.
func (r Report) WarnCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityWarning {
			n++
		}
	}
	return n
}

// Validate runs all validation checks against the config and returns a Report.
// dotfilesSource is the resolved source directory to check template file existence;
// pass "" to skip template-source existence checks.
func Validate(cfg *config.Config, dotfilesSource string) Report {
	var findings []Finding

	findings = append(findings, validateConfig(cfg)...)
	findings = append(findings, validatePackages(cfg)...)
	findings = append(findings, validateDotfiles(cfg, dotfilesSource)...)
	findings = append(findings, validateSecrets(cfg)...)
	findings = append(findings, validateProfiles(cfg, dotfilesSource)...)

	return Report{
		Findings: findings,
		Passed:   len(findings) == 0 || !hasErrors(findings),
	}
}

func hasErrors(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityError {
			return true
		}
	}
	return false
}

func validateConfig(cfg *config.Config) []Finding {
	var out []Finding

	if cfg.Version == 0 {
		out = append(out, Finding{SeverityError, "config", "version is missing (must be 1)"})
	} else if cfg.Version != 1 {
		out = append(out, Finding{SeverityError, "config", fmt.Sprintf("unsupported version %d (only version 1 supported)", cfg.Version)})
	}

	switch cfg.Dotfiles.Strategy {
	case "", "copy", "symlink":
		// valid
	default:
		out = append(out, Finding{SeverityError, "config", fmt.Sprintf("dotfiles.strategy %q invalid (must be copy or symlink)", cfg.Dotfiles.Strategy)})
	}

	return out
}

// validPackageManagers lists the manager prefixes we recognise.
var validPackageManagers = map[string]bool{
	"brew":   true,
	"apt":    true,
	"dnf":    true,
	"pacman": true,
	"snap":   true,
}

func validatePackages(cfg *config.Config) []Finding {
	var out []Finding

	all := [][]string{
		cfg.Packages.Common,
		cfg.Packages.MacOS,
		cfg.Packages.Linux,
		cfg.Packages.WSL,
	}
	seen := map[string]bool{} // dedup per-category warnings
	for _, list := range all {
		for _, raw := range list {
			spec := parsePkgSpec(raw)
			if spec.manager != "" && !validPackageManagers[spec.manager] {
				key := spec.manager + ":" + raw
				if !seen[key] {
					out = append(out, Finding{
						SeverityWarning,
						"packages",
						fmt.Sprintf("unknown package manager %q in spec %q", spec.manager, raw),
					})
					seen[key] = true
				}
			}
		}
	}

	// check for profile packages too
	for name, prof := range cfg.Profiles {
		for _, raw := range prof.Packages {
			spec := parsePkgSpec(raw)
			if spec.manager != "" && !validPackageManagers[spec.manager] {
				key := name + ":" + spec.manager
				if !seen[key] {
					out = append(out, Finding{
						SeverityWarning,
						"packages",
						fmt.Sprintf("unknown package manager %q in profile %q spec %q", spec.manager, name, raw),
					})
					seen[key] = true
				}
			}
		}
	}

	return out
}

type parsedSpec struct {
	manager string
	sub     string
	name    string
}

func parsePkgSpec(raw string) parsedSpec {
	raw = trimSpace(raw)
	s := parsedSpec{name: raw}
	if idx := indexByte(raw, ':'); idx >= 0 {
		left := trimSpace(raw[:idx])
		s.name = trimSpace(raw[idx+1:])
		if slash := indexByte(left, '/'); slash >= 0 {
			s.manager = left[:slash]
			s.sub = left[slash+1:]
		} else {
			s.manager = left
		}
	}
	return s
}

func validateDotfiles(cfg *config.Config, source string) []Finding {
	// Base layer first: templateFindings covers empty fields, duplicate and
	// override dests, src existence, and render checks for .tmpl files.
	return templateFindings("dotfiles", cfg.Dotfiles.Templates, source, map[string]bool{})
}

func validateSecrets(cfg *config.Config) []Finding {
	var out []Finding

	if cfg.Secrets.Provider == "" {
		// no provider declared; if there are mappings, that's an error
		if len(cfg.Secrets.Mappings) > 0 {
			out = append(out, Finding{SeverityError, "secrets", fmt.Sprintf("%d secret mappings but no provider set", len(cfg.Secrets.Mappings))})
		}
		return out
	}

	// validate provider name
	if _, err := secrets.NewProvider(cfg.Secrets.Provider); err != nil {
		out = append(out, Finding{SeverityError, "secrets", fmt.Sprintf("invalid provider %q: %v", cfg.Secrets.Provider, err)})
		return out
	}

	// each mapping should have at least one inject target
	for i, m := range cfg.Secrets.Mappings {
		if m.Key == "" {
			out = append(out, Finding{SeverityError, "secrets", fmt.Sprintf("mapping #%d has empty key", i+1)})
		}
		if len(m.Inject) == 0 {
			out = append(out, Finding{SeverityWarning, "secrets", fmt.Sprintf("secret %q has no inject targets", m.Key)})
		}
	}

	return out
}

// templateFindings checks one template list: empty src/dest and duplicate
// dests, plus src existence when source is non-empty. prefix names the
// section in messages (e.g. "dotfiles", "profile work"). destSeen accumulates
// across calls so profile templates can be checked against base dests.
func templateFindings(prefix string, tmpl []config.Template, source string, destSeen map[string]bool) []Finding {
	var out []Finding
	layerSeen := map[string]bool{}
	for _, t := range tmpl {
		switch {
		case t.Dest == "":
			out = append(out, Finding{SeverityError, prefix, "template with empty dest path"})
			continue
		case t.Src == "":
			out = append(out, Finding{SeverityError, prefix, fmt.Sprintf("template dest %q has empty src", t.Dest)})
			continue
		}
		if layerSeen[t.Dest] {
			out = append(out, Finding{SeverityError, prefix, fmt.Sprintf("duplicate dest path %q", t.Dest)})
		} else if destSeen[t.Dest] {
			out = append(out, Finding{SeverityWarning, prefix, fmt.Sprintf("dest %q overrides a template from an earlier layer", t.Dest)})
		}
		layerSeen[t.Dest] = true
		destSeen[t.Dest] = true

		if source != "" {
			srcPath := t.Src
			if !filepath.IsAbs(srcPath) {
				srcPath = filepath.Join(source, srcPath)
			}
			if _, err := os.Stat(srcPath); err != nil {
				out = append(out, Finding{
					SeverityWarning,
					prefix,
					fmt.Sprintf("template src %q not found (%v)", t.Src, osErrText(err)),
				})
				continue
			}
			// Templates that will be rendered at deploy time are rendered
			// here too: an undefined key passes src-existence but fails
			// every future deploy, so report it now as an error.
			if strings.HasSuffix(srcPath, ".tmpl") {
				if _, err := dotfiles.Render(srcPath); err != nil {
					out = append(out, Finding{
						SeverityError,
						prefix,
						fmt.Sprintf("template src %q fails to render (%v)", t.Src, osErrText(err)),
					})
				}
			}
		}
	}
	return out
}

// mappingFindings checks one secret-mapping list: empty keys and inject-less
// mappings. prefix names the section in messages (e.g. "secrets", "profile work").
func mappingFindings(prefix string, mappings []config.Mapping) []Finding {
	var out []Finding
	for i, m := range mappings {
		if m.Key == "" {
			out = append(out, Finding{SeverityError, prefix, fmt.Sprintf("mapping #%d has empty key", i+1)})
			continue
		}
		if len(m.Inject) == 0 {
			out = append(out, Finding{SeverityWarning, prefix, fmt.Sprintf("secret %q has no inject targets", m.Key)})
		}
	}
	return out
}

func validateProfiles(cfg *config.Config, dotfilesSource string) []Finding {
	var out []Finding

	for name, prof := range cfg.Profiles {
		if name == "" {
			out = append(out, Finding{SeverityError, "profiles", "profile with empty name"})
		}
		if len(prof.Packages) == 0 && len(prof.Dotfiles) == 0 && len(prof.SecretMappings) == 0 {
			out = append(out, Finding{SeverityWarning, "profiles", fmt.Sprintf("profile %q declares no packages, dotfiles, or secrets", name)})
		}

		prefix := "profile " + name

		// Profile dotfiles get the same field/syntax checks as base templates.
		// destSeen starts from base dests: a profile dest shared with base is a
		// deliberate override (up deploys profile after base) and only warns;
		// two templates of the same layer colliding is an error, as in base.
		destSeen := map[string]bool{}
		for _, t := range cfg.Dotfiles.Templates {
			if t.Dest != "" {
				destSeen[t.Dest] = true
			}
		}
		out = append(out, templateFindings(prefix, prof.Dotfiles, dotfilesSource, destSeen)...)

		// Profile secrets get the same per-mapping checks as base; they also
		// need a provider, which the base branch only guards for base mappings.
		if cfg.Secrets.Provider == "" && len(prof.SecretMappings) > 0 {
			out = append(out, Finding{SeverityError, prefix, fmt.Sprintf("%d secret mappings but no provider set", len(prof.SecretMappings))})
		}
		out = append(out, mappingFindings(prefix, prof.SecretMappings)...)
	}

	return out
}

// Minimal string helpers to avoid importing strings (keep deps lean in tests).

func trimSpace(s string) string {
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s)
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func osErrText(err error) string {
	if err == nil {
		return ""
	}
	// Just the message without wrapping
	return err.Error()
}
