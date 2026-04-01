package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Live dashboard in your terminal",
	Long: `Display a live-updating dashboard showing health score, services,
alerts, anomalies, and error hotspots. Refreshes every 30 seconds.

Press 'q' or Ctrl+C to exit, 'r' to refresh.

Examples:
  tracekit dashboard
  tracekit dashboard --api-key tk_your_key
  tracekit dashboard --api-key tk_your_key --url http://localhost:8081`,
	RunE: runDashboard,
}

func init() {
	rootCmd.AddCommand(dashboardCmd)
	dashboardCmd.Flags().String("api-key", "", "TraceKit API key (overrides .env)")
	dashboardCmd.Flags().String("url", "", "API base URL (default: https://app.tracekit.dev)")
	dashboardCmd.Flags().Bool("dev", false, "")
	dashboardCmd.Flags().MarkHidden("dev")
}

// -- Colors --
var (
	cBrand   = lipgloss.Color("#6366f1")
	cSuccess = lipgloss.Color("#10b981")
	cWarning = lipgloss.Color("#f59e0b")
	cDanger  = lipgloss.Color("#ef4444")
	cMuted   = lipgloss.Color("#6b7280")
	cText    = lipgloss.Color("#e5e7eb")
	cDim     = lipgloss.Color("#4b5563")
)

// -- Bubbletea model --

type dashModel struct {
	client    *client.Client
	data      *client.DashboardData
	err       error
	width     int
	lastFetch time.Time
	quitting  bool
}

type tickMsg time.Time
type dataMsg struct {
	data *client.DashboardData
	err  error
}

func fetchDashData(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		data, err := c.GetDashboard()
		return dataMsg{data: data, err: err}
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m dashModel) Init() tea.Cmd {
	return tea.Batch(fetchDashData(m.client), tickEvery(30*time.Second))
}

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m, fetchDashData(m.client)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
	case tickMsg:
		return m, tea.Batch(fetchDashData(m.client), tickEvery(30*time.Second))
	case dataMsg:
		m.lastFetch = time.Now()
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.data = msg.data
			m.err = nil
		}
	}
	return m, nil
}

