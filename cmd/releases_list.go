package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var releasesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all releases",
	Long: `List all releases with pagination support.

Shows a table of releases with version, source, creation date,
finalization status, and deploy count.

Examples:
  tracekit releases list
  tracekit releases list --page 2 --page-size 10
  tracekit releases list --json`,
	RunE: runReleasesList,
}

func init() {
	releasesListCmd.Flags().Int("page", 1, "Page number")
	releasesListCmd.Flags().Int("page-size", 20, "Number of releases per page")
}

func runReleasesList(cmd *cobra.Command, args []string) error {
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

	page, _ := cmd.Flags().GetInt("page")
	pageSize, _ := cmd.Flags().GetInt("page-size")
	service, _ := cmd.Flags().GetString("service")

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// List releases
	result, err := c.ListReleases(page, pageSize, service)
	if err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
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

	if result.Total == 0 {
		ui.PrintMuted("No releases found.")
		fmt.Println("\nCreate your first release:")
		fmt.Println("  tracekit releases new v1.0.0")
		return nil
	}

	fmt.Printf("\nReleases (page %d, %d total):\n\n", result.Page, result.Total)
	fmt.Printf("  %-20s %-20s %-15s %-20s %-20s\n", "VERSION", "SERVICE", "SOURCE", "CREATED", "FINALIZED")
	fmt.Printf("  %-20s %-20s %-15s %-20s %-20s\n", "-------", "-------", "------", "-------", "---------")

	for _, r := range result.Releases {
		finalized := "-"
		if r.FinalizedAt != nil {
			finalized = *r.FinalizedAt
			// Truncate to date portion if it's a full timestamp
			if len(finalized) > 19 {
				finalized = finalized[:19]
			}
		}

		created := r.CreatedAt
		if len(created) > 19 {
			created = created[:19]
		}

		svcName := r.ServiceName
		if svcName == "" {
			svcName = "-"
		}

		fmt.Printf("  %-20s %-20s %-15s %-20s %-20s\n", r.Version, svcName, r.Source, created, finalized)
	}

	if len(result.Environments) > 0 {
		fmt.Printf("\nEnvironments: ")
		for i, env := range result.Environments {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(env)
		}
		fmt.Println()
	}

	fmt.Println()
	return nil
}
