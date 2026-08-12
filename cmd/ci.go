package cmd

import (
	"fmt"
	"io"

	"github.com/jfjrh2014/nestor/internal/ci"
	"github.com/jfjrh2014/nestor/internal/config"
	"github.com/jfjrh2014/nestor/internal/ui"
	"github.com/spf13/cobra"
)

var ciQuiet bool

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Validate your nestor.yml config from CI",
	Long: `Validates your nestor.yml without installing anything,
writing files, or touching the network. CI-safe.

Use in CI: run 'nestor ci' after checkout. Exits non-zero if
the config has errors.

Flags:
  -q, --quiet     Only output on failure (CI log noise reduction)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runCI(cmd.OutOrStdout())
	},
}

func init() {
	ciCmd.Flags().BoolVarP(&ciQuiet, "quiet", "q", false, "only output on failure")
	rootCmd.AddCommand(ciCmd)
}

func runCI(w io.Writer) error {
	path := configPath()
	cfg, err := config.Load(path)
	if err != nil {
		return fmt.Errorf("ci: %w", err)
	}

	report := ci.Validate(cfg, cfg.Dotfiles.Source)

	if ciQuiet && !report.HasErrors() {
		// quiet mode: no output on success
		return nil
	}

	p := ui.New(w)

	if len(report.Findings) == 0 {
		p.OK(fmt.Sprintf("config valid (%s)", path))
		return nil
	}

	for _, f := range report.Findings {
		switch f.Severity {
		case ci.SeverityError:
			p.Error(fmt.Sprintf("[%s] %s", f.Category, f.Message))
		case ci.SeverityWarning:
			p.Warn(fmt.Sprintf("[%s] %s", f.Category, f.Message))
		}
	}

	p.Info(fmt.Sprintf("%d error(s), %d warning(s)", report.ErrorCount(), report.WarnCount()))

	if report.HasErrors() {
		return fmt.Errorf("config validation failed with %d error(s)", report.ErrorCount())
	}
	return nil
}
