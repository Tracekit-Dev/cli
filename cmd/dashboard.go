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
	cText    = lipgloss.AdaptiveColor{Light: "#111827", Dark: "#ffffff"}
	cDim     = lipgloss.AdaptiveColor{Light: "#6b7280", Dark: "#6b7280"}
	cBorder  = lipgloss.AdaptiveColor{Light: "#d1d5db", Dark: "#4b5563"}
	cCyan    = lipgloss.Color("#0891b2")
	cPurple  = lipgloss.Color("#7c3aed")
	cAmber   = lipgloss.Color("#d97706")
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
	window    string // "1h", "6h", "24h"
}

type tickMsg time.Time
type dataMsg struct {
	data *client.DashboardData
	err  error
}

func fetchDashData(c *client.Client, window string) tea.Cmd {
	return func() tea.Msg {
		data, err := c.GetDashboard(window)
		return dataMsg{data: data, err: err}
	}
}

func (m dashModel) Init() tea.Cmd {
	return tea.Batch(fetchDashData(m.client, m.window), tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }))
}

func (m dashModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "r":
			return m, fetchDashData(m.client, m.window)
		case "1":
			m.window = "1h"
			return m, fetchDashData(m.client, m.window)
		case "2":
			m.window = "6h"
			return m, fetchDashData(m.client, m.window)
		case "3":
			m.window = "24h"
			return m, fetchDashData(m.client, m.window)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tickMsg:
		return m, tea.Batch(fetchDashData(m.client, m.window), tea.Tick(30*time.Second, func(t time.Time) tea.Msg { return tickMsg(t) }))
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
	if w < 40 {
		return lipgloss.NewStyle().Foreground(cWarning).Padding(1, 1).Render(
			"Terminal too narrow (min 40 cols)\nCurrent: " + fmt.Sprintf("%d", w) + " cols\nPlease widen your terminal.")
	}

	contentWidth := w - 2
	isNarrow := w < 80

	var b strings.Builder

	// -- Header line (inline stats like competitor) --
	b.WriteString("\n")
	b.WriteString(renderHeaderLine(m, contentWidth))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(cDanger).Padding(0, 1).Render(fmt.Sprintf(" Error: %s", m.err.Error())))
		b.WriteString("\n")
		return b.String()
	}
	if m.data == nil {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Render("  Loading..."))
		return b.String()
	}

	d := m.data

	// -- Stats summary line --
	b.WriteString(renderStatsLine(d, contentWidth))
	b.WriteString("\n\n")

	// -- Charts: Throughput + Error Rate side by side (or stacked on narrow) --
	if len(d.TimeSeries) > 1 {
		if isNarrow {
			b.WriteString(renderChartPanel(d, contentWidth, "Requests", "req"))
			b.WriteString("\n")
			b.WriteString(renderChartPanel(d, contentWidth, "Error Rate", "err"))
		} else {
			halfW := (contentWidth - 3) / 2
			reqPanel := renderChartPanel(d, halfW, "Requests", "req")
			errPanel := renderChartPanel(d, halfW, "Error Rate", "err")
			b.WriteString(" " + lipgloss.JoinHorizontal(lipgloss.Top, reqPanel, " ", errPanel))
		}
		b.WriteString("\n\n")
	}

	// -- Latency chart (P50/P95/P99) full width --
	if len(d.TimeSeries) > 1 {
		b.WriteString(renderChartPanel(d, contentWidth, "Response Time", "latency"))
		b.WriteString("\n\n")
	}

	// -- Services + Alerts side by side (or stacked on narrow) --
	if isNarrow {
		b.WriteString(renderServicesPanel(d, contentWidth))
		b.WriteString("\n")
		b.WriteString(renderAlertsPanel(d, contentWidth))
	} else {
		halfW := (contentWidth - 3) / 2
		svcPanel := renderServicesPanel(d, halfW)
		alertPanel := renderAlertsPanel(d, halfW)
		b.WriteString(" " + lipgloss.JoinHorizontal(lipgloss.Top, svcPanel, " ", alertPanel))
	}
	b.WriteString("\n\n")

	// -- Error Hotspots --
	if len(d.ErrorHotspots) > 0 {
		b.WriteString(renderHotspots(d, contentWidth))
		b.WriteString("\n")
	}

	// -- Footer --
	b.WriteString(renderFooter(m))
	b.WriteString("\n")

	return b.String()
}

// -- Header line: "TraceKit  .  last 1h  .  refreshed 30s ago" --

