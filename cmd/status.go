package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/detector"
	"github.com/yourusername/context.io/cli/internal/ui"
	"github.com/yourusername/context.io/cli/internal/utils"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show TraceKit configuration and integration status",
	Long: `Display your current TraceKit configuration, integration status,
and usage information.

This command will:
  1. Check your .env file for TraceKit configuration
  2. Verify your API key is valid
  3. Show your integration status
  4. Display framework detection results

Example:
  tracekit status`,
	RunE: runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().Bool("dev", false, "")
	statusCmd.Flags().MarkHidden("dev")
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Print banner
	ui.PrintBanner()
	fmt.Println()

	// Step 1: Check for .env file and read config
	ui.PrintSection("📋 Configuration")
	fmt.Println()

	cfg, err := config.Read()
	if err != nil {
		ui.PrintError("No TraceKit configuration found")
		ui.PrintMuted("   Run 'tracekit init' to set up your project")
		return nil
	}

	// Display config (mask API key)
	ui.PrintSuccess("Configuration found in .env")
	fmt.Println()
	ui.PrintMuted(fmt.Sprintf("   API Key:      %s", utils.MaskAPIKey(cfg.APIKey)))
	ui.PrintMuted(fmt.Sprintf("   Endpoint:     %s", cfg.Endpoint))
	ui.PrintMuted(fmt.Sprintf("   Service:      %s", cfg.ServiceName))
	ui.PrintMuted(fmt.Sprintf("   Enabled:      %s", cfg.Enabled))
	ui.PrintMuted(fmt.Sprintf("   Code Monitor: %s", cfg.CodeMonitoringEnabled))
	fmt.Println()

	// Step 2: Detect framework
	ui.PrintSection("🔍 Framework Detection")
	fmt.Println()

	framework, err := detector.Detect()
	if err != nil {
		ui.PrintWarning("Framework detection failed")
	} else if framework.Name == "generic" {
		ui.PrintWarning("No framework detected")
	} else {
		ui.PrintSuccess(fmt.Sprintf("Detected: %s (%s)", framework.Name, framework.Type))
		if framework.Version != "" {
			ui.PrintMuted(fmt.Sprintf("   Version: %s", framework.Version))
		}
	}
	fmt.Println()

	// Step 3: Check API key validity and get integration status
	ui.PrintSection("🔌 Integration Status")
	fmt.Println()

	// Determine API URL
	useDev, _ := cmd.Flags().GetBool("dev")
	apiURL := client.DefaultBaseURL
	if useDev {
		apiURL = client.DevBaseURL
		ui.PrintInfo("Using development API: " + apiURL)
		fmt.Println()
	}

	apiClient := client.NewClient(apiURL)
	apiClient.APIKey = cfg.APIKey

	status, err := apiClient.GetStatus()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to verify API key: %v", err))
		ui.PrintMuted("   Your API key may be invalid or revoked")
		ui.PrintMuted("   Run 'tracekit init' to generate a new API key")
		return nil
	}

	// Display integration status
	if statusStr, ok := status["status"].(string); ok && statusStr == "active" {
		ui.PrintSuccess("Integration active")

		if integration, ok := status["integration"].(map[string]interface{}); ok {
			fmt.Println()
			ui.PrintMuted(fmt.Sprintf("   Service:      %v", integration["service_name"]))
			ui.PrintMuted(fmt.Sprintf("   Type:         %v", integration["integration_type"]))
			ui.PrintMuted(fmt.Sprintf("   Source:       %v", integration["source"]))

			if firstData := integration["first_data_at"]; firstData != nil {
				ui.PrintMuted(fmt.Sprintf("   First trace:  %v", firstData))
			} else {
				ui.PrintMuted("   First trace:  No traces received yet")
			}

			if lastData := integration["last_data_at"]; lastData != nil {
				ui.PrintMuted(fmt.Sprintf("   Last trace:   %v", lastData))
			}
		}
	} else if statusStr == "no_integration" {
		ui.PrintWarning("No integration found")
		ui.PrintMuted("   Run 'tracekit init' to set up integration")
	}

	fmt.Println()
	ui.PrintDivider()
	fmt.Println()

	// Show next steps
	var steps []string
	if framework.Name == "generic" || framework.Name == "" {
		steps = []string{
			"Install the appropriate TraceKit SDK for your language",
			"Initialize the SDK with your API key",
			"Send a test trace with 'tracekit test'",
		}
	} else {
		// Check if first_data_at is nil (no traces received)
		hasTraces := false
		if integration, ok := status["integration"].(map[string]interface{}); ok {
			if firstData := integration["first_data_at"]; firstData != nil {
				hasTraces = true
			}
		}

		if !hasTraces {
			steps = []string{
				fmt.Sprintf("Install SDK for %s", framework.Name),
				"Initialize the SDK in your application",
				"Send a test trace with 'tracekit test'",
				"Visit https://app.tracekit.dev to view traces",
			}
		} else {
			steps = []string{
				"Visit https://app.tracekit.dev to view traces",
				"Send a test trace with 'tracekit test'",
			}
		}
	}

	ui.PrintNextSteps(steps)

	return nil
}
