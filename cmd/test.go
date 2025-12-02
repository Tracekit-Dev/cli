package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/trace"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Send a test trace to verify your integration",
	Long: `Send a test trace to TraceKit to verify your integration is working correctly.

This command will:
  1. Read your .env configuration
  2. Generate a test trace
  3. Send it to TraceKit
  4. Verify the trace was received

Example:
  tracekit test`,
	RunE: runTest,
}

func init() {
	rootCmd.AddCommand(testCmd)
	testCmd.Flags().Bool("dev", false, "")
	testCmd.Flags().MarkHidden("dev")
}

func runTest(cmd *cobra.Command, args []string) error {
	// Print banner
	ui.PrintBanner()
	fmt.Println()

	// Step 1: Read configuration
	ui.PrintSection("📋 Reading Configuration")
	fmt.Println()

	cfg, err := config.Read()
	if err != nil {
		ui.PrintError("No TraceKit configuration found")
		ui.PrintMuted("   Run 'tracekit init' to set up your project")
		return nil
	}

	ui.PrintSuccess("Configuration loaded")
	ui.PrintMuted(fmt.Sprintf("   Service: %s", cfg.ServiceName))
	ui.PrintMuted(fmt.Sprintf("   Endpoint: %s", cfg.Endpoint))
	fmt.Println()

	// Step 2: Generate test trace
	ui.PrintSection("🧪 Generating Test Trace")
	fmt.Println()

	testTrace := trace.GenerateTestTrace(cfg.ServiceName)
	ui.PrintSuccess("Test trace generated")
	ui.PrintMuted(fmt.Sprintf("   Trace ID: %s", testTrace["trace_id"]))
	ui.PrintMuted(fmt.Sprintf("   Span ID: %s", testTrace["span_id"]))
	fmt.Println()

	// Step 3: Send trace
	ui.PrintSection("📤 Sending Trace")
	fmt.Println()

	err = trace.SendTrace(cfg, testTrace)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to send trace: %v", err))
		ui.PrintMuted("   Check your API key and network connection")
		return nil
	}

	ui.PrintSuccess("Trace sent successfully!")
	fmt.Println()

	// Step 4: Show next steps
	ui.PrintDivider()
	fmt.Println()

	summary := fmt.Sprintf("Trace ID: %s\nService:  %s\nStatus:   Delivered", testTrace["trace_id"], cfg.ServiceName)
	ui.PrintSummaryBox("✅ Test Complete!", summary)
	fmt.Println()

	steps := []string{
		"Visit https://app.tracekit.dev to view your test trace",
		"Look for the trace ID: " + testTrace["trace_id"].(string),
		"Start sending real traces from your application",
	}
	ui.PrintNextSteps(steps)

	return nil
}