func renderHeaderLine(m dashModel, w int) string {
	brand := lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render("TraceKit")
	sep := lipgloss.NewStyle().Foreground(cDim).Render("  ·  ")

	windowLabel := lipgloss.NewStyle().Foreground(cText).Render("last " + m.window)

	refreshed := ""
	if !m.lastFetch.IsZero() {
		ago := time.Since(m.lastFetch).Round(time.Second)
		refreshed = lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("refreshed %s ago", ago))
	} else {
		refreshed = lipgloss.NewStyle().Foreground(cMuted).Render("loading...")
	}

	return " " + brand + sep + windowLabel + sep + refreshed
}

// -- Stats summary: "Health: 95%  .  5 services  .  1,234 traces  .  12 errors (0.5%)  .  45ms avg" --

func renderStatsLine(d *client.DashboardData, w int) string {
	s := d.Stats
	sep := lipgloss.NewStyle().Foreground(cDim).Render("  ·  ")

	healthColor := cSuccess
	if s.HealthScore < 80 {
		healthColor = cWarning
	}
	if s.HealthScore < 60 {
		healthColor = cDanger
	}

	parts := []string{
		lipgloss.NewStyle().Foreground(cMuted).Render("Health: ") +
			lipgloss.NewStyle().Foreground(healthColor).Bold(true).Render(fmt.Sprintf("%.0f%%", s.HealthScore)),
		lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%d", s.Services)) +
			lipgloss.NewStyle().Foreground(cMuted).Render(" services"),
		lipgloss.NewStyle().Foreground(cText).Render(fmtNumber(s.Traces24h)) +
			lipgloss.NewStyle().Foreground(cMuted).Render(" traces"),
		lipgloss.NewStyle().Foreground(getErrColor(s.ErrorRate)).Render(fmt.Sprintf("%d", s.Errors24h)) +
			lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf(" errors (%.1f%%)", s.ErrorRate)),
		lipgloss.NewStyle().Foreground(getLatColor(s.AvgResponse)).Render(fmtMs(float64(s.AvgResponse))) +
			lipgloss.NewStyle().Foreground(cMuted).Render(" avg"),
	}

	// Add deltas if available
	deltaStr := ""
	if s.Deltas.HasPrevious && s.Deltas.Health != 0 {
		deltaStr = "  " + fmtDelta(s.Deltas.Health, false, "pts")
	}

	return " " + strings.Join(parts, sep) + deltaStr
}

// -- Chart Panel with Y-axis labels, X-axis time labels, title in border --

func renderChartPanel(d *client.DashboardData, w int, title string, chartType string) string {
	yAxisW := 7 // width for Y-axis labels (e.g., "1.2K  ")
	chartW := w - yAxisW - 4 // subtract y-axis, border padding
	chartH := 6

	if chartW < 10 {
		chartW = 10
	}

	var data []float64
	var chartStr string
	var maxVal, minVal float64
	var summaryLine string

	switch chartType {
	case "req":
		data = make([]float64, len(d.TimeSeries))
		peak := 0
		latest := 0
		for i, ts := range d.TimeSeries {
			data[i] = float64(ts.Requests)
			if ts.Requests > peak {
				peak = ts.Requests
			}
			latest = ts.Requests
		}
		chartStr = ui.RenderSparkline(data, chartW, chartH, cCyan)
		maxVal, minVal = sliceMinMax(data)
		summaryLine = lipgloss.NewStyle().Foreground(cMuted).Render(
			fmt.Sprintf("Now: ") + lipgloss.NewStyle().Foreground(cCyan).Render(fmtNumber(latest)) +
				lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("  Avg: %s  Peak: %s", fmtNumber(int(avg(data))), fmtNumber(peak))))

	case "err":
		data = make([]float64, len(d.TimeSeries))
		for i, ts := range d.TimeSeries {
			rate := 0.0
			if ts.Requests > 0 {
				rate = float64(ts.Errors) / float64(ts.Requests) * 100
			}
			data[i] = rate
		}
		chartStr = ui.RenderSparklineLine(data, chartW, chartH, cDanger)
		maxVal, minVal = sliceMinMax(data)
		current := 0.0
		if len(data) > 0 {
			current = data[len(data)-1]
		}
		summaryLine = lipgloss.NewStyle().Foreground(cMuted).Render(
			fmt.Sprintf("Now: ") + lipgloss.NewStyle().Foreground(cDanger).Render(fmt.Sprintf("%.1f%%", current)) +
				lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("  Avg: %.1f%%  Peak: %.1f%%", avg(data), maxVal)))

	case "latency":
		p50Data := make([]float64, len(d.TimeSeries))
		p95Data := make([]float64, len(d.TimeSeries))
		p99Data := make([]float64, len(d.TimeSeries))
		for i, ts := range d.TimeSeries {
			p50Data[i] = float64(ts.P50)
			p95Data[i] = float64(ts.P95)
			p99Data[i] = float64(ts.P99)
		}
		chartStr = ui.RenderMultiSparkline([]ui.SparklineSeries{
			{Data: p99Data, Color: cAmber},
			{Data: p95Data, Color: cPurple},
			{Data: p50Data, Color: cCyan},
		}, chartW, chartH)

		// Find overall max/min across all series
		allData := append(append(p50Data, p95Data...), p99Data...)
		maxVal, minVal = sliceMinMax(allData)

		// Current values
		curP50, curP95, curP99 := 0.0, 0.0, 0.0
		if len(d.TimeSeries) > 0 {
			last := d.TimeSeries[len(d.TimeSeries)-1]
			curP50 = float64(last.P50)
			curP95 = float64(last.P95)
			curP99 = float64(last.P99)
		}

		summaryLine = lipgloss.NewStyle().Foreground(cCyan).Render(fmt.Sprintf("p50: %s", fmtMs(curP50))) + "  " +
			lipgloss.NewStyle().Foreground(cPurple).Render(fmt.Sprintf("p95: %s", fmtMs(curP95))) + "  " +
			lipgloss.NewStyle().Foreground(cAmber).Render(fmt.Sprintf("p99: %s", fmtMs(curP99)))
	}

	// Build Y-axis labels (max, mid, min) aligned to chart rows
	yLabels := buildYAxis(maxVal, minVal, chartH, yAxisW, chartType)

	// Combine Y-axis + chart lines
	chartLines := strings.Split(chartStr, "\n")
	var combined strings.Builder
	for i := 0; i < len(chartLines); i++ {
		label := ""
		if i < len(yLabels) {
			label = yLabels[i]
		} else {
			label = strings.Repeat(" ", yAxisW)
		}
		combined.WriteString(label + chartLines[i] + "\n")
	}

	// X-axis time labels
	xAxis := strings.Repeat(" ", yAxisW) + renderDistributedTimeAxis(d, chartW)

	// Legend for latency
	legend := ""
	if chartType == "latency" {
		legend = "  " + lipgloss.NewStyle().Foreground(cCyan).Render("● p50") + "  " +
			lipgloss.NewStyle().Foreground(cPurple).Render("● p95") + "  " +
			lipgloss.NewStyle().Foreground(cAmber).Render("● p99")
	}

	// Build panel with title in border
	content := combined.String() + xAxis + "\n" + summaryLine
	if legend != "" {
		content += "\n" + legend
	}

	panel := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(cText).Bold(true).Render(title) + "\n" + content)

	return " " + panel
}

