package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/utils"
)

var profileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage server profiles",
	Long: `List, switch, and remove saved server profiles.

Each profile stores credentials for a different TraceKit server
(production, staging, local dev, etc.).

Examples:
  tracekit profile                     List all profiles
  tracekit profile use <url>           Switch active profile
  tracekit profile remove <url>        Remove a saved profile`,
	RunE: runProfileList,
}

var profileUseCmd = &cobra.Command{
	Use:   "use <url>",
	Short: "Switch the active profile",
	Long: `Set the active profile to the given server URL.
All commands will use this profile's credentials by default.

Example:
  tracekit profile use http://localhost:8081
  tracekit profile use https://app.tracekit.dev`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileUse,
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <url>",
	Short: "Remove a saved profile",
	Long: `Delete a saved profile and its stored credentials.

Example:
  tracekit profile remove http://localhost:8081`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileRemove,
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileRemoveCmd)
}

func runProfileList(cmd *cobra.Command, args []string) error {
	profiles, active, err := config.ListProfiles()
	if err != nil {
		fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Render(
			"  No profiles found. Run 'tracekit login' to create one."))
		return nil
	}

	header := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#6366f1"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ca3af"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981"))
	activeTag := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10b981"))

	fmt.Println()
	fmt.Println("  " + header.Render("Saved Profiles"))
	fmt.Println()

	for url, profile := range profiles {
		marker := "  "
		urlStyle := text
		if url == active {
			marker = green.Render("> ")
			urlStyle = activeTag
		}

		fmt.Printf("  %s%s", marker, urlStyle.Render(url))
		if url == active {
			fmt.Printf("  %s", activeTag.Render("(active)"))
		}
		fmt.Println()
		fmt.Printf("      %s  %s\n", dim.Render("API Key:"), text.Render(utils.MaskAPIKey(profile.APIKey)))
		if profile.ServiceName != "" {
			fmt.Printf("      %s  %s\n", dim.Render("Service:"), text.Render(profile.ServiceName))
		}
		fmt.Println()
	}

	fmt.Println("  " + dim.Render("Switch with: tracekit profile use <url>"))
	fmt.Println()

	return nil
}

func runProfileUse(cmd *cobra.Command, args []string) error {
	url := args[0]

	if err := config.SetActiveProfile(url); err != nil {
		return err
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	fmt.Println()
	fmt.Println("  " + green.Render("Switched active profile to") + " " + url)
	fmt.Println()

	return nil
}

func runProfileRemove(cmd *cobra.Command, args []string) error {
	url := args[0]

	if err := config.RemoveProfile(url); err != nil {
		return err
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	fmt.Println()
	fmt.Println("  " + green.Render("Removed profile") + " " + url)
	fmt.Println()

	return nil
}
