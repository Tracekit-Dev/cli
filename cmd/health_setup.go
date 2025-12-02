package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var healthSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure a new health check",
	Long: `Configure a new health check for monitoring your service endpoints.

This command will guide you through setting up either:
  - Pull-based health check (TraceKit pings your endpoint)
  - Push-based health check (your service sends heartbeats)

Example:
  tracekit health setup`,
	RunE: runHealthSetup,
}

func init() {
	healthSetupCmd.Flags().Bool("dev", false, "")
	healthSetupCmd.Flags().MarkHidden("dev")
}

func runHealthSetup(cmd *cobra.Command, args []string) error {
	// Print banner
	ui.PrintBanner()
	fmt.Println()

	// Read configuration
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
	fmt.Println()

	// Choose health check type
	ui.PrintSection("🏥 Health Check Type")
	fmt.Println()
	ui.PrintInfo("Choose the type of health check:")
	ui.PrintMuted("   [1] Pull-based - TraceKit pings your endpoint")
	ui.PrintMuted("   [2] Push-based - Your service sends heartbeats")
	fmt.Println()

	ui.PrintPrompt("Select type (1 or 2):")
	reader := bufio.NewReader(os.Stdin)
	typeInput, _ := reader.ReadString('\n')
	typeInput = strings.TrimSpace(typeInput)
	fmt.Println()

	if typeInput == "1" {
		return setupPullBasedHealthCheck(cfg)
	} else if typeInput == "2" {
		return setupPushBasedHealthCheck(cfg)
	} else {
		ui.PrintError("Invalid selection")
		return nil
	}
}

func setupPullBasedHealthCheck(cfg *config.Config) error {
	ui.PrintSection("🔍 Pull-Based Health Check Setup")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Get check name
	ui.PrintPrompt("Check name (e.g., 'api-health'):")
	checkName, _ := reader.ReadString('\n')
	checkName = strings.TrimSpace(checkName)
	fmt.Println()

	if checkName == "" {
		checkName = "health-check"
	}

	// Get endpoint URL
	ui.PrintPrompt("Endpoint URL to monitor:")
	endpointURL, _ := reader.ReadString('\n')
	endpointURL = strings.TrimSpace(endpointURL)
	fmt.Println()

	if endpointURL == "" {
		ui.PrintError("Endpoint URL is required")
		return nil
	}

	// Get check interval
	ui.PrintPrompt("Check interval in seconds (default: 60):")
	intervalInput, _ := reader.ReadString('\n')
	intervalInput = strings.TrimSpace(intervalInput)
	fmt.Println()

	interval := 60
	if intervalInput != "" {
		parsedInterval, err := strconv.Atoi(intervalInput)
		if err == nil && parsedInterval > 0 {
			interval = parsedInterval
		}
	}

	// Get expected status code
	ui.PrintPrompt("Expected HTTP status code (default: 200):")
	statusInput, _ := reader.ReadString('\n')
	statusInput = strings.TrimSpace(statusInput)
	fmt.Println()

	expectedStatus := 200
	if statusInput != "" {
		parsedStatus, err := strconv.Atoi(statusInput)
		if err == nil && parsedStatus > 0 {
			expectedStatus = parsedStatus
		}
	}

	// Create health check via API
	ui.PrintSection("📤 Creating Health Check")
	fmt.Println()

	requestBody := map[string]interface{}{
		"service_name":          cfg.ServiceName,
		"check_name":            checkName,
		"endpoint_url":          endpointURL,
		"check_method":          "GET",
		"expected_status_code":  expectedStatus,
		"check_interval_seconds": interval,
		"alert_enabled":         true,
	}

	bodyBytes, _ := json.Marshal(requestBody)

	// Determine API URL
	apiURL := strings.Replace(cfg.Endpoint, "/v1/traces", "", 1)
	req, err := http.NewRequest("POST", apiURL+"/api/health-checks", bytes.NewReader(bodyBytes))
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create request: %v", err))
		return nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", cfg.APIKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Request failed: %v", err))
		return nil
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if resp.StatusCode != http.StatusCreated {
		errorMsg := "Unknown error"
		if msg, ok := result["error"].(string); ok {
			errorMsg = msg
		}
		ui.PrintError(fmt.Sprintf("Failed to create health check: %s", errorMsg))
		return nil
	}

	ui.PrintSuccess("Health check created!")
	fmt.Println()

	// Show summary
	ui.PrintDivider()
	fmt.Println()

	summary := fmt.Sprintf("Type:       Pull-based (TraceKit pings your endpoint)\nService:    %s\nCheck:      %s\nEndpoint:   %s\nInterval:   Every %d seconds\nStatus:     %d",
		cfg.ServiceName, checkName, endpointURL, interval, expectedStatus)

	ui.PrintSummaryBox("✅ Health Check Configured", summary)
	fmt.Println()

	// Next steps
	steps := []string{
		fmt.Sprintf("TraceKit will ping %s every %d seconds", endpointURL, interval),
		"Alerts will be triggered if 3 consecutive checks fail",
		"Run 'tracekit health list' to view all health checks",
		"View alerts in your dashboard",
	}
	ui.PrintNextSteps(steps)

	return nil
}

