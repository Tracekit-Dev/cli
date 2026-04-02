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
  tracekit profile                        List all profiles
  tracekit profile use <url-or-tag>       Switch active profile
  tracekit profile tag <url> <name>       Tag a profile for quick switching
  tracekit profile remove <url-or-tag>    Remove a saved profile`,
	RunE: runProfileList,
}

var profileUseCmd = &cobra.Command{
	Use:   "use <url-or-tag>",
	Short: "Switch the active profile",
	Long: `Set the active profile by URL or tag name.
All commands will use this profile's credentials by default.

Example:
  tracekit profile use prod
  tracekit profile use local
  tracekit profile use http://localhost:8081`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileUse,
}

var profileTagCmd = &cobra.Command{
	Use:   "tag <url-or-tag> <name>",
	Short: "Tag a profile with a short name",
	Long: `Assign a tag to a profile so you can switch with a short name
instead of typing the full URL.

Example:
  tracekit profile tag https://app.tracekit.dev prod
  tracekit profile tag http://localhost:8081 local
  tracekit profile use prod`,
	Args: cobra.ExactArgs(2),
	RunE: runProfileTag,
}

var profileRemoveCmd = &cobra.Command{
	Use:   "remove <url-or-tag>",
	Short: "Remove a saved profile",
	Long: `Delete a saved profile and its stored credentials.

Example:
  tracekit profile remove local
  tracekit profile remove http://localhost:8081`,
	Args: cobra.ExactArgs(1),
	RunE: runProfileRemove,
}

func init() {
	rootCmd.AddCommand(profileCmd)
	profileCmd.AddCommand(profileUseCmd)
	profileCmd.AddCommand(profileTagCmd)
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
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#10b981"))
	tagStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b5cf6"))

	fmt.Println()
	fmt.Println("  " + header.Render("Saved Profiles"))
	fmt.Println()

	for url, profile := range profiles {
		marker := "  "
		urlRender := text
		if url == active {
			marker = green.Render("> ")
			urlRender = activeStyle
		}

		line := fmt.Sprintf("  %s%s", marker, urlRender.Render(url))
		if profile.Tag != "" {
			line += "  " + tagStyle.Render("["+profile.Tag+"]")
		}
		if url == active {
			line += "  " + activeStyle.Render("(active)")
		}
		fmt.Println(line)

		fmt.Printf("      %s  %s\n", dim.Render("API Key:"), text.Render(utils.MaskAPIKey(profile.APIKey)))
		if profile.ServiceName != "" {
			fmt.Printf("      %s  %s\n", dim.Render("Service:"), text.Render(profile.ServiceName))
		}
		fmt.Println()
	}

	fmt.Println("  " + dim.Render("Switch with: tracekit profile use <url-or-tag>"))
	fmt.Println("  " + dim.Render("Tag with:    tracekit profile tag <url> <name>"))
	fmt.Println()

	return nil
}

func runProfileUse(cmd *cobra.Command, args []string) error {
	target := args[0]

	if err := config.SetActiveProfile(target); err != nil {
		return err
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	fmt.Println()
	fmt.Println("  " + green.Render("Switched active profile to") + " " + target)
	fmt.Println()

	return nil
}

func runProfileTag(cmd *cobra.Command, args []string) error {
	target := args[0]
	tag := args[1]

	if err := config.TagProfile(target, tag); err != nil {
		return err
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	purple := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b5cf6"))
	fmt.Println()
	fmt.Println("  " + green.Render("Tagged") + " " + target + " " + green.Render("as") + " " + purple.Render("["+tag+"]"))
	fmt.Println()

	return nil
}

func runProfileRemove(cmd *cobra.Command, args []string) error {
	target := args[0]

	if err := config.RemoveProfile(target); err != nil {
		return err
	}

	green := lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Bold(true)
	fmt.Println()
	fmt.Println("  " + green.Render("Removed profile") + " " + target)
	fmt.Println()

	return nil
}
