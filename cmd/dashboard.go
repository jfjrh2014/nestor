package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/dotfiles"
	"github.com/jfjrh2014/nestor/internal/packages"
	"github.com/jfjrh2014/nestor/internal/platform"
	"github.com/spf13/cobra"
)

// --- styles ---

var (
	dashTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 2)

	dashSubtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262"))

	dashGreenStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575"))

	dashRedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF4646"))

	dashYellowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500"))

	dashBoldStyle = lipgloss.NewStyle().Bold(true)

	dashSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#7D56F4"))

	dashCategoryStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#3F88C5")).
				MarginTop(1)
)

// --- model ---

type dashTab int

const (
	dashTabOverview dashTab = iota
	dashTabPackages
	dashTabDotfiles
	dashTabSecrets
	dashTabCount
)

var dashTabNames = []string{"Overview", "Packages", "Dotfiles", "Secrets"}

type dashboardModel struct {
	cfg       *config.Config
	cursor    int
	activeTab dashTab
	width     int
	height    int

	// collected status data
	platformInfo   platform.Info
	pkgSpecs       []packages.Spec
	installedCount int
	missingPkgs    []packages.Spec

	dotfileStatuses []dashDotfileStatus
	secretStatuses  []dashSecretStatus

	loaded bool
	err    error
}

type dashDotfileStatus struct {
	tmpl    config.Template
	present bool
	drift   bool
}

type dashSecretStatus struct {
	mapping config.Mapping
}

// --- messages ---

type dashStatusLoadedMsg struct {
	platformInfo   platform.Info
	pkgSpecs       []packages.Spec
	installedCount int
	missingPkgs    []packages.Spec
	dotfiles       []dashDotfileStatus
	secrets        []dashSecretStatus
}

type dashErrMsg struct{ err error }

func (e dashErrMsg) Error() string { return e.err.Error() }

// --- commands ---

func dashLoadStatus(cfg *config.Config) tea.Cmd {
	return func() tea.Msg {
		plat, err := platform.Detect()
		if err != nil {
			return dashErrMsg{err}
		}

		resolver := packages.Resolver{
			Common: cfg.Packages.Common,
			Lists: map[string][]string{
				"macos": cfg.Packages.MacOS,
				"linux": cfg.Packages.Linux,
				"wsl":   cfg.Packages.WSL,
			},
		}
		resolved := resolver.Resolve(plat.OS)

		specs := make([]packages.Spec, 0, len(resolved))
		for _, raw := range resolved {
			specs = append(specs, packages.ParseSpec(raw, plat.PackageManager))
		}

		mgr, _ := packages.NewManager(plat.PackageManager)

		installedCount := 0
		var missing []packages.Spec
		for _, s := range specs {
			if mgr != nil {
				if ok, _ := mgr.IsInstalled(s); ok {
					installedCount++
					continue
				}
			}
			missing = append(missing, s)
		}

		home, _ := os.UserHomeDir()
		sourceDir := cfg.Dotfiles.Source
		if sourceDir == "" {
			sourceDir = filepath.Join(home, ".config", "nestor", "dotfiles")
		}

		// Reuse the canonical drift detector from internal/dotfiles instead of
		// reimplementing it here. The previous inline check used os.Stat (which
		// follows symlinks, so a drifted link read as "missing") and a suffix
		// match on the link target (which matched any link ending in the file
		// name, not just links into the configured source dir). Check() uses
		// os.Lstat and compares the resolved source path.
		deployer := dotfiles.Deployer{
			Strategy: dotfiles.Strategy(cfg.Dotfiles.Strategy),
			Source:   sourceDir,
		}

		var dotStatuses []dashDotfileStatus
		for _, t := range cfg.Dotfiles.Templates {
			present := false
			drift := false
			switch deployer.Check(t.Src, t.Dest) {
			case dotfiles.CheckPresent:
				present = true
			case dotfiles.CheckDrifted:
				present = true
				drift = true
			case dotfiles.CheckAbsent, dotfiles.CheckSrcMissing:
				// present stays false
			}

			dotStatuses = append(dotStatuses, dashDotfileStatus{
				tmpl:    t,
				present: present,
				drift:   drift,
			})
		}

		var secStatuses []dashSecretStatus
		for _, m := range cfg.Secrets.Mappings {
			secStatuses = append(secStatuses, dashSecretStatus{mapping: m})
		}

		return dashStatusLoadedMsg{
			platformInfo:   plat,
			pkgSpecs:       specs,
			installedCount: installedCount,
			missingPkgs:    missing,
			dotfiles:       dotStatuses,
			secrets:        secStatuses,
		}
	}
}

