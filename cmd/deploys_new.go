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

var deploysNewCmd = &cobra.Command{
	Use:   "new [version]",
	Short: "Register a new deploy for a release",
	Long: `Register a new deploy for a release to track which versions are
running in which environments.

If no version is provided, auto-detects from package.json version field,
then falls back to the latest git tag.

Examples:
  tracekit deploys new v1.2.3 --env production
  tracekit deploys new --env staging --deployer deploy-bot
  tracekit deploys new --env production --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runDeploysNew,
}

func init() {
	deploysNewCmd.Flags().String("env", "", "Deployment environment (required)")
	deploysNewCmd.MarkFlagRequired("env")
	deploysNewCmd.Flags().String("deployer", "", "Name of the deployer or deploy system")
}

func runDeploysNew(cmd *cobra.Command, args []string) error {
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

	env, _ := cmd.Flags().GetString("env")
	deployer, _ := cmd.Flags().GetString("deployer")

	// Create client
	c := client.NewClient(cfg.GetAPIBase())
	c.APIKey = cfg.APIKey

	// Create deploy
	req := &client.CreateDeployRequest{
		Environment: env,
		Deployer:    deployer,
	}

	deploy, err := c.CreateDeploy(ver, req)
	if err != nil {
		return fmt.Errorf("failed to register deploy: %w", err)
	}

	// Output
	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		out, err := json.MarshalIndent(deploy, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal response: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	deployInfo := fmt.Sprintf("Deployed %s to %s", ver, env)
	if deployer != "" {
		deployInfo += fmt.Sprintf(" by %s", deployer)
	}
	deployInfo += fmt.Sprintf(" at %s", deploy.DeployedAt)
	ui.PrintSuccess(deployInfo)

	return nil
}
