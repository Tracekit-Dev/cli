package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/detector"
	"github.com/yourusername/context.io/cli/internal/sdk"
	"github.com/yourusername/context.io/cli/internal/trace"
	"github.com/yourusername/context.io/cli/internal/ui"
)

const CLIVersion = "2.0.0"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize TraceKit in your project",
	Long: `Initialize TraceKit by creating an account, detecting your framework,
and automatically configuring your project for monitoring.

This command will:
  1. Detect your framework (gemvc, Laravel, Express, etc.)
  2. Create a TraceKit account (or use existing)
  3. Generate an API key
  4. Create .env file with configuration
  5. Provide setup instructions

Example:
  tracekit init`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("email", "", "Your email address")
	initCmd.Flags().String("api-url", "", "API base URL (default: https://app.tracekit.dev)")
	initCmd.Flags().Bool("dev", false, "")
	initCmd.Flags().MarkHidden("dev")
}

func runInit(cmd *cobra.Command, args []string) error {
	// Read flags
	email, _ := cmd.Flags().GetString("email")
	apiURL, _ := cmd.Flags().GetString("api-url")
	useDev, _ := cmd.Flags().GetBool("dev")

	if useDev {
		apiURL = client.DevBaseURL
	}

	// Build API client
	apiClient := client.NewClient(apiURL)

	// Create wizard model
	model := newInitWizardModel(apiClient, apiURL, useDev)

	// Pre-fill email if provided via flag
	if email != "" {
		model.email = email
	}

	// Launch Bubbletea program
	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("init wizard error: %w", err)
	}

	// Check if user wants to switch to another command
	if m, ok := result.(initWizardModel); ok && m.navTarget != "" {
		return RunNavTarget(m.navTarget)
	}

	return nil
}

// promptWebhookSetup prompts user to configure a webhook
func promptWebhookSetup(cfg *config.Config, apiClient *client.Client, useDev bool) error {
	ui.PrintSection("🔗 Webhook Setup")
	fmt.Println()

	ui.PrintInfo("Set up webhooks for real-time event notifications?")
	ui.PrintMuted("   Receive instant alerts when events occur:")
	ui.PrintMuted("   • Health check failures")
	ui.PrintMuted("   • Alert triggers")
	ui.PrintMuted("   • Trace errors")
	fmt.Println()

	ui.PrintPrompt("Configure webhook now? (y/N):")
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		ui.PrintInfo("Skipping webhook setup")
		ui.PrintMuted("   You can set it up later with: tracekit webhook create")
		return nil
	}

	// Get webhook name
	fmt.Println()
	ui.PrintPrompt("Webhook name:")
	var name string
	reader := bufio.NewReader(os.Stdin)
	name, _ = reader.ReadString('\n')
	name = strings.TrimSpace(name)

	if name == "" {
		return fmt.Errorf("webhook name is required")
	}

	// Get webhook URL
	if useDev {
		ui.PrintPrompt("Webhook URL (http:// or https://):")
	} else {
		ui.PrintPrompt("Webhook URL (must be HTTPS):")
	}
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if url == "" {
		return fmt.Errorf("webhook URL is required")
	}

	if !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "http://") {
		return fmt.Errorf("webhook URL must start with https:// or http://")
	}

	// In production, enforce HTTPS (unless localhost)
	if !useDev && !strings.HasPrefix(url, "https://") && !strings.Contains(url, "localhost") && !strings.Contains(url, "127.0.0.1") {
		return fmt.Errorf("webhook URL must use HTTPS in production (got: %s)", url)
	}

	// Get description (optional)
	ui.PrintPrompt("Description (optional, press Enter to skip):")
	description, _ := reader.ReadString('\n')
	description = strings.TrimSpace(description)

	// Show available events
	fmt.Println()
	ui.PrintInfo("Select events to subscribe to:")
	ui.PrintMuted("   [1] health_check.failed")
	ui.PrintMuted("   [2] health_check.recovered")
	ui.PrintMuted("   [3] alert.triggered")
	ui.PrintMuted("   [4] alert.resolved")
	ui.PrintMuted("   [5] trace.error")
	ui.PrintMuted("   [6] anomaly.detected")
	fmt.Println()

	ui.PrintPrompt("Enter event numbers (comma-separated, e.g., 1,3,5):")
	eventsInput, _ := reader.ReadString('\n')
	eventsInput = strings.TrimSpace(eventsInput)

	availableEvents := []string{
		"health_check.failed",
		"health_check.recovered",
		"alert.triggered",
		"alert.resolved",
		"trace.error",
		"anomaly.detected",
	}

	// Parse selected events
	selectedEvents := []string{}
	if eventsInput != "" {
		selections := strings.Split(eventsInput, ",")
		for _, sel := range selections {
			sel = strings.TrimSpace(sel)
			var eventNum int
			fmt.Sscanf(sel, "%d", &eventNum)
			if eventNum >= 1 && eventNum <= len(availableEvents) {
				selectedEvents = append(selectedEvents, availableEvents[eventNum-1])
			}
		}
	}

	if len(selectedEvents) == 0 {
		return fmt.Errorf("at least one event must be selected")
	}

	// Create webhook via API
	payload := map[string]interface{}{
		"name":        name,
		"url":         url,
		"description": description,
		"events":      selectedEvents,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to create payload: %w", err)
	}

	// Use the API client's base URL
	req, err := http.NewRequest("POST", apiClient.BaseURL+"/v1/webhooks", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create webhook: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create webhook: %s", string(body))
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	webhookID := result["id"].(string)
	secret := result["secret"].(string)

	// Save to .env
	envPath := ".env"
	envFile, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		ui.PrintWarning(fmt.Sprintf("Could not save webhook to .env: %v", err))
	} else {
		defer envFile.Close()
		envFile.WriteString(fmt.Sprintf("\n# Webhook Configuration\n"))
		envFile.WriteString(fmt.Sprintf("TRACEKIT_WEBHOOK_ID=%s\n", webhookID))
		envFile.WriteString(fmt.Sprintf("TRACEKIT_WEBHOOK_URL=%s\n", url))
		envFile.WriteString(fmt.Sprintf("TRACEKIT_WEBHOOK_SECRET=%s\n", secret))
	}

	fmt.Println()
	ui.PrintSuccess("✅ Webhook created successfully!")
	fmt.Println()
	ui.PrintInfo(fmt.Sprintf("Webhook ID: %s", webhookID))
	ui.PrintInfo(fmt.Sprintf("URL: %s", url))
	fmt.Println()
	ui.PrintWarning("⚠️  IMPORTANT: Your webhook secret has been saved to .env")
	ui.PrintMuted(fmt.Sprintf("   Secret: %s", secret))
	ui.PrintMuted("   Use this secret to verify webhook signatures")
	fmt.Println()
	ui.PrintInfo("Subscribed to events:")
	for _, event := range selectedEvents {
		ui.PrintMuted(fmt.Sprintf("   • %s", event))
	}

	return nil
}

