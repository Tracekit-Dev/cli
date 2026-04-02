package cmd

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

// Version, CommitHash, and BuildDate are set via ldflags at build time
var Version = "dev"
var CommitHash = "unknown"
var BuildDate = "unknown"

// EnvFlag holds the --env flag value for custom config file path
var EnvFlag string

var rootCmd = &cobra.Command{
	Use:   "tracekit",
	Short: "TraceKit CLI - Zero-friction APM setup",
	Long: `TraceKit CLI enables single-command account creation, framework detection,
and SDK installation for application monitoring.

Examples:
  tracekit init              Initialize TraceKit in your project
  tracekit login             Login to existing account
  tracekit status            Show configuration and usage
  tracekit upgrade           Upgrade your subscription plan
  tracekit update            Update the CLI to the latest version`,
	Version: Version,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// HandleUnauthorized checks if an error is a 401 and launches the login flow.
// Accepts the base URL from the original command so login targets the same server.
// Returns true if the error was handled (login was launched).
func HandleUnauthorized(err error, baseURL string) bool {
	if err != nil && errors.Is(err, client.ErrUnauthorized) {
		fmt.Println("\nYour API key is invalid or expired. Please log in again.\n")
		loginCmd := findLoginCmd()
		if loginCmd != nil && loginCmd.RunE != nil {
			if baseURL != "" && baseURL != client.DefaultBaseURL {
				_ = loginCmd.Flags().Set("api-url", baseURL)
			}
			_ = loginCmd.RunE(loginCmd, []string{})
		}
		return true
	}
	return false
}

func findLoginCmd() *cobra.Command {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "login" {
			return cmd
		}
	}
	return nil
}

// NewAuthenticatedClient creates an API client from stored credentials.
// Enforces login -- returns an error if user_id is missing.
// Respects --api-key and --url flag overrides from the command.
func NewAuthenticatedClient(cmd *cobra.Command) (*client.Client, error) {
	cfg, err := config.RequireAuth(EnvFlag)
	if err != nil {
		return nil, err
	}

	baseURL := cfg.GetAPIBase()

	// Allow --url override if the command has it
	if cmd.Flags().Lookup("url") != nil {
		if customURL, _ := cmd.Flags().GetString("url"); customURL != "" {
			baseURL = customURL
		}
	}

	// Allow --dev override if the command has it
	if cmd.Flags().Lookup("dev") != nil {
		if isDev, _ := cmd.Flags().GetBool("dev"); isDev {
			baseURL = client.DevBaseURL
		}
	}

	c := client.NewClient(baseURL)
	c.APIKey = cfg.APIKey
	c.UserID = cfg.UserID

	return c, nil
}

func init() {
	// Custom version template with commit hash and build date
	rootCmd.SetVersionTemplate(fmt.Sprintf("tracekit v%s (%s, %s)\n", Version, CommitHash, BuildDate))

	// Global --env flag for custom config file path
	rootCmd.PersistentFlags().StringVar(&EnvFlag, "env", "", "path to env/config file (default: .env, fallback: ~/.tracekitconfig)")
}
