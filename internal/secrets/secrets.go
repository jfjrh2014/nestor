// Package secrets resolves and injects secrets from external providers
// (env, 1password, bitwarden, vault) into dotfiles on disk.
package secrets

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Provider resolves a secret key to its plaintext value.
type Provider interface {
	Resolve(key string) (string, error)
	Name() string
}

// cmdOutput abstracts running an external command and capturing its stdout so
// the CLI-backed providers are testable without shelling out to real secret
// managers. The production implementation is execCommand, powered by
// exec.Command; tests swap in a mock that records the commands it was asked
// to run and returns canned stdout.
type cmdOutput interface {
	Output(name string, args ...string) ([]byte, error)
}

// execCommand runs commands via the real os/exec package.
type execCommand struct{}

func (execCommand) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// cmdOut is the package-level command runner used by the CLI-backed
// providers. It defaults to execCommand; tests swap it via swapCmdOut
// and restore on cleanup.
var cmdOut cmdOutput = execCommand{}

// EnvProvider reads secrets from environment variables.
type EnvProvider struct{}

func (EnvProvider) Name() string { return "env" }

func (EnvProvider) Resolve(key string) (string, error) {
	v := os.Getenv(key)
	if v == "" {
		return "", fmt.Errorf("env: %s not set", key)
	}
	return v, nil
}

// OnePasswordProvider reads secrets via the `op` CLI.
type OnePasswordProvider struct{}

func (OnePasswordProvider) Name() string { return "1password" }

func (OnePasswordProvider) Resolve(key string) (string, error) {
	out, err := cmdOut.Output("op", "read", key)
	if err != nil {
		return "", fmt.Errorf("op read %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BitwardenProvider reads secrets via the `bw` CLI.
type BitwardenProvider struct{}

func (BitwardenProvider) Name() string { return "bitwarden" }

func (BitwardenProvider) Resolve(key string) (string, error) {
	out, err := cmdOut.Output("bw", "get", "password", key)
	if err != nil {
		return "", fmt.Errorf("bw get password %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// VaultProvider reads secrets via HashiCorp Vault's `vault read` CLI.
type VaultProvider struct{}

func (VaultProvider) Name() string { return "vault" }

func (VaultProvider) Resolve(key string) (string, error) {
	// key format: secret/path#field
	parts := strings.SplitN(key, "#", 2)
	path := key
	field := "value"
	if len(parts) == 2 {
		path = parts[0]
		field = parts[1]
	}
	out, err := cmdOut.Output("vault", "read", "-field="+field, path)
	if err != nil {
		return "", fmt.Errorf("vault read %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// NewProvider picks a provider by name.
func NewProvider(name string) (Provider, error) {
	switch name {
	case "env", "":
		return EnvProvider{}, nil
	case "1password":
		return OnePasswordProvider{}, nil
	case "bitwarden":
		return BitwardenProvider{}, nil
	case "vault":
		return VaultProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown secrets provider: %s", name)
	}
}

// Mapping mirrors the config Mapping struct.
type Mapping struct {
	Key     string
	Inject  map[string]string // dest path → template string with {{.key}}
}

// ResolveAll resolves every mapping key and returns a key→value map.
func ResolveAll(p Provider, mappings []Mapping) (map[string]string, error) {
	vals := make(map[string]string, len(mappings))
	for _, m := range mappings {
		v, err := p.Resolve(m.Key)
		if err != nil {
			return nil, fmt.Errorf("secret %s: %w", m.Key, err)
		}
		vals[m.Key] = v
	}
	return vals, nil
}

// InjectAll writes resolved secrets into their target files.
// For each mapping, each inject target's template string gets its {{.key}}
// placeholders replaced with the resolved value, then injected into the dest file.
func InjectAll(vals map[string]string, mappings []Mapping) []InjectResult {
	var results []InjectResult
	for _, m := range mappings {
		val, ok := vals[m.Key]
		if !ok {
			results = append(results, InjectResult{Key: m.Key, Status: StatusError, Err: fmt.Errorf("no resolved value")})
			continue
		}
		for dest, pattern := range m.Inject {
			r := injectOne(m.Key, val, dest, pattern)
			results = append(results, r)
		}
	}
	return results
}

// InjectResult reports the outcome for one injection target.
type InjectResult struct {
	Key    string
	Dest   string
	Status Status
	Err    error
}

// Status of a secret injection.
type Status int

const (
	StatusInjected Status = iota
	StatusError
)

func (s Status) String() string {
	switch s {
	case StatusInjected:
		return "injected"
	case StatusError:
		return "error"
	}
	return "unknown"
}

func injectOne(key, val, dest, pattern string) InjectResult {
	dest = expandHome(dest)

	// Create dest dir if needed.
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("mkdir: %w", err)}
	}

	// Resolve the placeholder(s) to produce the new line. We support both the
	// literal {{.Key}} form and the named {{.<key>}} form so a single pattern
	// works regardless of how the user wrote the placeholder.
	resolved := strings.ReplaceAll(pattern, "{{.Key}}", val)
	resolved = strings.ReplaceAll(resolved, "{{."+key+"}}", val)

	data, err := os.ReadFile(dest)
	if err == nil && len(data) > 0 {
		content := string(data)

		// Case 1: the literal pattern is still in the file (first injection
		// against a template that still carries its placeholder).
		if strings.Contains(content, pattern) {
			content = strings.ReplaceAll(content, pattern, resolved)
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
			}
			return InjectResult{Key: key, Dest: dest, Status: StatusInjected}
		}

		// Case 2: the pattern's static anchor prefix is present on a line — that
		// line is a previous injection (the placeholder was already resolved on
		// an earlier run). Replace the whole line so re-injection is idempotent
		// and value rotation replaces instead of appending a duplicate.
		if anchor := patternAnchor(pattern); anchor != "" {
			if updated, ok := replaceAnchoredLine(content, anchor, resolved); ok {
				if err := os.WriteFile(dest, []byte(updated), 0o644); err != nil {
					return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
				}
				return InjectResult{Key: key, Dest: dest, Status: StatusInjected}
			}
		}
	}

	// Case 3: dest is absent, empty, or has no matching anchor — append the line.
	line := resolved + "\n"
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("open: %w", err)}
	}
	defer f.Close()
	if _, err := f.WriteString(line); err != nil {
		return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
	}
	return InjectResult{Key: key, Dest: dest, Status: StatusInjected}
}

// patternAnchor returns the static prefix of pattern up to the first {{
// placeholder. If pattern has no placeholder, patternAnchor returns the whole
// pattern. The anchor identifies which line in an already-injected file belongs
// to this mapping, so a second injection replaces the stale value in place
// instead of appending a duplicate.
func patternAnchor(pattern string) string {
	if idx := strings.Index(pattern, "{{"); idx >= 0 {
		return pattern[:idx]
	}
	return pattern
}

// replaceAnchoredLine replaces the first line in content that begins with
// anchor with newLine. It returns the updated content and whether a matching
// line was found. An empty anchor never matches.
func replaceAnchoredLine(content, anchor, newLine string) (string, bool) {
	if anchor == "" {
		return content, false
	}
	lines := strings.Split(content, "\n")
	for i, ln := range lines {
		if strings.HasPrefix(ln, anchor) {
			lines[i] = newLine
			return strings.Join(lines, "\n"), true
		}
	}
	return content, false
}

func expandHome(p string) string {
	if p == "" {
		return p
	}
	if p[0] == '~' {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, p[1:])
		}
	}
	return p
}
