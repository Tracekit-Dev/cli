package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

const CallbackPort = 17834

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade your TraceKit subscription plan",
	Long: `Upgrade your TraceKit subscription to get higher trace limits and more features.

This command will:
  1. Generate a secure token for browser authentication
  2. Open your browser to the upgrade page
  3. Wait for you to complete the Stripe checkout
  4. Confirm your upgrade and show updated plan details

The upgrade process is secure and uses Stripe for payment processing.

Example:
  tracekit upgrade`,
	RunE: runUpgrade,
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
	upgradeCmd.Flags().String("api-url", "", "API base URL (default: https://api.tracekit.dev)")
	upgradeCmd.Flags().Bool("dev", false, "")
	upgradeCmd.Flags().MarkHidden("dev")
}

type UpgradeTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	ExpiresIn int    `json:"expires_in"`
}

type SubscriptionResponse struct {
	Plan   string `json:"plan"`
	Status string `json:"status"`
	Usage  struct {
		TracesUsed int64   `json:"traces_used"`
		TraceLimit int64   `json:"trace_limit"`
		Percentage float64 `json:"percentage"`
	} `json:"usage"`
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	// Print banner
	ui.PrintBanner()
	fmt.Println()

	// Step 1: Load config and verify authentication
	ui.PrintSection("🚀 Subscription Upgrade")
	fmt.Println()

	cfg, err := config.Read()
	if err != nil || cfg.APIKey == "" {
		ui.PrintError("You must be logged in to upgrade")
		fmt.Println()
		ui.PrintMuted("Please run: tracekit login")
		return fmt.Errorf("not authenticated")
	}

	// Determine API URL
	apiURL, _ := cmd.Flags().GetString("api-url")
	useDev, _ := cmd.Flags().GetBool("dev")
	if useDev {
		apiURL = client.DevBaseURL
		ui.PrintInfo("Using development API: " + apiURL)
		fmt.Println()
	} else if apiURL == "" {
		apiURL = client.DefaultBaseURL
	}

	// Create API client with auth
	apiClient := client.NewClient(apiURL)
	apiClient.APIKey = cfg.APIKey

	// Step 2: Get current subscription status
	ui.PrintInfo("Fetching current subscription...")
	subscription, err := getSubscription(apiClient)
	if err != nil {
		return fmt.Errorf("failed to fetch subscription: %w", err)
	}

	fmt.Println()
	ui.PrintSuccess(fmt.Sprintf("Current Plan: %s", subscription.Plan))
	if subscription.Usage.TraceLimit > 0 {
		ui.PrintMuted(fmt.Sprintf("   Usage: %d / %d traces (%.1f%%)",
			subscription.Usage.TracesUsed,
			subscription.Usage.TraceLimit,
			subscription.Usage.Percentage))
	}
	fmt.Println()

	// Check if already on paid plan
	if subscription.Plan != "free" && subscription.Plan != "hacker" {
		ui.PrintWarning(fmt.Sprintf("You're already on the %s plan", subscription.Plan))
		fmt.Println()
		ui.PrintMuted("To change plans, please contact support at support@tracekit.dev")
		return nil
	}

	// Step 3: Generate upgrade token
	ui.PrintSection("🔐 Generating secure token...")
	fmt.Println()

	tokenResp, err := createUpgradeToken(apiClient)
	if err != nil {
		return fmt.Errorf("failed to create upgrade token: %w", err)
	}

	ui.PrintSuccess("Token generated successfully")
	fmt.Println()

	// Step 4: Start local callback server
	ui.PrintSection("📡 Starting local callback server...")
	fmt.Println()

	resultChan := make(chan UpgradeResult, 1)
	server := startCallbackServer(resultChan)
	defer server.Shutdown(context.Background())

	ui.PrintSuccess(fmt.Sprintf("Listening on http://localhost:%d", CallbackPort))
	fmt.Println()

	// Step 5: Open browser
	ui.PrintSection("🌐 Opening upgrade page in browser...")
	fmt.Println()

	appURL := "https://app.tracekit.dev"
	if useDev {
		appURL = "http://localhost:8081"
	}

	upgradeURL := fmt.Sprintf("%s/upgrade?token=%s&source=cli&callback_port=%d",
		appURL, tokenResp.Token, CallbackPort)

	// Always print the URL (clickable in most terminals)
	fmt.Println(upgradeURL)
	fmt.Println()

	if err := openBrowser(upgradeURL); err != nil {
		ui.PrintWarning("Failed to open browser automatically")
		fmt.Println()
		ui.PrintPrompt("Click the URL above or copy it to your browser")
	} else {
		ui.PrintSuccess("Browser opened - please complete the upgrade in your browser")
	}

	fmt.Println()
	ui.PrintMuted("Waiting for upgrade to complete...")
	fmt.Println()

	// Step 6: Wait for callback or timeout (2 minutes)
	select {
	case result := <-resultChan:
		// Success - browser callback received
		ui.PrintDivider()
		fmt.Println()
		ui.PrintSuccess(fmt.Sprintf("🎉 Upgrade successful! You're now on the %s plan", result.Plan))
		fmt.Println()

		// Fetch updated subscription
		newSub, err := getSubscription(apiClient)
		if err == nil {
			ui.PrintMuted(fmt.Sprintf("   New trace limit: %d traces/month", newSub.Usage.TraceLimit))
		}

		return nil

	case <-time.After(2 * time.Minute):
		// Timeout - fall back to polling
		ui.PrintWarning("Browser callback timed out - checking upgrade status...")
		fmt.Println()
		return pollForUpgrade(apiClient, subscription.Plan, appURL)
	}
}