// buildYAxis creates Y-axis labels for chart rows
func buildYAxis(maxVal, minVal float64, chartH, labelW int, chartType string) []string {
	labels := make([]string, chartH)
	midVal := (maxVal + minVal) / 2

	fmtVal := func(v float64) string {
		if chartType == "err" {
			return fmt.Sprintf("%.1f%%", v)
		}
		if chartType == "latency" {
			return fmtMs(v)
		}
		return fmtNumber(int(v))
	}

	maxStr := fmtVal(maxVal)
	midStr := fmtVal(midVal)
	minStr := fmtVal(minVal)

	for i := range labels {
		labels[i] = strings.Repeat(" ", labelW)
	}

	// Top row = max, middle row = mid, bottom row = min
	if chartH > 0 {
		labels[0] = padRight(maxStr, labelW)
	}
	if chartH > 2 {
		labels[chartH/2] = padRight(midStr, labelW)
	}
	if chartH > 1 {
		labels[chartH-1] = padRight(minStr, labelW)
	}

	return labels
}

// renderDistributedTimeAxis distributes 5 evenly spaced time labels
func renderDistributedTimeAxis(d *client.DashboardData, chartW int) string {
	n := len(d.TimeSeries)
	if n <= 1 {
		return ""
	}

	// Pick 5 evenly spaced indices (or fewer if not enough data)
	labelCount := 5
	if n < labelCount {
		labelCount = n
	}

	indices := make([]int, labelCount)
	for i := 0; i < labelCount; i++ {
		indices[i] = i * (n - 1) / (labelCount - 1)
	}

	// Map each index to a column position within chartW
	labels := make([]string, labelCount)
	positions := make([]int, labelCount)
	for i, idx := range indices {
		labels[i] = d.TimeSeries[idx].Time
		positions[i] = idx * chartW / (n - 1)
		if positions[i] > chartW-len(labels[i]) {
			positions[i] = chartW - len(labels[i])
		}
		if positions[i] < 0 {
			positions[i] = 0
		}
	}

	// Build the axis string
	axis := make([]byte, chartW)
	for i := range axis {
		axis[i] = ' '
	}

	for i, pos := range positions {
		label := labels[i]
		for j := 0; j < len(label) && pos+j < chartW; j++ {
			axis[pos+j] = label[j]
		}
	}

	return lipgloss.NewStyle().Foreground(cMuted).Render(string(axis))
}

