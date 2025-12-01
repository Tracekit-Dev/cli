package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
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

	trace := generateTestTrace(cfg.ServiceName)
	ui.PrintSuccess("Test trace generated")
	ui.PrintMuted(fmt.Sprintf("   Trace ID: %s", trace["trace_id"]))
	ui.PrintMuted(fmt.Sprintf("   Span ID: %s", trace["span_id"]))
	fmt.Println()

	// Step 3: Send trace
	ui.PrintSection("📤 Sending Trace")
	fmt.Println()

	err = sendTrace(cfg, trace)
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

	summary := fmt.Sprintf("Trace ID: %s\nService:  %s\nStatus:   Delivered", trace["trace_id"], cfg.ServiceName)
	ui.PrintSummaryBox("✅ Test Complete!", summary)
	fmt.Println()

	steps := []string{
		"Visit https://app.tracekit.dev to view your test trace",
		"Look for the trace ID: " + trace["trace_id"].(string),
		"Start sending real traces from your application",
	}
	ui.PrintNextSteps(steps)

	return nil
}

// generateTestTrace creates a test trace payload
func generateTestTrace(serviceName string) map[string]interface{} {
	now := time.Now()
	traceID := uuid.New().String()
	spanID := uuid.New().String()

	return map[string]interface{}{
		"trace_id":  traceID,
		"span_id":   spanID,
		"parent_id": nil,
		"name":      "CLI Test Trace",
		"kind":      "internal",
		"timestamp": now.UnixMilli(),
		"duration":  150, // 150ms simulated duration
		"service": map[string]interface{}{
			"name":    serviceName,
			"version": "1.0.0",
		},
		"resource": map[string]interface{}{
			"type": "cli_test",
			"name": "tracekit test",
		},
		"attributes": map[string]interface{}{
			"test":         true,
			"source":       "tracekit-cli",
			"cli_version":  CLIVersion,
			"generated_at": now.Format(time.RFC3339),
		},
		"events": []map[string]interface{}{
			{
				"timestamp": now.UnixMilli(),
				"name":      "test.start",
				"attributes": map[string]interface{}{
					"message": "TraceKit CLI test trace initiated",
				},
			},
			{
				"timestamp": now.Add(50 * time.Millisecond).UnixMilli(),
				"name":      "test.processing",
				"attributes": map[string]interface{}{
					"message": "Processing test trace",
				},
			},
			{
				"timestamp": now.Add(150 * time.Millisecond).UnixMilli(),
				"name":      "test.complete",
				"attributes": map[string]interface{}{
					"message": "Test trace completed successfully",
				},
			},
		},
		"status": map[string]interface{}{
			"code":    "ok",
			"message": "Test trace completed",
		},
	}
}

// sendTrace sends the trace to TraceKit endpoint
func sendTrace(cfg *config.Config, trace map[string]interface{}) error {
	// Determine endpoint
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.tracekit.dev/v1/traces"
	}

	// Prepare request body
	body, err := json.Marshal(trace)
	if err != nil {
		return fmt.Errorf("failed to marshal trace: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)
	req.Header.Set("User-Agent", "TraceKit-CLI/"+CLIVersion)

	// Send request
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Read error response
		var errBody bytes.Buffer
		errBody.ReadFrom(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errBody.String())
	}

	return nil
}
