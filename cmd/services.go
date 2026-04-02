package cmd

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Browse service health and metrics",
	Long: `Display an interactive service explorer with health indicators and detailed metrics.

Key bindings:
  j/k or arrows   Navigate service list
  enter            Open service detail
  tab              Switch between metrics and errors tabs
  esc              Go back to service list
  r                Refresh
  q or Ctrl+C      Quit`,
	RunE: runServices,
}

func init() {
	rootCmd.AddCommand(servicesCmd)
	servicesCmd.Flags().String("url", "", "API base URL")
	servicesCmd.Flags().Bool("dev", false, "")
	servicesCmd.Flags().MarkHidden("dev")
}

// -- Messages --

type svcListLoadedMsg struct {
	services []client.CLIServiceHealth
}

type svcDetailLoadedMsg struct {
	detail *client.CLIServiceDetail
}

type svcErrorsLoadedMsg struct {
	errors []client.CLIServiceError
}

type svcErrMsg struct {
	err error
}

// -- Commands --

func fetchServiceList(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetServicesWithHealth()
		if err != nil {
			return svcErrMsg{err: err}
		}
		return svcListLoadedMsg{services: resp.Services}
	}
}

func fetchSvcDetail(c *client.Client, name string) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.GetServiceDetail(name)
		if err != nil {
			return svcErrMsg{err: err}
		}
		return svcDetailLoadedMsg{detail: detail}
	}
}

func fetchSvcErrors(c *client.Client, name string) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetServiceErrors(name)
		if err != nil {
			return svcErrMsg{err: err}
		}
		return svcErrorsLoadedMsg{errors: resp.Errors}
	}
}

// -- Model --

type servicesModel struct {
	apiClient *client.Client
	width     int
	height    int

	// Service list
	services []client.CLIServiceHealth
	cursor   int
	loading  bool
	err      error
	quitting bool

	// Detail view
	view        string // "list" | "detail"
	detail      *client.CLIServiceDetail
	errors      []client.CLIServiceError
	detailTab   string // "metrics" | "errors"
	errorCursor int
}

func (m servicesModel) Init() tea.Cmd {
	return fetchServiceList(m.apiClient)
}

func (m servicesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case svcListLoadedMsg:
		m.loading = false
		m.services = msg.services
		m.err = nil
		if m.cursor >= len(m.services) {
			m.cursor = max(0, len(m.services)-1)
		}
		return m, nil

	case svcDetailLoadedMsg:
		m.detail = msg.detail
		if m.detail != nil && m.errors != nil {
			m.loading = false
		}
		return m, nil

	case svcErrorsLoadedMsg:
		m.errors = msg.errors
		if m.detail != nil && m.errors != nil {
			m.loading = false
		}
		return m, nil

	case svcErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleSvcKey(msg)
	}

	return m, nil
}

func (m servicesModel) handleSvcKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.view == "detail" {
		return m.handleDetailKey(key)
	}

	// List view keys
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.services)-1 {
			m.cursor++
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "enter":
		if len(m.services) > 0 && m.cursor < len(m.services) {
			svc := m.services[m.cursor]
			m.view = "detail"
			m.loading = true
			m.detail = nil
			m.errors = nil
			m.detailTab = "metrics"
			m.errorCursor = 0
			return m, tea.Batch(
				fetchSvcDetail(m.apiClient, svc.Name),
				fetchSvcErrors(m.apiClient, svc.Name),
			)
		}

	case "r":
		m.loading = true
		return m, fetchServiceList(m.apiClient)
	}

	return m, nil
}

func (m servicesModel) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.view = "list"
		m.detail = nil
		m.errors = nil
		m.errorCursor = 0

	case "tab":
		if m.detailTab == "metrics" {
			m.detailTab = "errors"
		} else {
			m.detailTab = "metrics"
		}

	case "j", "down":
		if m.detailTab == "errors" && m.errors != nil {
			if m.errorCursor < len(m.errors)-1 {
				m.errorCursor++
			}
		}

	case "k", "up":
		if m.detailTab == "errors" && m.errorCursor > 0 {
			m.errorCursor--
		}

	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

// -- View --

func (m servicesModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 100
	}

	if m.view == "detail" {
		if m.loading {
			return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading service details...") + "\n"
		}
		if m.detail != nil {
			if m.detailTab == "errors" {
				return m.renderErrorsTab(w)
			}
			return m.renderMetricsTab(w)
		}
	}

	// List view
	if m.loading && len(m.services) == 0 {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading services...") + "\n"
	}

	if m.err != nil {
		return "\n" + lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(fmt.Sprintf("Error: %s", m.err.Error())) + "\n"
	}

	if len(m.services) == 0 {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No services found") + "\n"
	}

	return m.renderListView(w)
}