// -- Services Panel --

func renderServicesPanel(d *client.DashboardData, w int) string {
	var content strings.Builder
	if len(d.Services) == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(cMuted).Render("No services detected"))
	} else {
		hdr := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
		content.WriteString(hdr.Render(fmt.Sprintf("%-20s %6s %6s %6s", "NAME", "REQS", "ERR%", "AVG")))
		content.WriteString("\n")

		limit := len(d.Services)
		if limit > 8 {
			limit = 8
		}
		for i := 0; i < limit; i++ {
			svc := d.Services[i]
			name := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%-20s", trunc(svc.Name, 20)))
			reqs := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%6d", svc.Traces))
			errR := lipgloss.NewStyle().Foreground(getErrColor(svc.ErrorRate)).Render(fmt.Sprintf("%5.1f%%", svc.ErrorRate))
			avgR := lipgloss.NewStyle().Foreground(getLatColor(svc.AvgResponse)).Render(fmt.Sprintf("%6s", fmtMs(float64(svc.AvgResponse))))
			content.WriteString(fmt.Sprintf("%s %s %s %s\n", name, reqs, errR, avgR))
		}
		if len(d.Services) > 8 {
			content.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(fmt.Sprintf("+%d more", len(d.Services)-8)))
		}
	}

	panel := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Services") + "\n" + content.String())

	return " " + panel
}

// -- Alerts Panel --

func renderAlertsPanel(d *client.DashboardData, w int) string {
	var content strings.Builder

	if d.Alerts.Count == 0 {
		content.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Render("No active alerts") + "\n")
	} else {
		content.WriteString(lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(
			fmt.Sprintf("%d firing", d.Alerts.Count)) + "\n")
		limit := d.Alerts.Count
		if limit > 4 {
			limit = 4
		}
		for i := 0; i < limit; i++ {
			a := d.Alerts.Items[i]
			sev := fmtSeverity(a.Severity)
			content.WriteString(fmt.Sprintf(" %s %s %s\n",
				sev,
				lipgloss.NewStyle().Foreground(cText).Render(trunc(a.Name, 28)),
				lipgloss.NewStyle().Foreground(cDim).Render(a.Duration),
			))
		}
		if d.Alerts.Count > 4 {
			content.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(fmt.Sprintf(" +%d more\n", d.Alerts.Count-4)))
		}
	}

	// Anomalies
	content.WriteString("\n")
	if d.Anomalies.Unacknowledged > 0 {
		anomColor := cWarning
		if d.Anomalies.Critical > 0 {
			anomColor = cDanger
		}
		anomText := fmt.Sprintf("%d unacknowledged", d.Anomalies.Unacknowledged)
		if d.Anomalies.Critical > 0 {
			anomText += fmt.Sprintf(" (%d critical)", d.Anomalies.Critical)
		}
		content.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Anomalies") + "\n")
		content.WriteString(lipgloss.NewStyle().Foreground(anomColor).Render(" "+anomText) + "\n")
	} else {
		content.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Anomalies") + "\n")
		content.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Render(" None") + "\n")
	}

	panel := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Active Alerts") + "\n" + content.String())

	return " " + panel
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

	panel := lipgloss.NewStyle().
		Width(w).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(cBorder).
		Padding(0, 1).
		Render(lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Error Hotspots") + "\n" + content.String())

	return " " + panel
}

// -- Footer: "1 1h  2 6h  3 24h  .  r refresh  q quit" --

func renderFooter(m dashModel) string {
	var parts []string
	windows := []struct{ key, label string }{{"1", "1h"}, {"2", "6h"}, {"3", "24h"}}
	for _, win := range windows {
		if m.window == win.label {
			parts = append(parts, lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render(win.key+" "+win.label))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(cDim).Render(win.key+" "+win.label))
		}
	}
	windowBar := strings.Join(parts, "  ")
	sep := lipgloss.NewStyle().Foreground(cDim).Render("  ·  ")
	controls := lipgloss.NewStyle().Foreground(cDim).Render("r refresh  q quit")

	return " " + windowBar + sep + controls
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
		fmt.Sprintf("%s%.0f%s", arrow, math.Abs(val), unit))
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
		return "0 ms"
	}
	if ms < 1000 {
		return fmt.Sprintf("%.0f ms", ms)
	}
	return fmt.Sprintf("%.1f s", ms/1000)
}

func fmtNumber(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func padRight(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func sliceMinMax(data []float64) (float64, float64) {
	if len(data) == 0 {
		return 0, 0
	}
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return max, min
}

func avg(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
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

	p := tea.NewProgram(dashModel{client: c, window: "1h"}, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("dashboard error: %w", err)
	}
	return nil
}