type UpgradeResult struct {
	Status string
	Plan   string
}

// startCallbackServer starts a local HTTP server to receive upgrade callbacks
func startCallbackServer(resultChan chan<- UpgradeResult) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		status := r.URL.Query().Get("status")
		plan := r.URL.Query().Get("plan")

		if status == "success" {
			resultChan <- UpgradeResult{
				Status: status,
				Plan:   plan,
			}

			// Send success response
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`
				<html>
				<body style="font-family: system-ui; text-align: center; padding: 50px;">
					<h1>✅ Upgrade Confirmed</h1>
					<p>You can close this window and return to the CLI.</p>
				</body>
				</html>
			`))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Callback received"))
		}
	})

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", CallbackPort),
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Silently ignore - this is expected when server shuts down
		}
	}()

	return server
}

// pollForUpgrade polls the API to check if upgrade completed
func pollForUpgrade(apiClient *client.Client, oldPlan, appURL string) error {
	ui.PrintInfo("Polling for upgrade status...")
	fmt.Println()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	timeout := time.After(5 * time.Minute)

	for {
		select {
		case <-ticker.C:
			sub, err := getSubscription(apiClient)
			if err != nil {
				continue // Retry on error
			}

			if sub.Plan != oldPlan {
				ui.PrintSuccess(fmt.Sprintf("🎉 Upgrade detected! You're now on the %s plan", sub.Plan))
				fmt.Println()
				ui.PrintMuted(fmt.Sprintf("   New trace limit: %d traces/month", sub.Usage.TraceLimit))
				return nil
			}

		case <-timeout:
			return fmt.Errorf("upgrade timeout - please check your dashboard at %s", appURL)
		}
	}
}

// createUpgradeToken calls the API to generate a one-time upgrade token
func createUpgradeToken(apiClient *client.Client) (*UpgradeTokenResponse, error) {
	req, err := http.NewRequest("POST", apiClient.BaseURL+"/v1/auth/upgrade-token", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", apiClient.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := apiClient.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp client.ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var tokenResp UpgradeTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// getSubscription fetches current subscription details
func getSubscription(apiClient *client.Client) (*SubscriptionResponse, error) {
	req, err := http.NewRequest("GET", apiClient.BaseURL+"/v1/billing/subscription", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", apiClient.APIKey)

	resp, err := apiClient.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp client.ErrorResponse
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("API error: %s", errResp.Error)
	}

	var sub SubscriptionResponse
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, err
	}

	return &sub, nil
}

// openBrowser opens a URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}