// promptHealthCheckSetup prompts user to configure health check monitoring
func promptHealthCheckSetup(cfg *config.Config, apiClient *client.Client) error {
	ui.PrintSection("🏥 Health Check Setup")
	fmt.Println()

	ui.PrintInfo("Set up health check monitoring for your service?")
	ui.PrintMuted("   Monitor your service health with automatic alerts")
	ui.PrintMuted("   • Pull-based: TraceKit pings your endpoint")
	ui.PrintMuted("   • Push-based: Your service sends heartbeats")
	fmt.Println()

	ui.PrintPrompt("Configure health check now? (Y/n):")
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "n" || response == "no" {
		ui.PrintInfo("Skipping health check setup")
		ui.PrintMuted("   You can set it up later with: tracekit health setup")
		return nil
	}

	// User wants to set up health check
	fmt.Println()
	ui.PrintInfo("Choose health check type:")
	ui.PrintMuted("   [1] Pull-based - TraceKit pings your endpoint (recommended)")
	ui.PrintMuted("   [2] Push-based - Your service sends heartbeats")
	ui.PrintMuted("   [0] Skip for now")
	fmt.Println()

	ui.PrintPrompt("Select type (0-2):")
	var typeChoice string
	fmt.Scanln(&typeChoice)
	typeChoice = strings.TrimSpace(typeChoice)

	if typeChoice == "0" {
		ui.PrintInfo("Skipping health check setup")
		ui.PrintMuted("   You can set it up later with: tracekit health setup")
		return nil
	}

	if typeChoice == "1" {
		return setupPullBasedHealthCheckFromInit(cfg, apiClient)
	} else if typeChoice == "2" {
		return setupPushBasedHealthCheckFromInit(cfg, apiClient)
	}

	ui.PrintWarning("Invalid selection, skipping health check setup")
	return nil
}

