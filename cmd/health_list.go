package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var healthListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured health checks",
	Long: `List all configured health checks for your service.

Shows:
  - Health check status
  - Check type (push/pull)
  - Uptime percentage
  - Last check time

Example:
  tracekit health list`,
	RunE: runHealthList,
}

func init() {
	healthListCmd.Flags().Bool("dev", false, "")
	healthListCmd.Flags().MarkHidden("dev")
}

func runHealthList(cmd *cobra.Command, args []string) error {
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

	// Fetch health checks from API
	ui.PrintSection("🏥 Health Checks")
	fmt.Println()

	apiURL := strings.Replace(cfg.Endpoint, "/v1/traces", "", 1)
	req, err := http.NewRequest("GET", apiURL+"/api/health-checks", nil)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create request: %v", err))
		return nil
	}

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

	if resp.StatusCode != http.StatusOK {
		errorMsg := "Unknown error"
		if msg, ok := result["error"].(string); ok {
			errorMsg = msg
		}
		ui.PrintError(fmt.Sprintf("Failed to fetch health checks: %s", errorMsg))
		return nil
	}

	// Parse health checks
	healthChecks, ok := result["health_checks"].([]interface{})
	if !ok || len(healthChecks) == 0 {
		ui.PrintWarning("No health checks configured")
		ui.PrintMuted("   Run 'tracekit health setup' to create a health check")
		return nil
	}

	// Display each health check
	for i, hc := range healthChecks {
		healthCheck := hc.(map[string]interface{})

		serviceName := healthCheck["service_name"].(string)
		checkName := healthCheck["check_name"].(string)
		checkType := healthCheck["check_type"].(string)
		status := healthCheck["status"].(string)
		uptimePercentage := healthCheck["uptime_percentage"].(float64)
		enabled := healthCheck["enabled"].(bool)

		// Status icon and color
		var statusIcon string
		switch status {
		case "healthy":
			statusIcon = "✅"
		case "degraded":
			statusIcon = "⚠️"
		case "unhealthy":
			statusIcon = "❌"
		default:
			statusIcon = "❓"
		}

		// Print check header
		fmt.Printf("%s %s / %s\n", statusIcon, serviceName, checkName)
		ui.PrintMuted(fmt.Sprintf("   Type: %s", strings.ToUpper(checkType)))
		ui.PrintMuted(fmt.Sprintf("   Status: %s", status))
		ui.PrintMuted(fmt.Sprintf("   Uptime: %.2f%%", uptimePercentage))

		if !enabled {
			ui.PrintMuted("   Enabled: false")
		}

		// Type-specific details
		if checkType == "pull" {
			if endpointURL, ok := healthCheck["endpoint_url"].(string); ok {
				ui.PrintMuted(fmt.Sprintf("   Endpoint: %s", endpointURL))
			}
			if interval, ok := healthCheck["check_interval_seconds"].(float64); ok {
				ui.PrintMuted(fmt.Sprintf("   Interval: Every %.0f seconds", interval))
			}
		} else if checkType == "push" {
			if interval, ok := healthCheck["heartbeat_interval_seconds"].(float64); ok {
				ui.PrintMuted(fmt.Sprintf("   Expected: Every %.0f seconds", interval))
			}
		}

		// Last check time
		if lastCheckAt, ok := healthCheck["last_check_at"].(string); ok && lastCheckAt != "" {
			parsedTime, err := time.Parse(time.RFC3339, lastCheckAt)
			if err == nil {
				ui.PrintMuted(fmt.Sprintf("   Last check: %s", formatTimeAgo(parsedTime)))
			}
		}

		// Consecutive failures
		if consecutiveFailures, ok := healthCheck["consecutive_failures"].(float64); ok && consecutiveFailures > 0 {
			ui.PrintWarning(fmt.Sprintf("   ⚠️  %d consecutive failures", int(consecutiveFailures)))
		}

		// Add spacing between checks
		if i < len(healthChecks)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	ui.PrintDivider()
	fmt.Println()

	// Summary
	count := len(healthChecks)
	healthyCount := 0
	for _, hc := range healthChecks {
		healthCheck := hc.(map[string]interface{})
		if healthCheck["status"].(string) == "healthy" {
			healthyCount++
		}
	}

	summary := fmt.Sprintf("Total checks: %d\nHealthy: %d\nUnhealthy: %d",
		count, healthyCount, count-healthyCount)

	ui.PrintSummaryBox("📊 Health Check Summary", summary)

	return nil
}

// formatTimeAgo formats a time as a human-readable "time ago" string
func formatTimeAgo(t time.Time) string {
	duration := time.Since(t)

	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		minutes := int(duration.Minutes())
		if minutes == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", minutes)
	} else if duration < 24*time.Hour {
		hours := int(duration.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	} else {
		days := int(duration.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
