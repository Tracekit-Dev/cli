package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#2563eb") // blue
	successColor   = lipgloss.Color("#16a34a") // green
	warningColor   = lipgloss.Color("#f59e0b") // amber
	mutedColor     = lipgloss.Color("#6b7280") // gray
	dangerColor    = lipgloss.Color("#dc2626") // red

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Align(lipgloss.Center).
			Padding(1, 2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Width(60)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	promptStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	bulletStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			MarginLeft(2)
)

// PrintBanner prints the welcome banner
func PrintBanner() {
	banner := `
╔════════════════════════════════════════════════════════════╗
║                                                            ║
║              🚀 Welcome to TraceKit CLI                    ║
║                                                            ║
╚════════════════════════════════════════════════════════════╝

Zero-friction APM setup for modern applications.
Get production monitoring in under 60 seconds.

⚡ Features:
  • Automatic framework detection
  • Instant account creation
  • 200k free traces per month
  • Partner revenue sharing
  • Beautiful dashboards

Let's get you set up!
`
	fmt.Println(lipgloss.NewStyle().Foreground(primaryColor).Render(banner))
}

// PrintSection prints a boxed section title
func PrintSection(title string) {
	section := fmt.Sprintf("\n╭─────────────────────────────────────────────────────────────╮\n│  %s\n╰─────────────────────────────────────────────────────────────╯", title)
	fmt.Println(lipgloss.NewStyle().Foreground(primaryColor).Render(section))
}

// PrintSuccess prints a success message
func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render("✅ " + msg))
}

// PrintError prints an error message
func PrintError(msg string) {
	fmt.Println(lipgloss.NewStyle().Foreground(dangerColor).Bold(true).Render("❌ " + msg))
}

// PrintWarning prints a warning message
func PrintWarning(msg string) {
	fmt.Println(warningStyle.Render("⚠️  " + msg))
}

// PrintInfo prints an info message
func PrintInfo(msg string) {
	fmt.Println(lipgloss.NewStyle().Foreground(primaryColor).Render("ℹ️  " + msg))
}

// PrintMuted prints a muted message
func PrintMuted(msg string) {
	fmt.Println(mutedStyle.Render(msg))
}

// PrintPrompt prints a prompt for user input
func PrintPrompt(msg string) {
	fmt.Print(promptStyle.Render("❯ " + msg + " "))
}

// PrintBullet prints a bulleted item
func PrintBullet(msg string) {
	fmt.Println(bulletStyle.Render("• " + msg))
}

// PrintSummaryBox prints a boxed summary
func PrintSummaryBox(title, content string) {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(successColor).
		Padding(1, 2).
		Width(60).
		Render(fmt.Sprintf("%s\n\n%s",
			lipgloss.NewStyle().Bold(true).Render(title),
			content))

	fmt.Println(box)
}

// PrintKeyValue prints a key-value pair
func PrintKeyValue(key, value string) {
	keyStyle := lipgloss.NewStyle().Foreground(mutedColor).Width(20)
	valueStyle := lipgloss.NewStyle().Bold(true)
	fmt.Printf("%s %s\n", keyStyle.Render(key+":"), valueStyle.Render(value))
}

// PrintDivider prints a horizontal divider
func PrintDivider() {
	divider := strings.Repeat("─", 62)
	fmt.Println(mutedStyle.Render(divider))
}

// PrintNextSteps prints the next steps section
func PrintNextSteps(steps []string) {
	fmt.Println()
	PrintSection("📖 Next Steps")
	fmt.Println()

	for i, step := range steps {
		fmt.Printf("  %s %s\n",
			lipgloss.NewStyle().Foreground(primaryColor).Bold(true).Render(fmt.Sprintf("%d.", i+1)),
			step)
	}
	fmt.Println()
}

// Spinner characters for loading animations
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
