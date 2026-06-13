package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var profilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available profiles in your config",
	RunE: func(cmd *cobra.Command, args []string) error {
		p := ui.New(os.Stdout)

		path := configPath()
		cfg, err := config.Load(path)
		if err != nil {
			return fmt.Errorf("profiles: %w", err)
		}

		if len(cfg.Profiles) == 0 {
			p.Info("no profiles defined")
			return nil
		}

		p.Header("profiles")

		// sorted for deterministic output
		names := make([]string, 0, len(cfg.Profiles))
		for name := range cfg.Profiles {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			prof := cfg.Profiles[name]
			p.OK(fmt.Sprintf("%s (%d packages)", name, len(prof.Packages)))
			for _, pkg := range prof.Packages {
				p.Detail("package", pkg)
			}
		}

		p.Info(fmt.Sprintf("%d profile(s) total", len(names)))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profilesCmd)
}