// setupPullBasedHealthCheckFromInit sets up pull-based health check during init
func setupPullBasedHealthCheckFromInit(cfg *config.Config, apiClient *client.Client) error {
	fmt.Println()
	ui.PrintInfo("Pull-based Health Check Configuration")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Get endpoint URL
	ui.PrintPrompt("Health check endpoint URL (e.g., https://myapp.com/health):")
	endpointURL, _ := reader.ReadString('\n')
	endpointURL = strings.TrimSpace(endpointURL)

	if endpointURL == "" {
		ui.PrintWarning("Endpoint URL is required")
		ui.PrintMuted("   Run 'tracekit health setup' to configure later")
		return fmt.Errorf("endpoint URL required")
	}
	fmt.Println()

	// Use sensible defaults for init flow (no prompts)
	checkName := "health-check"
	interval := 60
	expectedStatus := 200

	ui.PrintInfo("Using default settings:")
	ui.PrintMuted(fmt.Sprintf("   Check name: %s", checkName))
	ui.PrintMuted(fmt.Sprintf("   Interval: %d seconds", interval))
	ui.PrintMuted(fmt.Sprintf("   Expected status: %d", expectedStatus))
	fmt.Println()

	// Create health check via API
	ui.PrintInfo("Creating health check...")

	requestBody := map[string]interface{}{
		"service_name":              cfg.ServiceName,
		"check_name":                checkName,
		"endpoint_url":              endpointURL,
		"check_method":              "GET",
		"expected_status_code":      expectedStatus,
		"check_interval_seconds":    interval,
		"alert_enabled":             true,
	}

	apiURL := strings.Replace(cfg.Endpoint, "/v1/traces", "", 1)
	if err := apiClient.PostHealthCheck(apiURL, cfg.APIKey, requestBody); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create health check: %v", err))
		ui.PrintMuted("   Run 'tracekit health setup' to try again")
		return err
	}

	ui.PrintSuccess("Health check configured!")
	fmt.Println()

	ui.PrintMuted("✓ TraceKit will ping your endpoint every 60 seconds")
	ui.PrintMuted("✓ Alerts triggered if 3 consecutive checks fail")
	ui.PrintMuted("✓ Run 'tracekit health list' to view status")

	return nil
}

// setupPushBasedHealthCheckFromInit sets up push-based health check during init
func setupPushBasedHealthCheckFromInit(cfg *config.Config, apiClient *client.Client) error {
	fmt.Println()
	ui.PrintInfo("Push-based Health Check Configuration")
	fmt.Println()

	interval := 60
	gracePeriod := 30
	checkName := "heartbeat"

	ui.PrintMuted("Your service should send heartbeats every 60 seconds")
	fmt.Println()

	// Create health check via API
	ui.PrintInfo("Creating health check...")

	requestBody := map[string]interface{}{
		"service_name":               cfg.ServiceName,
		"check_name":                 checkName,
		"check_type":                 "push",
		"heartbeat_interval_seconds": interval,
		"grace_period_seconds":       gracePeriod,
		"alert_enabled":              true,
	}

	apiURL := strings.Replace(cfg.Endpoint, "/v1/traces", "", 1)
	if err := apiClient.PostHealthCheck(apiURL, cfg.APIKey, requestBody); err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create health check: %v", err))
		ui.PrintMuted("   Run 'tracekit health setup' to try again")
		return err
	}

	ui.PrintSuccess("Health check configured!")
	fmt.Println()

	// Show implementation example
	ui.PrintInfo("Add this code to your service:")
	fmt.Println()

	heartbeatEndpoint := apiURL + "/v1/health/heartbeat"

	ui.PrintMuted("Example (adapt for your language):")
	fmt.Printf(`
  POST %s
  Headers: X-API-Key: %s
  Body: {
    "service_name": "%s",
    "status": "healthy"
  }

  Send this request every %d seconds
`, heartbeatEndpoint, cfg.APIKey, cfg.ServiceName, interval)

	fmt.Println()

	ui.PrintMuted("✓ Send heartbeats from your service every 60 seconds")
	ui.PrintMuted("✓ Alerts triggered if 3 heartbeats are missed")
	ui.PrintMuted("✓ Run 'tracekit health list' to view status")

	return nil
}

func promptEmail() (string, error) {
	ui.PrintPrompt("Enter your email:")
	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read email: %w", err)
	}

	email = strings.TrimSpace(email)
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	return email, nil
}

// sendTestTraceInternal sends a test trace (reused from test.go logic)
func sendTestTraceInternal(cfg *config.Config) error {
	testTrace := trace.GenerateTestTrace(cfg.ServiceName)
	return trace.SendTrace(cfg, testTrace)
}

