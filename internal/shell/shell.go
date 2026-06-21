package shell

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// markerLines wrap the managed block so re-runs are idempotent.
	markerBegin = "# >>> nestor managed shell plugins >>>"
	markerEnd   = "# <<< nestor managed shell plugins <<<"

	pluginsDir = "nestor" // subdir under ~/.config
)

// Detect returns the current login shell binary name (e.g. "zsh", "bash").
// Falls back to $SHELL if getent/chsh queries are unavailable.
func Detect() (string, error) {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "", fmt.Errorf("SHELL environment variable not set")
	}
	return filepath.Base(shell), nil
}

// RCFile returns the path to the shell's rc file for the given shell name.
// Returns empty string for unknown shells.
func RCFile(shellName string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch shellName {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		return filepath.Join(home, ".bashrc")
	default:
		return ""
	}
}

// Plugin is a parsed shell plugin entry.
type Plugin struct {
	Raw   string // original config string
	Type  PluginType
	Owner string // github owner (for github type)
	Repo  string // github repo (for github type)
}

type PluginType int

const (
	PluginGitHub PluginType = iota
	PluginNamed // standalone tool like "starship" — no action needed
)

// ParsePlugin classifies a plugin string from the config.
// Formats:
//   "starship"                      → named (standalone tool)
//   "zsh-users/zsh-autosuggestions" → github
func ParsePlugin(raw string) Plugin {
	raw = strings.TrimSpace(raw)
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
		return Plugin{
			Raw:   raw,
			Type:  PluginGitHub,
			Owner: parts[0],
			Repo:  parts[1],
		}
	}
	return Plugin{Raw: raw, Type: PluginNamed}
}

// PluginResult captures the outcome of processing a single plugin.
type PluginResult struct {
	Plugin Plugin
	Status PluginStatus
	Path   string // cloned local path (for github type, installed status)
	Err    error
}

type PluginStatus int

const (
	StatusInstalled    PluginStatus = iota // cloned / updated successfully
	StatusSkipped                          // named plugin — nothing to do
	StatusError                            // clone failed
)

// PluginsPath returns the directory where nestor clones shell plugins.
func PluginsPath() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, pluginsDir, "plugins"), nil
}

// InstallPlugins clones or updates all GitHub-type plugins and returns per-plugin
// results. Named plugins (standalone tools) are skipped — they're expected as
// system packages.
func InstallPlugins(rawPlugins []string) []PluginResult {
	pluginsDir, err := PluginsPath()
	if err != nil {
		// single error result
		results := make([]PluginResult, len(rawPlugins))
		for i, r := range rawPlugins {
			results[i] = PluginResult{Plugin: ParsePlugin(r), Status: StatusError, Err: fmt.Errorf("plugins dir: %w", err)}
		}
		return results
	}

	_ = os.MkdirAll(pluginsDir, 0755)

	results := make([]PluginResult, 0, len(rawPlugins))
	for _, raw := range rawPlugins {
		p := ParsePlugin(raw)

		if p.Type != PluginGitHub {
			results = append(results, PluginResult{Plugin: p, Status: StatusSkipped})
			continue
		}

		localPath := filepath.Join(pluginsDir, p.Repo)
		err := cloneOrUpdate(p.Owner, p.Repo, localPath)
		if err != nil {
			results = append(results, PluginResult{Plugin: p, Status: StatusError, Err: err})
			continue
		}
		results = append(results, PluginResult{Plugin: p, Status: StatusInstalled, Path: localPath})
	}
	return results
}

func cloneOrUpdate(owner, repo, localPath string) error {
	// Already cloned? Try to pull.
	if _, err := os.Stat(filepath.Join(localPath, ".git")); err == nil {
		cmd := exec.Command("git", "-C", localPath, "pull", "--ff-only")
		return cmd.Run()
	}

	url := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
	cmd := exec.Command("git", "clone", "--depth", "1", url, localPath)
	return cmd.Run()
}

// SourceLines returns the shell source lines for a set of installed GitHub plugins,
// wrapped in nestor markers. Lines are ordered deterministically.
func SourceLines(results []PluginResult) []string {
	var lines []string
	seen := map[string]bool{}

	for _, r := range results {
		if r.Status != StatusInstalled || r.Plugin.Type != PluginGitHub {
			continue
		}
		// source the plugin's main file: <path>/<repo>.plugin.zsh is the zsh convention.
		// Fall back to sourcing the dir.
		entry := fmt.Sprintf("%s.zsh", r.Plugin.Repo)
		sourcePath := filepath.Join(r.Path, entry)
		line := fmt.Sprintf("source %s", sourcePath)
		if seen[line] {
			continue
		}
		seen[line] = true
		lines = append(lines, line)
	}
	return lines
}

// WriteSourceBlock reads the rc file, inserts or replaces the nestor-managed
// block with the given source lines. Idempotent: re-runs update the block in place.
func WriteSourceBlock(rcPath string, sourceLines []string) error {
	existing, _ := os.ReadFile(rcPath)
	content := string(existing)

	newBlock := markerBegin + "\n"
	if len(sourceLines) > 0 {
		newBlock += strings.Join(sourceLines, "\n") + "\n"
	}
	newBlock += markerEnd + "\n"

	// Check if an existing block is present.
	beginIdx := strings.Index(content, markerBegin)
	endIdx := strings.Index(content, markerEnd)

	if beginIdx >= 0 && endIdx >= 0 && endIdx > beginIdx {
		// Replace existing block
		replaceEnd := endIdx + len(markerEnd)
		// Consume trailing newline after end marker
		if replaceEnd < len(content) && content[replaceEnd] == '\n' {
			replaceEnd++
		}
		content = content[:beginIdx] + newBlock + content[replaceEnd:]
	} else {
		// Append new block
		if content != "" && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += newBlock
	}

	if err := os.MkdirAll(filepath.Dir(rcPath), 0755); err != nil {
		return fmt.Errorf("creating rc dir: %w", err)
	}
	return os.WriteFile(rcPath, []byte(content), 0644)
}
