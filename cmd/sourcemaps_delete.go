package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var sourcemapsDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete source maps for a release",
	Long: `Delete all source maps associated with a specific release.

Requires the --release flag to specify which release's source maps to delete.

Examples:
  tracekit sourcemaps delete --release v1.2.3
  tracekit sourcemaps delete --release v1.2.3 --json`,
	RunE: runSourcemapsDelete,
}

func init() {
	sourcemapsDeleteCmd.Flags().String("release", "", "Release version to delete source maps for (required)")
	sourcemapsDeleteCmd.MarkFlagRequired("release")
}

func runSourcemapsDelete(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.ReadWithFallback(EnvFlag)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if cfg.APIKey == "" {
		return fmt.Errorf("not authenticated. Run 'tracekit login' first")
	}

	// Check --dev flag
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		cfg.Endpoint = "http://localhost:8081"
	}

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Get release flag
	release, _ := cmd.Flags().GetString("release")
	if release == "" {
		return fmt.Errorf("--release flag is required")
	}

	// Delete source maps
	result, err := c.DeleteSourceMaps(release)
	if err != nil {
		return fmt.Errorf("failed to delete source maps: %w", err)
	}

	// Output
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		out, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	if result.DeletedCount > 0 {
		ui.PrintSuccess(fmt.Sprintf("Deleted %d source map(s) for release %s", result.DeletedCount, release))
	} else {
		ui.PrintInfo(fmt.Sprintf("No source maps found for release %s", release))
	}

	return nil
}