func (m dashModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}
	if w > 110 {
		w = 110
	}

	var b strings.Builder

	// Header
	title := lipgloss.NewStyle().Bold(true).Foreground(cBrand)
	muted := lipgloss.NewStyle().Foreground(cMuted)

	b.WriteString("\n")
	b.WriteString("  " + title.Render("TraceKit Dashboard"))

	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString("  " + muted.Render(fmt.Sprintf("updated %s ago", ago)))
	}
	b.WriteString("\n")
	b.WriteString("  " + muted.Render("r: refresh  q: quit"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(fmt.Sprintf("Error: %s", m.err.Error())))
		b.WriteString("\n")
		return b.String()
	}

	if m.data == nil {
		b.WriteString("  " + muted.Render("Loading..."))
		return b.String()
	}

	d := m.data

	// -- Stat Cards Row --
	b.WriteString(renderStatCards(d))
	b.WriteString("\n")

	// -- Active Alerts --
	section := lipgloss.NewStyle().Bold(true).Foreground(cText)
	b.WriteString("  " + section.Render("Active Alerts"))

	if d.Alerts.Count == 0 {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(cSuccess).Render("None"))
		b.WriteString("\n")
	} else {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(fmt.Sprintf("%d firing", d.Alerts.Count)))
		b.WriteString("\n")
		for i, a := range d.Alerts.Items {
			if i >= 5 {
				b.WriteString("  " + muted.Render(fmt.Sprintf("  ... +%d more", d.Alerts.Count-5)))
				b.WriteString("\n")
				break
			}
			sev := renderSeverity(a.Severity)
			b.WriteString(fmt.Sprintf("    %s  %s  %s\n", sev, lipgloss.NewStyle().Foreground(cText).Render(a.Name), muted.Render(a.Duration)))
		}
	}

	// -- Anomalies --
	if d.Anomalies.Unacknowledged > 0 {
		b.WriteString("  " + section.Render("Anomalies"))
		anomalyText := fmt.Sprintf("  %d unacknowledged", d.Anomalies.Unacknowledged)
		if d.Anomalies.Critical > 0 {
			anomalyText += fmt.Sprintf(" (%d critical)", d.Anomalies.Critical)
			b.WriteString(lipgloss.NewStyle().Foreground(cDanger).Render(anomalyText))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cWarning).Render(anomalyText))
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// -- Services Table --
	if len(d.Services) > 0 {
		b.WriteString("  " + section.Render("Services (24h)"))
		b.WriteString("\n")

		hdr := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
		b.WriteString(hdr.Render(fmt.Sprintf("    %-24s %8s %8s %8s %8s", "SERVICE", "TRACES", "ERRORS", "ERR%", "AVG")))
		b.WriteString("\n")

		for i, svc := range d.Services {
			if i >= 10 {
				b.WriteString("    " + muted.Render(fmt.Sprintf("... +%d more services", len(d.Services)-10)))
				b.WriteString("\n")
				break
			}

			name := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%-24s", trunc(svc.Name, 24)))
			traces := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%8d", svc.Traces))
			errors := lipgloss.NewStyle().Foreground(errColor(svc.ErrorRate)).Render(fmt.Sprintf("%8d", svc.Errors))
			errRate := lipgloss.NewStyle().Foreground(errColor(svc.ErrorRate)).Render(fmt.Sprintf("%7.1f%%", svc.ErrorRate))
			avg := lipgloss.NewStyle().Foreground(latColor(svc.AvgResponse)).Render(fmt.Sprintf("%8s", fmtMs(float64(svc.AvgResponse))))

			b.WriteString(fmt.Sprintf("    %s %s %s %s %s\n", name, traces, errors, errRate, avg))
		}
		b.WriteString("\n")
	}

	// -- Error Hotspots --
	if len(d.ErrorHotspots) > 0 {
		b.WriteString("  " + section.Render("Error Hotspots (24h)"))
		b.WriteString("\n")

		for _, h := range d.ErrorHotspots {
			name := fmt.Sprintf("%s / %s", h.Service, h.Operation)
			if len(name) > 45 {
				name = name[:44] + "\u2026"
			}
			b.WriteString(fmt.Sprintf("    %s  %s  %s\n",
				lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%-45s", name)),
				lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(fmt.Sprintf("%d errors", h.Errors)),
				muted.Render(fmt.Sprintf("%.0f%% of %d", h.ErrorRate, h.Total)),
			))
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString("  " + muted.Render("Open app.tracekit.dev for full dashboard"))
	b.WriteString("\n")

	return b.String()
}

func renderStatCards(d *client.DashboardData) string {
	s := d.Stats

	healthColor := cSuccess
	if s.HealthScore < 80 {
		healthColor = cWarning
	}
	if s.HealthScore < 60 {
		healthColor = cDanger
	}

	bold := func(c lipgloss.Color, text string) string {
		return lipgloss.NewStyle().Foreground(c).Bold(true).Render(text)
	}
	label := func(text string) string {
		return lipgloss.NewStyle().Foreground(cMuted).Render(text)
	}

	// Build stat line
	parts := []string{
		fmt.Sprintf("%s %s", label("Health:"), bold(healthColor, fmt.Sprintf("%.0f%%", s.HealthScore))),
		fmt.Sprintf("%s %s", label("Services:"), bold(cText, fmt.Sprintf("%d", s.Services))),
		fmt.Sprintf("%s %s", label("Traces:"), bold(cText, fmt.Sprintf("%d", s.TotalTraces))),
		fmt.Sprintf("%s %s", label("Errors:"), bold(errColor(s.ErrorRate), fmt.Sprintf("%d", s.Errors24h))),
		fmt.Sprintf("%s %s", label("Avg:"), bold(latColor(s.AvgResponse), fmtMs(float64(s.AvgResponse)))),
	}

	line := "  " + strings.Join(parts, "  "+lipgloss.NewStyle().Foreground(cDim).Render("|")+"  ")

	// Delta line
	if s.Deltas.HasPrevious {
		deltaLine := "  "
		if s.Deltas.Health != 0 {
			deltaLine += deltaStr(s.Deltas.Health, false, "pts") + "  "
		}
		deltaLine += "          " // spacing for services/traces
		if s.Deltas.Errors != 0 {
			deltaLine += "         " + deltaStr(s.Deltas.Errors, true, "%") + "  "
		}
		if s.Deltas.AvgResponse != 0 {
			deltaLine += deltaStr(s.Deltas.AvgResponse, true, "%")
		}
		return line + "\n" + lipgloss.NewStyle().Foreground(cMuted).Render(deltaLine) + "\n"
	}

	return line + "\n"
}

func deltaStr(val float64, invert bool, unit string) string {
	arrow := "\u2191"
	color := cSuccess
	if val < 0 {
		arrow = "\u2193"
	}
	if (val > 0 && invert) || (val < 0 && !invert) {
		color = cDanger
	}
	return lipgloss.NewStyle().Foreground(color).Render(fmt.Sprintf("%s%.0f%s vs yesterday", arrow, val, unit))
}

func renderSeverity(s string) string {
	switch s {
	case "critical":
		return lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("CRIT")
	case "warning":
		return lipgloss.NewStyle().Foreground(cWarning).Bold(true).Render("WARN")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render("INFO")
	}
}

func errColor(rate float64) lipgloss.Color {
	if rate > 5 {
		return cDanger
	}
	if rate > 1 {
		return cWarning
	}
	return cSuccess
}

func latColor(ms int) lipgloss.Color {
	if ms > 1000 {
		return cDanger
	}
	if ms > 500 {
		return cWarning
	}
	return cSuccess
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

func fmtMs(ms float64) string {
	if ms < 1 {
		return "0ms"
	}
	if ms < 1000 {
		return fmt.Sprintf("%.0fms", ms)
	}
	return fmt.Sprintf("%.2fs", ms/1000)
}

func runDashboard(cmd *cobra.Command, args []string) error {
	apiKey, _ := cmd.Flags().GetString("api-key")
	customURL, _ := cmd.Flags().GetString("url")
	isDev, _ := cmd.Flags().GetBool("dev")

	if apiKey == "" {
		cfg, err := config.Read()
		if err != nil {
			return fmt.Errorf("no API key provided. Use --api-key or run 'tracekit init' first")
		}
		apiKey = cfg.APIKey
	}

	baseURL := client.DefaultBaseURL
	if customURL != "" {
		baseURL = customURL
	} else if isDev {
		baseURL = client.DevBaseURL
	}

	c := client.NewClient(baseURL)
	c.APIKey = apiKey

	p := tea.NewProgram(dashModel{client: c}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dashboard error: %w", err)
	}
	return nil
}
