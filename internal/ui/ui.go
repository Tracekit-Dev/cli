package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Premium color palette
	brandColor     = lipgloss.Color("#6366f1") // Indigo - primary brand
	accentColor    = lipgloss.Color("#8b5cf6") // Purple - accent
	successColor   = lipgloss.Color("#10b981") // Emerald - success
	warningColor   = lipgloss.Color("#f59e0b") // Amber - warning
	dangerColor    = lipgloss.Color("#ef4444") // Red - error
	mutedColor     = lipgloss.Color("#9ca3af") // Gray - muted
	subtleColor    = lipgloss.Color("#374151") // Dark gray - subtle
	highlightColor = lipgloss.Color("#fbbf24") // Yellow - highlight

	// Gradient colors
	gradientStart = lipgloss.Color("#6366f1")
	gradientEnd   = lipgloss.Color("#8b5cf6")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(brandColor).
			Align(lipgloss.Center).
			Padding(1, 2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(brandColor).
			Padding(1, 3).
			Width(65)

	fancyBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(brandColor).
			Padding(1, 3).
			Width(65)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(dangerColor).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	subtleStyle = lipgloss.NewStyle().
			Foreground(subtleColor).
			Italic(true)

	promptStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true)

	highlightStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Bold(true)

	bulletStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			MarginLeft(2)

	sectionStyle = lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true).
			Padding(0, 1)
)

// PrintBanner prints the welcome banner
func PrintBanner() {
	// ASCII art logo
	logo := lipgloss.NewStyle().
		Foreground(brandColor).
		Bold(true).
		Render(`
  ████████╗██████╗  █████╗  ██████╗███████╗██╗  ██╗██╗████████╗
  ╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██╔════╝██║ ██╔╝██║╚══██╔══╝
     ██║   ██████╔╝███████║██║     █████╗  █████╔╝ ██║   ██║
     ██║   ██╔══██╗██╔══██║██║     ██╔══╝  ██╔═██╗ ██║   ██║
     ██║   ██║  ██║██║  ██║╚██████╗███████╗██║  ██╗██║   ██║
     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝
`)

	tagline := lipgloss.NewStyle().
		Foreground(accentColor).
		Italic(true).
		Align(lipgloss.Center).
		Width(65).
		Render("Zero-friction APM for modern applications")

	features := lipgloss.NewStyle().
		Foreground(mutedColor).
		MarginTop(1).
		Render(`
  ⚡ Auto-detect framework    🔑 Instant account setup
  📊 200k free traces/month   🎨 Beautiful dashboards
`)

	fmt.Println(logo)
	fmt.Println(tagline)
	fmt.Println(features)
	fmt.Println()
}

// PrintSection prints a premium section header
func PrintSection(title string) {
	// Create a more premium section header
	header := lipgloss.NewStyle().
		Foreground(brandColor).
		Bold(true).
		Padding(0, 2).
		Render(title)

	bar := lipgloss.NewStyle().
		Foreground(accentColor).
		Render(strings.Repeat("━", 65))

	fmt.Println()
	fmt.Println(bar)
	fmt.Println(header)
	fmt.Println(bar)
	fmt.Println()
}

// PrintSuccess prints a success message
func PrintSuccess(msg string) {
	icon := lipgloss.NewStyle().Foreground(successColor).Render("✓")
	fmt.Printf("%s %s\n", icon, successStyle.Render(msg))
}

// PrintError prints an error message
func PrintError(msg string) {
	icon := lipgloss.NewStyle().Foreground(dangerColor).Render("✗")
	fmt.Printf("%s %s\n", icon, errorStyle.Render(msg))
}

// PrintWarning prints a warning message
func PrintWarning(msg string) {
	icon := lipgloss.NewStyle().Foreground(warningColor).Render("⚠")
	fmt.Printf("%s %s\n", icon, warningStyle.Render(msg))
}

// PrintInfo prints an info message
func PrintInfo(msg string) {
	icon := lipgloss.NewStyle().Foreground(brandColor).Render("●")
	text := lipgloss.NewStyle().Foreground(brandColor).Render(msg)
	fmt.Printf("%s %s\n", icon, text)
}

// PrintMuted prints a muted message
func PrintMuted(msg string) {
	fmt.Println(mutedStyle.Render(msg))
}

// PrintSubtle prints a subtle/secondary message
func PrintSubtle(msg string) {
	fmt.Println(subtleStyle.Render(msg))
}

// PrintHighlight prints a highlighted message
func PrintHighlight(msg string) {
	fmt.Println(highlightStyle.Render(msg))
}

// PrintPrompt prints a premium prompt for user input
func PrintPrompt(msg string) {
	prompt := lipgloss.NewStyle().
		Foreground(brandColor).
		Bold(true).
		Render("▸ " + msg)
	fmt.Print(prompt + " ")
}

// PrintBullet prints a bulleted item
func PrintBullet(msg string) {
	bullet := lipgloss.NewStyle().Foreground(brandColor).Render("  •")
	fmt.Printf("%s %s\n", bullet, msg)
}

// PrintSummaryBox prints a premium boxed summary
func PrintSummaryBox(title, content string) {
	titleBar := lipgloss.NewStyle().
		Foreground(successColor).
		Bold(true).
		Padding(0, 1).
		Render(title)

	contentStyle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Padding(1, 0)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(successColor).
		Padding(2, 4).
		Width(65).
		Render(fmt.Sprintf("%s\n\n%s", titleBar, contentStyle.Render(content)))

	fmt.Println(box)
}

// PrintKeyValue prints a key-value pair
func PrintKeyValue(key, value string) {
	keyStyle := lipgloss.NewStyle().
		Foreground(mutedColor).
		Width(20)
	valueStyle := lipgloss.NewStyle().
		Foreground(brandColor).
		Bold(true)
	fmt.Printf("%s %s\n", keyStyle.Render(key+":"), valueStyle.Render(value))
}

// PrintDivider prints a horizontal divider
func PrintDivider() {
	divider := lipgloss.NewStyle().
		Foreground(subtleColor).
		Render(strings.Repeat("─", 65))
	fmt.Println(divider)
}

// PrintNextSteps prints the next steps section
func PrintNextSteps(steps []string) {
	PrintSection("📋 Next Steps")

	for i, step := range steps {
		number := lipgloss.NewStyle().
			Foreground(brandColor).
			Bold(true).
			Render(fmt.Sprintf("  %d.", i+1))

		fmt.Printf("%s %s\n", number, step)
	}
	fmt.Println()
}

// PrintSpinner shows a loading spinner with a message
func PrintSpinner(msg string) {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for i := 0; i < 10; i++ {
		frame := lipgloss.NewStyle().
			Foreground(brandColor).
			Render(frames[i%len(frames)])
		fmt.Printf("\r%s %s", frame, msg)
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Print("\r" + strings.Repeat(" ", len(msg)+10) + "\r")
}

// PrintProgress shows a progress indicator
func PrintProgress(current, total int, msg string) {
	percent := float64(current) / float64(total) * 100
	filled := int(percent / 5)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 20-filled)

	progressBar := lipgloss.NewStyle().
		Foreground(brandColor).
		Render(bar)

	percentText := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true).
		Render(fmt.Sprintf("%3.0f%%", percent))

	fmt.Printf("\r%s %s %s", progressBar, percentText, msg)
	if current == total {
		fmt.Println()
	}
}