func setupPushBasedHealthCheck(cfg *config.Config) error {
	ui.PrintSection("📡 Push-Based Health Check Setup")
	fmt.Println()

	ui.PrintInfo("Push-based health checks require your service to send heartbeats")
	ui.PrintMuted("   Your service sends periodic heartbeats to TraceKit")
	ui.PrintMuted("   Alerts are triggered if heartbeats stop")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	// Get heartbeat interval
	ui.PrintPrompt("Expected heartbeat interval in seconds (default: 60):")
	intervalInput, _ := reader.ReadString('\n')
	intervalInput = strings.TrimSpace(intervalInput)
	fmt.Println()

	interval := 60
	if intervalInput != "" {
		parsedInterval, err := strconv.Atoi(intervalInput)
		if err == nil && parsedInterval > 0 {
			interval = parsedInterval
		}
	}

	// Show implementation instructions
	ui.PrintDivider()
	fmt.Println()

	summary := fmt.Sprintf("Type:       Push-based (Your service sends heartbeats)\nService:    %s\nInterval:   Every %d seconds\nEndpoint:   POST %s/v1/health/heartbeat",
		cfg.ServiceName, interval, strings.Replace(cfg.Endpoint, "/v1/traces", "", 1))

	ui.PrintSummaryBox("✅ Configuration Ready", summary)
	fmt.Println()

	// Show implementation code
	ui.PrintSection("📝 Implementation")
	fmt.Println()

	ui.PrintInfo("Add this code to your service:")
	fmt.Println()

	ui.PrintMuted("Go:")
	fmt.Println(`
  import "net/http"
  import "bytes"
  import "encoding/json"

  func sendHeartbeat() {
      payload := map[string]interface{}{
          "service_name": "` + cfg.ServiceName + `",
          "status": "healthy",
          "metadata": map[string]interface{}{
              "uptime_seconds": getUptime(),
              "memory_mb": getMemoryUsage(),
          },
      }
      body, _ := json.Marshal(payload)
      req, _ := http.NewRequest("POST", "` + strings.Replace(cfg.Endpoint, "/v1/traces", "", 1) + `/v1/health/heartbeat", bytes.NewReader(body))
      req.Header.Set("Content-Type", "application/json")
      req.Header.Set("X-API-Key", "` + cfg.APIKey + `")
      http.DefaultClient.Do(req)
  }

  // Call sendHeartbeat() every ` + fmt.Sprintf("%d", interval) + ` seconds
`)

	fmt.Println()
	ui.PrintMuted("PHP:")
	fmt.Println(`
  function sendHeartbeat() {
      $data = [
          'service_name' => '` + cfg.ServiceName + `',
          'status' => 'healthy',
          'metadata' => [
              'uptime_seconds' => getUptime(),
              'memory_mb' => getMemoryUsage(),
          ],
      ];

      $ch = curl_init('` + strings.Replace(cfg.Endpoint, "/v1/traces", "", 1) + `/v1/health/heartbeat');
      curl_setopt($ch, CURLOPT_POST, true);
      curl_setopt($ch, CURLOPT_POSTFIELDS, json_encode($data));
      curl_setopt($ch, CURLOPT_HTTPHEADER, [
          'Content-Type: application/json',
          'X-API-Key: ` + cfg.APIKey + `'
      ]);
      curl_exec($ch);
      curl_close($ch);
  }

  // Call sendHeartbeat() every ` + fmt.Sprintf("%d", interval) + ` seconds
`)

	fmt.Println()

	// Next steps
	steps := []string{
		"Implement the heartbeat code in your service",
		fmt.Sprintf("Send heartbeats every %d seconds", interval),
		"Alerts will be triggered if 3 consecutive heartbeats are missed",
		"Run 'tracekit health list' to view health check status",
	}
	ui.PrintNextSteps(steps)

	return nil
}
