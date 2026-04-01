package cmd

import (
	"fmt"
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/ui"
)

var dashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Live dashboard in your terminal",
	Long: `Display a live-updating dashboard showing health score, services,
alerts, anomalies, and performance charts. Refreshes every 30 seconds.

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
	cWarning = lipgloss.Color("#d97706")
	cDanger  = lipgloss.Color("#dc2626")
	cMuted   = lipgloss.Color("#6b7280")
	cText    = lipgloss.AdaptiveColor{Light: "#1f2937", Dark: "#e5e7eb"}
	cDim     = lipgloss.AdaptiveColor{Light: "#9ca3af", Dark: "#374151"}
	cBorder  = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#374151"}
	cCyan    = lipgloss.Color("#0891b2")
	cPurple  = lipgloss.Color("#7c3aed")
)

// -- Bubbletea Model --

type dashModel struct {
	client    *client.Client
	data      *client.DashboardData
	err       error
	width     int
	height    int
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

func (m dashModel) Init() tea.Cmd {
	return tea.Batch(fetchDashData(m.client), tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }))
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
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(fetchDashData(m.client), tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }))
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
	if w < 60 {
		w = 80
	}

	var b strings.Builder

	// -- Header --
	headerStyle := lipgloss.NewStyle().
		Foreground(cBrand).
		Bold(true)

	statusStyle := lipgloss.NewStyle().Foreground(cMuted)

	b.WriteString("\n ")
	b.WriteString(headerStyle.Render("TraceKit"))
	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		b.WriteString(statusStyle.Render(fmt.Sprintf("  last %s  ", ago)))
	}
	b.WriteString(statusStyle.Render(" r refresh  q quit"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(cDanger).Padding(0, 1).Render(fmt.Sprintf(" Error: %s", m.err.Error())))
		b.WriteString("\n")
		return b.String()
	}
	if m.data == nil {
		b.WriteString(statusStyle.Render("  Loading..."))
		return b.String()
	}

	d := m.data
	contentWidth := w - 2 // margin

	// -- Row 1: Stat Cards --
	b.WriteString(renderStatRow(d, contentWidth))

	// -- Row 2: Charts (Throughput + Error Rate) --
	if len(d.TimeSeries) > 1 {
		b.WriteString(renderChartRow(d, contentWidth))
	}

	// -- Row 3: Services + Alerts side by side --
	b.WriteString(renderInfoRow(d, contentWidth))

	// -- Row 4: Error Hotspots --
	if len(d.ErrorHotspots) > 0 {
		b.WriteString(renderHotspots(d, contentWidth))
	}

	// Footer
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render("  app.tracekit.dev"))
	b.WriteString("\n")

	return b.String()
}

// -- Stat Cards Row --

func renderStatRow(d *client.DashboardData, w int) string {
	s := d.Stats
	cardW := (w - 4) / 5 // 5 cards with gaps

	makeCard := func(label, value string, valueColor lipgloss.TerminalColor, delta string) string {
		labelStr := lipgloss.NewStyle().Foreground(cMuted).Render(label)
		valueStr := lipgloss.NewStyle().Foreground(valueColor).Bold(true).Render(value)
		content := labelStr + "\n" + valueStr
		if delta != "" {
			content += "\n" + delta
		}
		return lipgloss.NewStyle().
			Width(cardW).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(cBorder).
			Padding(0, 1).
			Render(content)
	}

	healthColor := cSuccess
	if s.HealthScore < 80 {
		healthColor = cWarning
	}
	if s.HealthScore < 60 {
		healthColor = cDanger
	}

	healthDelta := ""
	errDelta := ""
	avgDelta := ""
	if s.Deltas.HasPrevious {
		if s.Deltas.Health != 0 {
			healthDelta = fmtDelta(s.Deltas.Health, false, "pts")
		}
		if s.Deltas.Errors != 0 {
			errDelta = fmtDelta(s.Deltas.Errors, true, "%")
		}
		if s.Deltas.AvgResponse != 0 {
			avgDelta = fmtDelta(s.Deltas.AvgResponse, true, "%")
		}
	}

	cards := lipgloss.JoinHorizontal(lipgloss.Top,
		makeCard("Health Score", fmt.Sprintf("%.0f%%", s.HealthScore), healthColor, healthDelta),
		" ",
		makeCard("Services", fmt.Sprintf("%d", s.Services), cText, "active"),
		" ",
		makeCard("Total Traces", fmt.Sprintf("%d", s.TotalTraces), cText, "all time"),
		" ",
		makeCard("Errors (24h)", fmt.Sprintf("%d", s.Errors24h), getErrColor(s.ErrorRate), errDelta),
		" ",
		makeCard("Avg Response", fmtMs(float64(s.AvgResponse)), getLatColor(s.AvgResponse), avgDelta),
	)

	return " " + cards + "\n\n"
}

// -- Chart Row --

func renderChartRow(d *client.DashboardData, w int) string {
	halfW := (w - 3) / 2
	chartW := halfW - 4 // padding inside border
	chartH := 6

	// Throughput data
	reqData := make([]float64, len(d.TimeSeries))
	maxReq := 0
	for i, ts := range d.TimeSeries {
		reqData[i] = float64(ts.Requests)
		if ts.Requests > maxReq {
			maxReq = ts.Requests
		}
	}

	latestTime := ""
	if len(d.TimeSeries) > 0 {
		latestTime = d.TimeSeries[len(d.TimeSeries)-1].Time
	}

	reqChart := ui.RenderSparkline(reqData, chartW, chartH, cCyan)
	reqTitle := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Requests per Hour")
	reqStats := lipgloss.NewStyle().Foreground(cMuted).Render(
		fmt.Sprintf("Peak: %d  Latest: %s", maxReq, latestTime))

	reqPanel := lipgloss.NewStyle().
		Width(halfW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(reqTitle + "\n" + reqChart + "\n" + reqStats)

	// Error rate data
	errData := make([]float64, len(d.TimeSeries))
	maxErr := 0.0
	for i, ts := range d.TimeSeries {
		rate := 0.0
		if ts.Requests > 0 {
			rate = float64(ts.Errors) / float64(ts.Requests) * 100
		}
		errData[i] = rate
		if rate > maxErr {
			maxErr = rate
		}
	}

	errChart := ui.RenderSparklineLine(errData, chartW, chartH, cDanger)
	errTitle := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Error Rate %")
	errStats := lipgloss.NewStyle().Foreground(cMuted).Render(
		fmt.Sprintf("Current: %.1f%%  Peak: %.1f%%", d.Stats.ErrorRate, maxErr))

	errPanel := lipgloss.NewStyle().
		Width(halfW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(errTitle + "\n" + errChart + "\n" + errStats)

	return " " + lipgloss.JoinHorizontal(lipgloss.Top, reqPanel, " ", errPanel) + "\n\n"
}

// -- Info Row: Services + Alerts --

func renderInfoRow(d *client.DashboardData, w int) string {
	halfW := (w - 3) / 2

	// Services panel
	var svcContent strings.Builder
	if len(d.Services) == 0 {
		svcContent.WriteString(lipgloss.NewStyle().Foreground(cMuted).Render("No services detected"))
	} else {
		hdr := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
		svcContent.WriteString(hdr.Render(fmt.Sprintf("%-20s %6s %6s %6s", "NAME", "REQS", "ERR%", "AVG")))
		svcContent.WriteString("\n")

		limit := len(d.Services)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			svc := d.Services[i]
			name := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%-20s", trunc(svc.Name, 20)))
			reqs := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%6d", svc.Traces))
			errR := lipgloss.NewStyle().Foreground(getErrColor(svc.ErrorRate)).Render(fmt.Sprintf("%5.1f%%", svc.ErrorRate))
			avg := lipgloss.NewStyle().Foreground(getLatColor(svc.AvgResponse)).Render(fmt.Sprintf("%6s", fmtMs(float64(svc.AvgResponse))))
			svcContent.WriteString(fmt.Sprintf("%s %s %s %s\n", name, reqs, errR, avg))
		}
		if len(d.Services) > 8 {
			svcContent.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(fmt.Sprintf("+%d more", len(d.Services)-8)))
		}
	}

	svcTitle := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Services (24h)")
	svcPanel := lipgloss.NewStyle().
		Width(halfW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(svcTitle + "\n" + svcContent.String())

	// Alerts + Anomalies panel
	var alertContent strings.Builder

	// Active alerts
	if d.Alerts.Count == 0 {
		alertContent.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Render("No active alerts") + "\n")
	} else {
		alertContent.WriteString(lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(
			fmt.Sprintf("%d firing", d.Alerts.Count)) + "\n")
		limit := d.Alerts.Count
		if limit > 4 {
			limit = 4
		}
		for i := 0; i < limit; i++ {
			a := d.Alerts.Items[i]
			sev := fmtSeverity(a.Severity)
			alertContent.WriteString(fmt.Sprintf(" %s %s %s\n",
				sev,
				lipgloss.NewStyle().Foreground(cText).Render(trunc(a.Name, 28)),
				lipgloss.NewStyle().Foreground(cDim).Render(a.Duration),
			))
		}
		if d.Alerts.Count > 4 {
			alertContent.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(fmt.Sprintf(" +%d more\n", d.Alerts.Count-4)))
		}
	}

	// Anomalies
	alertContent.WriteString("\n")
	if d.Anomalies.Unacknowledged > 0 {
		anomColor := cWarning
		if d.Anomalies.Critical > 0 {
			anomColor = cDanger
		}
		anomText := fmt.Sprintf("%d unacknowledged", d.Anomalies.Unacknowledged)
		if d.Anomalies.Critical > 0 {
			anomText += fmt.Sprintf(" (%d critical)", d.Anomalies.Critical)
		}
		alertContent.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Anomalies") + "\n")
		alertContent.WriteString(lipgloss.NewStyle().Foreground(anomColor).Render(" "+anomText) + "\n")
	} else {
		alertContent.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Anomalies") + "\n")
		alertContent.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Render(" None") + "\n")
	}

	alertTitle := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Active Alerts")
	alertPanel := lipgloss.NewStyle().
		Width(halfW).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(alertTitle + "\n" + alertContent.String())

	return " " + lipgloss.JoinHorizontal(lipgloss.Top, svcPanel, " ", alertPanel) + "\n\n"
}

// -- Error Hotspots --

func renderHotspots(d *client.DashboardData, w int) string {
	var content strings.Builder
	for _, h := range d.ErrorHotspots {
		name := trunc(fmt.Sprintf("%s / %s", h.Service, h.Operation), 40)
		content.WriteString(fmt.Sprintf(" %s  %s  %s\n",
			lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%-40s", name)),
			lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(fmt.Sprintf("%d err", h.Errors)),
			lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("%.0f%% of %d", h.ErrorRate, h.Total)),
		))
	}

	title := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Error Hotspots (24h)")
	panel := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(title + "\n" + content.String())

	return " " + panel + "\n\n"
}

// -- Helpers --

func fmtDelta(val float64, invert bool, unit string) string {
	arrow := "\u2191"
	color := cSuccess
	if val < 0 {
		arrow = "\u2193"
	}
	if (val > 0 && invert) || (val < 0 && !invert) {
		color = cDanger
	}
	return lipgloss.NewStyle().Foreground(color).Render(
		fmt.Sprintf("%s%.0f%s vs yday", arrow, math.Abs(val), unit))
}

func fmtSeverity(s string) string {
	switch s {
	case "critical":
		return lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("CRIT")
	case "warning":
		return lipgloss.NewStyle().Foreground(cWarning).Bold(true).Render("WARN")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render("INFO")
	}
}

func getErrColor(rate float64) lipgloss.TerminalColor {
	if rate > 5 {
		return cDanger
	}
	if rate > 1 {
		return cWarning
	}
	return cSuccess
}

func getLatColor(ms int) lipgloss.TerminalColor {
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