// --- bubbletea boilerplate ---

func (m dashboardModel) Init() tea.Cmd {
	return dashLoadStatus(m.cfg)
}

func (m dashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "tab":
			m.activeTab = (m.activeTab + 1) % dashTabCount
			m.cursor = 0
		case "shift+tab":
			if m.activeTab == 0 {
				m.activeTab = dashTabCount - 1
			} else {
				m.activeTab--
			}
			m.cursor = 0
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			maxCursor := m.listLen() - 1
			if m.cursor < maxCursor {
				m.cursor++
			}
		case "1":
			m.activeTab = dashTabOverview
			m.cursor = 0
		case "2":
			m.activeTab = dashTabPackages
			m.cursor = 0
		case "3":
			m.activeTab = dashTabDotfiles
			m.cursor = 0
		case "4":
			m.activeTab = dashTabSecrets
			m.cursor = 0
		}
	case dashStatusLoadedMsg:
		m.platformInfo = msg.platformInfo
		m.pkgSpecs = msg.pkgSpecs
		m.installedCount = msg.installedCount
		m.missingPkgs = msg.missingPkgs
		m.dotfileStatuses = msg.dotfiles
		m.secretStatuses = msg.secrets
		m.loaded = true
	case dashErrMsg:
		m.err = msg.err
	}
	return m, nil
}

func (m dashboardModel) listLen() int {
	switch m.activeTab {
	case dashTabPackages:
		return len(m.pkgSpecs)
	case dashTabDotfiles:
		return len(m.dotfileStatuses)
	case dashTabSecrets:
		return len(m.secretStatuses)
	default:
		return 0
	}
}

func (m dashboardModel) View() string {
	if m.err != nil {
		return dashRedStyle.Render("✗ "+m.err.Error()) + "\n\nPress q to quit."
	}

	var b strings.Builder

	// title
	b.WriteString(dashTitleStyle.Render(" nestor "))
	b.WriteString("  ")
	b.WriteString(dashSubtleStyle.Render("Your dev environment, from zero to coding."))
	b.WriteString("\n")

	// platform line
	b.WriteString(dashSubtleStyle.Render(fmt.Sprintf("  %s · %s · %s",
		m.platformInfo.OS, m.platformInfo.Arch, m.platformInfo.PackageManager)))
	b.WriteString("\n")

	// tabs
	b.WriteString(m.renderTabs())
	b.WriteString("\n")

	switch m.activeTab {
	case dashTabOverview:
		b.WriteString(m.renderOverview())
	case dashTabPackages:
		b.WriteString(m.renderPackages())
	case dashTabDotfiles:
		b.WriteString(m.renderDotfiles())
	case dashTabSecrets:
		b.WriteString(m.renderSecrets())
	}

	// footer
	b.WriteString("\n")
	b.WriteString(dashSubtleStyle.Render("  ↑/↓ navigate · tab/shift+tab switch panels · 1-4 jump · q quit"))
	b.WriteString("\n")

	return b.String()
}

func (m dashboardModel) renderTabs() string {
	var tabs []string
	for i, name := range dashTabNames {
		if i == int(m.activeTab) {
			tabs = append(tabs, dashSelectedStyle.Render("▶ "+name))
		} else {
			tabs = append(tabs, dashSubtleStyle.Render("  "+name))
		}
	}
	return strings.Join(tabs, "  ")
}

func (m dashboardModel) renderOverview() string {
	var b strings.Builder

	totalPkgs := len(m.pkgSpecs)
	b.WriteString(dashCategoryStyle.Render("System Health"))
	b.WriteString("\n")

	b.WriteString(fmt.Sprintf("  Packages:  %s/%s installed",
		dashGreenStyle.Render(fmt.Sprintf("%d", m.installedCount)),
		dashBoldStyle.Render(fmt.Sprintf("%d", totalPkgs))))

	if len(m.missingPkgs) > 0 {
		b.WriteString("  " + dashRedStyle.Render(fmt.Sprintf("(%d missing)", len(m.missingPkgs))))
	}
	b.WriteString("\n")

	dotOK := 0
	dotDrift := 0
	dotMissing := 0
	for _, d := range m.dotfileStatuses {
		if !d.present {
			dotMissing++
		} else if d.drift {
			dotDrift++
		} else {
			dotOK++
		}
	}
	b.WriteString(fmt.Sprintf("  Dotfiles:  %s ok, %s drifted, %s missing\n",
		dashGreenStyle.Render(fmt.Sprintf("%d", dotOK)),
		dashYellowStyle.Render(fmt.Sprintf("%d", dotDrift)),
		dashRedStyle.Render(fmt.Sprintf("%d", dotMissing))))

	b.WriteString(fmt.Sprintf("  Secrets:   %d configured (%s)\n",
		len(m.secretStatuses),
		dashSubtleStyle.Render(dashSecretsProvider(m.cfg))))

	if len(dashProfiles(m.cfg)) > 0 {
		b.WriteString(fmt.Sprintf("  Profiles:  %s\n",
			dashSubtleStyle.Render(strings.Join(dashProfiles(m.cfg), ", "))))
	}

	return b.String()
}

