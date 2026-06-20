package cmd

import (
	"fmt"

	"github.com/jfjrh2014/nestor/internal/restore"
	"github.com/spf13/cobra"
)

var (
	restoreForce   bool
	restoreDryRun  bool
	restoreOutput  string
)

var restoreCmd = &cobra.Command{
	Use:   "restore --from <url>",
	Short: "Pull a nestor.yml from a URL — perfect for team onboarding",
	Long: `restore fetches a nestor.yml config from an HTTP(S) URL, validates it,
shows a preview, and optionally writes it to disk.

Examples:
  nestor restore --from https://raw.githubusercontent.com/team/repo/main/nestor.yml
  nestor restore --from https://example.com/nestor.yml --dry-run
  nestor restore --from https://example.com/nestor.yml -o ~/.config/nestor/nestor.yml --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fromURL, _ := cmd.Flags().GetString("from")
		if fromURL == "" {
			return fmt.Errorf("--from <url> is required")
		}

		// 1. Fetch
		fmt.Printf("nestor: fetching %s\n", fromURL)
		data, err := restore.Fetch(fromURL)
		if err != nil {
			return err
		}
		fmt.Printf("nestor: fetched %d bytes\n", len(data))

		// 2. Validate
		cfg, err := restore.Validate(data)
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		fmt.Println("nestor: config is valid ✓")

		// 3. Preview
		fmt.Println("\n--- preview ---")
		fmt.Print(restore.Preview(cfg))
		fmt.Println("--- end preview ---")

		// 4. Dry run stops here
		if restoreDryRun {
			fmt.Println("nestor: dry run, not writing to disk")
			return nil
		}

		// 5. Write
		dest := restoreOutput
		if dest == "" {
			dest = "nestor.yml"
		}

		if err := restore.Write(data, dest, restoreForce); err != nil {
			return err
		}

		fmt.Printf("nestor: config written to %s\n", dest)
		fmt.Println("nestor: run 'nestor up' to apply it")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)

	restoreCmd.Flags().String("from", "", "URL to fetch nestor.yml from (required)")
	restoreCmd.Flags().BoolVarP(&restoreDryRun, "dry-run", "n", false, "fetch and preview without writing")
	restoreCmd.Flags().StringVarP(&restoreOutput, "output", "o", "", "destination path (default: ./nestor.yml)")
	restoreCmd.Flags().BoolVarP(&restoreForce, "force", "f", false, "overwrite existing file")
}
