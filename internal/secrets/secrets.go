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
	out, err := exec.Command("op", "read", key).Output()
	if err != nil {
		return "", fmt.Errorf("op read %s: %w", key, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// BitwardenProvider reads secrets via the `bw` CLI.
type BitwardenProvider struct{}

func (BitwardenProvider) Name() string { return "bitwarden" }

func (BitwardenProvider) Resolve(key string) (string, error) {
	out, err := exec.Command("bw", "get", "password", key).Output()
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
	out, err := exec.Command("vault", "read", "-field="+field, path).Output()
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

	// Replace key placeholder in pattern.
	// Pattern like: "oauth_token: {{.Key}}" → "oauth_token: secretval"
	replaced := strings.ReplaceAll(pattern, "{{.Key}}", val)
	replaced = strings.ReplaceAll(replaced, "{{."+key+"}}", val)

	// If dest exists, try to find and replace the pattern in the existing file.
	data, err := os.ReadFile(dest)
	if err == nil && len(data) > 0 {
		content := string(data)
		if strings.Contains(content, pattern) {
			content = strings.ReplaceAll(content, pattern, replaced)
			if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
				return InjectResult{Key: key, Dest: dest, Status: StatusError, Err: fmt.Errorf("write: %w", err)}
			}
			return InjectResult{Key: key, Dest: dest, Status: StatusInjected}
		}
	}

	// Dest doesn't exist or pattern not found — append the line.
	line := replaced + "\n"
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