// dashSecretsProvider returns the provider name for display. An empty provider
// is valid and defaults to env (sessions #34-#36 fixed this guard in other
// commands; this was the last display-side copy).
func dashSecretsProvider(cfg *config.Config) string {
	if len(cfg.Secrets.Mappings) == 0 {
		return "none"
	}
	if cfg.Secrets.Provider == "" {
		return "env"
	}
	return cfg.Secrets.Provider
}

func dashProfiles(cfg *config.Config) []string {
	var names []string
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m dashboardModel) renderPackages() string {
	if len(m.pkgSpecs) == 0 {
		return dashSubtleStyle.Render("  No packages configured.") + "\n"
	}

	missingSet := make(map[string]bool)
	for _, p := range m.missingPkgs {
		missingSet[p.Raw] = true
	}

	var b strings.Builder
	b.WriteString(dashCategoryStyle.Render(fmt.Sprintf("Packages (%d)", len(m.pkgSpecs))))
	b.WriteString("\n")

	for i, spec := range m.pkgSpecs {
		cursor := "  "
		style := dashSubtleStyle
		status := dashGreenStyle.Render("✓")
		if missingSet[spec.Raw] {
			status = dashRedStyle.Render("✗")
			style = dashBoldStyle
		}
		if i == m.cursor {
			cursor = dashSelectedStyle.Render("→ ")
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n", cursor, status, style.Render(spec.Name)))
	}
	return b.String()
}

func (m dashboardModel) renderDotfiles() string {
	if len(m.dotfileStatuses) == 0 {
		return dashSubtleStyle.Render("  No dotfiles configured.") + "\n"
	}

	var b strings.Builder
	b.WriteString(dashCategoryStyle.Render(fmt.Sprintf("Dotfiles (%d)", len(m.dotfileStatuses))))
	b.WriteString("\n")

	for i, d := range m.dotfileStatuses {
		cursor := "  "
		if i == m.cursor {
			cursor = dashSelectedStyle.Render("→ ")
		}

		status := dashGreenStyle.Render("✓")
		label := d.tmpl.Dest
		if !d.present {
			status = dashRedStyle.Render("✗")
			label = fmt.Sprintf("%s (missing)", label)
		} else if d.drift {
			status = dashYellowStyle.Render("~")
			label = fmt.Sprintf("%s (drifted)", label)
		}
		b.WriteString(fmt.Sprintf("  %s %s %s\n", cursor, status, dashSubtleStyle.Render(label)))
	}
	return b.String()
}

func (m dashboardModel) renderSecrets() string {
	if len(m.secretStatuses) == 0 {
		return dashSubtleStyle.Render("  No secrets configured.") + "\n"
	}

	var b strings.Builder
	b.WriteString(dashCategoryStyle.Render(fmt.Sprintf("Secrets (%d)", len(m.secretStatuses))))
	b.WriteString("\n")

	for i, s := range m.secretStatuses {
		cursor := "  "
		if i == m.cursor {
			cursor = dashSelectedStyle.Render("→ ")
		}
		status := dashYellowStyle.Render("•")
		b.WriteString(fmt.Sprintf("  %s %s %s\n", cursor, status, dashSubtleStyle.Render(s.mapping.Key)))
	}
	return b.String()
}

// --- command wiring ---

func newDashboardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "dashboard",
		Short: "Interactive dashboard to view your environment status",
		Long: `Launch an interactive TUI dashboard showing your managed packages,
dotfiles, secrets, and their current status.

Navigation:
  ↑/↓ or k/j   Move selection
  Tab          Cycle panels (Overview → Packages → Dotfiles → Secrets)
  1-4          Jump to panel directly
  q / Esc      Quit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(configPath())
			if err != nil {
				return err
			}

			m := dashboardModel{cfg: cfg}
			p := tea.NewProgram(m, tea.WithAltScreen())
			_, err = p.Run()
			return err
		},
	}
}

var dashboardCmd = newDashboardCmd()

func init() {
	rootCmd.AddCommand(dashboardCmd)
}