// -- List View --

func (m servicesModel) renderListView(w int) string {
	var b strings.Builder

	// Build service rows
	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-3s %-24s %10s %12s %12s", "", "SERVICE", "ERROR RATE", "AVG LATENCY", "REQUESTS")
	rows = append(rows, headerStyle.Render(hdr))

	for i, svc := range m.services {
		isSelected := i == m.cursor
		dot := healthDot(svc.ErrorRate)
		name := trunc(svc.Name, 24)
		errRate := formatErrorRate(svc.ErrorRate)
		latency := formatLatency(svc.AvgLatency)
		reqs := formatCount(svc.RequestCount)

		row := fmt.Sprintf("  %s %-24s %10s %12s %12s", dot, name, errRate, latency, reqs)

		if isSelected {
			row = lipgloss.NewStyle().
				Background(cBrand).
				Foreground(lipgloss.Color("#ffffff")).
				Width(w - 4).
				Render(row)
		} else {
			row = lipgloss.NewStyle().Foreground(cText).Render(row)
		}
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	title := fmt.Sprintf("Services (%d)", len(m.services))
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n\n")

	// Footer
	footer := lipgloss.NewStyle().Foreground(cDim).Render(
		" j/k navigate | enter open | r refresh | q quit")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Detail: Metrics Tab --

func (m servicesModel) renderMetricsTab(w int) string {
	var b strings.Builder
	d := m.detail

	// Summary panel
	summaryLines := []string{
		lipgloss.NewStyle().Foreground(cDim).Render("Service:  ") +
			lipgloss.NewStyle().Foreground(cText).Bold(true).Render(d.Name),
		lipgloss.NewStyle().Foreground(cDim).Render("Requests: ") +
			lipgloss.NewStyle().Foreground(cText).Render(formatCount(d.RequestCount)),
		lipgloss.NewStyle().Foreground(cDim).Render("Error Rate: ") +
			lipgloss.NewStyle().Foreground(healthColor(d.ErrorRate)).Bold(true).Render(formatErrorRate(d.ErrorRate)),
		lipgloss.NewStyle().Foreground(cDim).Render("P50 Latency: ") +
			lipgloss.NewStyle().Foreground(cText).Render(formatLatency(d.P50Latency)),
		lipgloss.NewStyle().Foreground(cDim).Render("P95 Latency: ") +
			lipgloss.NewStyle().Foreground(cText).Render(formatLatency(d.P95Latency)),
		lipgloss.NewStyle().Foreground(cDim).Render("P99 Latency: ") +
			lipgloss.NewStyle().Foreground(cText).Render(formatLatency(d.P99Latency)),
		lipgloss.NewStyle().Foreground(cDim).Render("Avg Latency: ") +
			lipgloss.NewStyle().Foreground(cText).Render(formatLatency(d.AvgLatency)),
	}
	summaryContent := strings.Join(summaryLines, "\n")
	b.WriteString("\n")
	b.WriteString(titledPanel("Service Detail", summaryContent, w-2))
	b.WriteString("\n")

	// Operations table
	if len(d.Operations) > 0 {
		var opRows []string
		opHdr := fmt.Sprintf("  %-30s %8s %10s %12s %12s", "OPERATION", "COUNT", "ERROR RATE", "P95", "AVG")
		opRows = append(opRows, lipgloss.NewStyle().Foreground(cMuted).Bold(true).Render(opHdr))

		for _, op := range d.Operations {
			errStyle := lipgloss.NewStyle().Foreground(healthColor(op.ErrorRate))
			row := fmt.Sprintf("  %-30s %8s %10s %12s %12s",
				trunc(op.OperationName, 30),
				formatCount(op.Count),
				errStyle.Render(formatErrorRate(op.ErrorRate)),
				formatLatency(op.P95Latency),
				formatLatency(op.AvgLatency),
			)
			opRows = append(opRows, lipgloss.NewStyle().Foreground(cText).Render(row))
		}

		opContent := strings.Join(opRows, "\n")
		b.WriteString(titledPanel("Operations", opContent, w-2))
		b.WriteString("\n")
	}

	// Top errors summary
	if len(d.TopErrors) > 0 {
		var errRows []string
		errHdr := fmt.Sprintf("  %-60s %8s", "ERROR MESSAGE", "COUNT")
		errRows = append(errRows, lipgloss.NewStyle().Foreground(cMuted).Bold(true).Render(errHdr))

		for _, e := range d.TopErrors {
			row := fmt.Sprintf("  %-60s %8d",
				trunc(e.Message, 60),
				e.Count,
			)
			errRows = append(errRows, lipgloss.NewStyle().Foreground(cDanger).Render(row))
		}

		errContent := strings.Join(errRows, "\n")
		b.WriteString(titledPanel("Top Errors", errContent, w-2))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	footer := lipgloss.NewStyle().Foreground(cDim).Render(" tab: errors | esc: back | q: quit")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Detail: Errors Tab --

func (m servicesModel) renderErrorsTab(w int) string {
	var b strings.Builder
	d := m.detail

	// Header with service name
	headerContent := lipgloss.NewStyle().Foreground(cDim).Render("Service: ") +
		lipgloss.NewStyle().Foreground(cText).Bold(true).Render(d.Name) +
		"  " +
		lipgloss.NewStyle().Foreground(cDim).Render("Errors: ") +
		lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(fmt.Sprintf("%d", len(m.errors)))

	b.WriteString("\n")
	b.WriteString(titledPanel("Recent Errors", headerContent, w-2))
	b.WriteString("\n")

	if len(m.errors) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No recent errors"))
		b.WriteString("\n")
	} else {
		var errRows []string
		errHdr := fmt.Sprintf("  %-12s %-24s %-40s %10s", "TIME", "OPERATION", "MESSAGE", "DURATION")
		errRows = append(errRows, lipgloss.NewStyle().Foreground(cMuted).Bold(true).Render(errHdr))

		// Calculate visible window
		visibleRows := m.height - 12
		if visibleRows < 5 {
			visibleRows = 5
		}

		startIdx := 0
		if m.errorCursor >= visibleRows {
			startIdx = m.errorCursor - visibleRows + 1
		}
		endIdx := startIdx + visibleRows
		if endIdx > len(m.errors) {
			endIdx = len(m.errors)
		}

		for i := startIdx; i < endIdx; i++ {
			e := m.errors[i]
			isSelected := i == m.errorCursor
			ts := formatTimestamp(e.Timestamp)
			op := trunc(e.Operation, 24)
			msg := trunc(e.Message, 40)
			dur := formatLatency(e.DurationMs)

			row := fmt.Sprintf("  %-12s %-24s %-40s %10s", ts, op, msg, dur)

			if isSelected {
				row = lipgloss.NewStyle().
					Background(cDanger).
					Foreground(lipgloss.Color("#ffffff")).
					Width(w - 4).
					Render(row)
			} else {
				row = lipgloss.NewStyle().Foreground(cText).Render(row)
			}
			errRows = append(errRows, row)
		}

		errContent := strings.Join(errRows, "\n")
		b.WriteString(titledPanel("Error Spans", errContent, w-2))
		b.WriteString("\n")

		// Scroll indicator
		if len(m.errors) > visibleRows {
			scrollInfo := fmt.Sprintf(" [%d/%d]", m.errorCursor+1, len(m.errors))
			b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(scrollInfo))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	footer := lipgloss.NewStyle().Foreground(cDim).Render(" tab: metrics | j/k scroll | esc: back | q: quit")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Health Helpers --

func healthColor(errorRate float64) lipgloss.TerminalColor {
	if errorRate >= 5.0 {
		return cDanger
	}
	if errorRate >= 1.0 {
		return cWarning
	}
	return cSuccess
}

func healthDot(errorRate float64) string {
	return lipgloss.NewStyle().Foreground(healthColor(errorRate)).Render("\u25cf")
}

// -- Format Helpers --

func formatLatency(ms float64) string {
	if ms >= 1000 {
		return fmt.Sprintf("%.1fs", ms/1000)
	}
	return fmt.Sprintf("%.1fms", ms)
}

func formatCount(n int64) string {
	if n >= 10000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d", n)
}

func formatErrorRate(rate float64) string {
	return fmt.Sprintf("%.1f%%", rate)
}

func formatTimestamp(ts string) string {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	dur := time.Since(t)
	switch {
	case dur < time.Minute:
		return fmt.Sprintf("%ds ago", int(dur.Seconds()))
	case dur < time.Hour:
		return fmt.Sprintf("%dm ago", int(dur.Minutes()))
	case dur < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(dur.Hours()))
	default:
		return t.Format("15:04:05")
	}
}

// -- Runner --

func runServices(cmd *cobra.Command, args []string) error {
	c, err := NewAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	model := servicesModel{
		apiClient: c,
		view:      "list",
		loading:   true,
		detailTab: "metrics",
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("services error: %w", err)
	}
	return nil
}