// showStatusInternal shows integration status (reused from status.go logic)
func showStatusInternal(cfg *config.Config, apiClient *client.Client, useDev bool) error {
	// Detect framework
	framework, _ := detector.Detect()
	if framework != nil && framework.Name != "generic" {
		ui.PrintMuted(fmt.Sprintf("   Framework: %s (%s)", framework.Name, framework.Type))
	}

	// Get integration status from API
	apiClient.APIKey = cfg.APIKey
	status, err := apiClient.GetStatus()
	if err != nil {
		return err
	}

	// Display integration status
	if statusStr, ok := status["status"].(string); ok && statusStr == "active" {
		if integration, ok := status["integration"].(map[string]interface{}); ok {
			ui.PrintMuted(fmt.Sprintf("   Service:     %v", integration["service_name"]))
			ui.PrintMuted(fmt.Sprintf("   Source:      %v", integration["source"]))

			if firstData := integration["first_data_at"]; firstData != nil {
				ui.PrintMuted(fmt.Sprintf("   First trace: %v", firstData))
			} else {
				ui.PrintMuted("   First trace: Just now!")
			}
		}
	}

	return nil
}

// promptSDKInstall prompts user to install SDK
func promptSDKInstall(framework *detector.Framework) error {
	ui.PrintSection("📦 SDK Installation")
	fmt.Println()

	// Get recommended SDK
	recommendedSDK := sdk.GetRecommendedSDK(framework.Type, framework.Name)
	if recommendedSDK == nil {
		ui.PrintInfo("No SDK recommendation available for your framework")
		ui.PrintMuted("   Visit https://docs.tracekit.dev for manual setup")
		return nil
	}

	// Show recommendation
	ui.PrintInfo(fmt.Sprintf("Recommended: %s", recommendedSDK.Name))
	ui.PrintMuted(fmt.Sprintf("   %s", recommendedSDK.Description))
	fmt.Println()

	// Prompt user
	fmt.Println("Install " + recommendedSDK.Name + " now?")
	ui.PrintMuted("   Y     = Install recommended SDK")
	ui.PrintMuted("   n     = Skip installation")
	ui.PrintMuted("   other = Show all available SDKs")
	fmt.Println()
	ui.PrintPrompt("Your choice:")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		// Install recommended SDK
		return installSDK(*recommendedSDK)
	} else if response == "n" || response == "no" {
		ui.PrintInfo("Skipping SDK installation")
		fmt.Println()
		ui.PrintMuted("You can install manually later:")
		ui.PrintMuted("   " + recommendedSDK.InstallCmd)
		return nil
	} else {
		// Show all available SDKs
		return promptSDKSelection()
	}
}

// promptSDKSelection shows all SDKs and lets user choose
func promptSDKSelection() error {
	fmt.Println()
	ui.PrintInfo("Available SDKs:")
	fmt.Println()

	sdks := sdk.GetAvailableSDKs()
	for i, s := range sdks {
		ui.PrintMuted(fmt.Sprintf("   %d. %s - %s", i+1, s.Name, s.Description))
	}
	fmt.Println()

	ui.PrintPrompt("Select SDK number (or 0 to skip):")
	var choice int
	fmt.Scanln(&choice)

	if choice == 0 {
		ui.PrintInfo("Skipping SDK installation")
		return nil
	}

	if choice < 1 || choice > len(sdks) {
		return fmt.Errorf("invalid selection")
	}

	selectedSDK := sdks[choice-1]
	return installSDK(selectedSDK)
}

// installSDK installs the selected SDK
func installSDK(selectedSDK sdk.SDK) error {
	fmt.Println()
	ui.PrintInfo(fmt.Sprintf("Installing %s...", selectedSDK.Name))
	ui.PrintMuted("   Running: " + selectedSDK.InstallCmd)
	fmt.Println()

	if err := sdk.Install(selectedSDK); err != nil {
		ui.PrintError(fmt.Sprintf("Installation failed: %v", err))
		fmt.Println()
		ui.PrintMuted("Please install manually:")
		ui.PrintMuted("   " + selectedSDK.InstallCmd)
		return err
	}

	ui.PrintSuccess(fmt.Sprintf("%s installed successfully!", selectedSDK.Name))
	fmt.Println()

	// Show initialization instructions
	instructions := sdk.GetInstallInstructions(selectedSDK)
	ui.PrintInfo("Next steps:")
	for _, instruction := range instructions {
		if instruction == "" {
			fmt.Println()
		} else {
			ui.PrintMuted("   " + instruction)
		}
	}

	return nil
}
