package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/version"
)

var releasesFinalizeCmd = &cobra.Command{
	Use:   "finalize [version]",
	Short: "Mark a release as finalized",
	Long: `Mark a release as finalized to indicate it is complete and deployed.

If no version is provided, auto-detects from package.json version field,
then falls back to the latest git tag.

Examples:
  tracekit releases finalize v1.2.3
  tracekit releases finalize              # auto-detect version
  tracekit releases finalize --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runReleasesFinalize,
}

func runReleasesFinalize(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Read()
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

	// Determine version
	var ver string
	if len(args) > 0 {
		ver = args[0]
	} else {
		detected, err := version.DetectVersion()
		if err != nil {
			return fmt.Errorf("version detection failed: %w", err)
		}
		ver = detected
	}

	service, _ := cmd.Flags().GetString("service")

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Finalize release
	release, err := c.FinalizeRelease(ver, service)
	if err != nil {
		return fmt.Errorf("failed to finalize release: %w", err)
	}

	// Output
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		out, err := json.MarshalIndent(release, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	finalizedAt := "now"
	if release.FinalizedAt != nil {
		finalizedAt = *release.FinalizedAt
	}
	ui.PrintSuccess(fmt.Sprintf("Finalized release %s at %s", release.Version, finalizedAt))

	return nil
}
