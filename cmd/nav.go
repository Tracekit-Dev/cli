package cmd

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// navItem represents a navigable CLI command
type navItem struct {
	key   string // short key for quick select (d, t, l, s, a, i, q)
	name  string // display name
	desc  string // short description
	cmd   string // cobra command name to launch
}

var navItems = []navItem{
	{key: "d", name: "Dashboard", desc: "Live health metrics", cmd: "dashboard"},
	{key: "t", name: "Traces", desc: "Browse and filter traces", cmd: "traces"},
	{key: "l", name: "Logs", desc: "Stream live traces", cmd: "logs"},
	{key: "s", name: "Services", desc: "Service health and metrics", cmd: "services"},
	{key: "a", name: "Alerts", desc: "Manage alert rules", cmd: "alerts"},
	{key: "i", name: "Incidents", desc: "Triage inbox", cmd: "incidents"},
}

// NavSwitchMsg is returned when the user picks a command to switch to.
// The runner function should detect this and launch the new command.
type NavSwitchMsg struct {
	Command string
}

// navModel is an embeddable overlay for command switching.
type navModel struct {
	active  bool
	cursor  int
	current string // current command name, to highlight/skip
}

func newNavModel(currentCmd string) navModel {
	return navModel{current: currentCmd}
}

// HandleNavKey processes a key event for the nav overlay.
// Returns updated navModel, whether the event was consumed, and an optional tea.Cmd.
func (n navModel) HandleNavKey(msg tea.KeyMsg) (navModel, bool, tea.Cmd) {
	if !n.active {
		// Trigger on colon
		if msg.String() == ":" {
			n.active = true
			// Start cursor on first item that isn't current
			n.cursor = 0
			for i, item := range navItems {
				if item.cmd != n.current {
					n.cursor = i
					break
				}
			}
			return n, true, nil
		}
		return n, false, nil
	}

	// Nav is active -- consume all keys
	switch msg.String() {
	case "esc", ":":
		n.active = false
		return n, true, nil
	case "j", "down":
		n.cursor++
		if n.cursor >= len(navItems) {
			n.cursor = 0
		}
		// Skip current command
		if navItems[n.cursor].cmd == n.current {
			n.cursor++
			if n.cursor >= len(navItems) {
				n.cursor = 0
			}
		}
		return n, true, nil
	case "k", "up":
		n.cursor--
		if n.cursor < 0 {
			n.cursor = len(navItems) - 1
		}
		if navItems[n.cursor].cmd == n.current {
			n.cursor--
			if n.cursor < 0 {
				n.cursor = len(navItems) - 1
			}
		}
		return n, true, nil
	case "enter":
		selected := navItems[n.cursor]
		n.active = false
		return n, true, func() tea.Msg {
			return NavSwitchMsg{Command: selected.cmd}
		}
	default:
		// Quick-select by letter key
		key := msg.String()
		for _, item := range navItems {
			if item.key == key && item.cmd != n.current {
				n.active = false
				return n, true, func() tea.Msg {
					return NavSwitchMsg{Command: item.cmd}
				}
			}
		}
		return n, true, nil
	}
}

// ViewNav renders the nav overlay. Returns empty string if not active.
func (n navModel) ViewNav(width int) string {
	if !n.active {
		return ""
	}

	var b strings.Builder
	brand := lipgloss.Color("#6366f1")
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	title := lipgloss.NewStyle().Foreground(brand).Bold(true).Render("  Switch to...")
	b.WriteString(title + "\n\n")

	for i, item := range navItems {
		if item.cmd == n.current {
			continue // skip current
		}

		prefix := "  "
		nameStyle := text
		descStyle := dim
		keyStyle := lipgloss.NewStyle().Foreground(brand).Bold(true)

		if i == n.cursor {
			prefix = lipgloss.NewStyle().Foreground(brand).Render("> ")
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true)
		}

		line := fmt.Sprintf("%s%s %s  %s",
			prefix,
			keyStyle.Render(item.key),
			nameStyle.Render(item.name),
			descStyle.Render(item.desc),
		)
		b.WriteString(line + "\n")
	}

	b.WriteString("\n" + dim.Render("  j/k navigate  enter select  esc close"))

	// Wrap in a box
	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(brand).
		Padding(1, 2).
		Width(48).
		Render(b.String())

	return overlay
}

// NavHint returns a short hint string for the footer (e.g., ": switch")
func NavHint() string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(": switch")
}

// RunNavTarget launches the selected command. Call this after the TUI exits with NavSwitchMsg.
func RunNavTarget(command string) error {
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == command {
			if cmd.RunE != nil {
				return cmd.RunE(cmd, []string{})
			}
			if cmd.Run != nil {
				cmd.Run(cmd, []string{})
				return nil
			}
			return fmt.Errorf("command %s has no runner", command)
		}
	}
	return fmt.Errorf("unknown command: %s", command)
}
