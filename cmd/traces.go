package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

var tracesCmd = &cobra.Command{
	Use:   "traces",
	Short: "Browse and filter traces interactively",
	Long: `Display an interactive trace browser with filtering and keyboard navigation.

Key bindings:
  j/k or arrows   Navigate trace list
  enter            Open trace detail (coming soon)
  /                Filter by service
  e                Toggle errors-only filter
  d                Set minimum duration filter
  t                Cycle time range (1h/6h/24h/all)
  r                Refresh
  q or Ctrl+C      Quit`,
	RunE: runTraces,
}

func init() {
	rootCmd.AddCommand(tracesCmd)
	tracesCmd.Flags().String("api-key", "", "TraceKit API key (overrides .env)")
	tracesCmd.Flags().String("url", "", "API base URL")
	tracesCmd.Flags().Bool("dev", false, "")
	tracesCmd.Flags().MarkHidden("dev")
}

// -- Messages --

type tracesLoadedMsg struct {
	traces     []client.CLITrace
	totalCount int
}

type servicesLoadedMsg struct {
	services []client.CLIService
}

type tracesErrMsg struct{ err error }

// -- Commands --

func fetchTraces(c *client.Client, service string, hasError bool, minDur int, timeWindow string, limit, offset int) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetTraces(service, hasError, minDur, timeWindow, limit, offset)
		if err != nil {
			return tracesErrMsg{err: err}
		}
		return tracesLoadedMsg{traces: resp.Traces, totalCount: resp.TotalCount}
	}
}

func fetchServices(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetServices()
		if err != nil {
			return tracesErrMsg{err: err}
		}
		return servicesLoadedMsg{services: resp.Services}
	}
}

// -- Model --

type tracesModel struct {
	client     *client.Client
	traces     []client.CLITrace
	services   []client.CLIService
	totalCount int
	cursor     int
	offset     int
	limit      int
	width      int
	height     int
	err        error
	loading    bool
	quitting   bool

	// Filter state
	filterService    string
	filterErrors     bool
	filterMinDurMs   int
	filterTimeWindow string
	filterMode       string // "" = normal, "service" = picking service, "duration" = entering duration
	filterInput      string
	serviceOptions   []string
	serviceCursor    int

	// View state
	selectedTraceID string // for future detail view (Plan 02)
}

func (m tracesModel) Init() tea.Cmd {
	return tea.Batch(
		fetchTraces(m.client, m.filterService, m.filterErrors, m.filterMinDurMs, m.filterTimeWindow, m.limit, m.offset),
		fetchServices(m.client),
	)
}

func (m tracesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tracesLoadedMsg:
		m.loading = false
		m.traces = msg.traces
		m.totalCount = msg.totalCount
		m.err = nil
		// Ensure cursor is within bounds
		if m.cursor >= len(m.traces) {
			m.cursor = max(0, len(m.traces)-1)
		}
		return m, nil

	case servicesLoadedMsg:
		m.services = msg.services
		m.serviceOptions = make([]string, 0, len(msg.services)+1)
		m.serviceOptions = append(m.serviceOptions, "") // "All Services"
		for _, s := range msg.services {
			m.serviceOptions = append(m.serviceOptions, s.Name)
		}
		return m, nil

	case tracesErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m tracesModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Service filter mode
	if m.filterMode == "service" {
		switch key {
		case "j", "down":
			if m.serviceCursor < len(m.serviceOptions)-1 {
				m.serviceCursor++
			}
		case "k", "up":
			if m.serviceCursor > 0 {
				m.serviceCursor--
			}
		case "enter":
			m.filterService = m.serviceOptions[m.serviceCursor]
			m.filterMode = ""
			m.offset = 0
			m.cursor = 0
			m.loading = true
			return m, m.refetch()
		case "esc":
			m.filterMode = ""
		}
		return m, nil
	}

	// Duration filter mode
	if m.filterMode == "duration" {
		switch key {
		case "0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
			m.filterInput += key
		case "backspace":
			if len(m.filterInput) > 0 {
				m.filterInput = m.filterInput[:len(m.filterInput)-1]
			}
		case "enter":
			if m.filterInput != "" {
				val, err := strconv.Atoi(m.filterInput)
				if err == nil {
					m.filterMinDurMs = val
				}
			} else {
				m.filterMinDurMs = 0
			}
			m.filterMode = ""
			m.filterInput = ""
			m.offset = 0
			m.cursor = 0
			m.loading = true
			return m, m.refetch()
		case "esc":
			m.filterMode = ""
			m.filterInput = ""
		}
		return m, nil
	}

	// Normal mode
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.traces)-1 {
			m.cursor++
		} else if m.offset+m.limit < m.totalCount {
			// Load next page
			m.offset += m.limit
			m.cursor = 0
			m.loading = true
			return m, m.refetch()
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		} else if m.offset > 0 {
			// Load previous page
			m.offset -= m.limit
			if m.offset < 0 {
				m.offset = 0
			}
			m.cursor = m.limit - 1
			m.loading = true
			return m, m.refetch()
		}

	case "enter":
		if len(m.traces) > 0 && m.cursor < len(m.traces) {
			m.selectedTraceID = m.traces[m.cursor].TraceID
			// Detail view placeholder for Plan 02
		}

	case "e":
		m.filterErrors = !m.filterErrors
		m.offset = 0
		m.cursor = 0
		m.loading = true
		return m, m.refetch()

	case "d":
		m.filterMode = "duration"
		m.filterInput = ""

	case "/":
		m.filterMode = "service"
		m.serviceCursor = 0
		// Position cursor on current filter
		for i, opt := range m.serviceOptions {
			if opt == m.filterService {
				m.serviceCursor = i
				break
			}
		}

	case "t":
		// Cycle time window: 1h -> 6h -> 24h -> all -> 1h
		switch m.filterTimeWindow {
		case "1h":
			m.filterTimeWindow = "6h"
		case "6h":
			m.filterTimeWindow = "24h"
		case "24h":
			m.filterTimeWindow = ""
		default:
			m.filterTimeWindow = "1h"
		}
		m.offset = 0
		m.cursor = 0
		m.loading = true
		return m, m.refetch()

	case "r":
		m.loading = true
		return m, m.refetch()
	}

	return m, nil
}

func (m tracesModel) refetch() tea.Cmd {
	return fetchTraces(m.client, m.filterService, m.filterErrors, m.filterMinDurMs, m.filterTimeWindow, m.limit, m.offset)
}

// -- View --

func (m tracesModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 100
	}

	var b strings.Builder

	// Header with filter chips
	b.WriteString("\n")
	b.WriteString(m.renderHeader(w))
	b.WriteString("\n")

	// Service filter overlay
	if m.filterMode == "service" {
		b.WriteString(m.renderServicePicker(w))
		return b.String()
	}

	// Duration filter overlay
	if m.filterMode == "duration" {
		b.WriteString(m.renderDurationInput(w))
		return b.String()
	}

	// Loading state
	if m.loading && len(m.traces) == 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading traces..."))
		b.WriteString("\n")
		return b.String()
	}

	// Error state
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(fmt.Sprintf("Error: %s", m.err.Error())))
		b.WriteString("\n")
		return b.String()
	}

	// Empty state
	if len(m.traces) == 0 {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No traces found"))
		b.WriteString("\n")
		hint := "Try adjusting filters or pressing 't' to change time range"
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render(hint))
		b.WriteString("\n")
		return b.String()
	}

	// Table header
	b.WriteString(m.renderTableHeader(w))
	b.WriteString("\n")

	// Table rows - fill available height
	visibleRows := m.height - 7 // header + table header + footer + padding
	if visibleRows < 3 {
		visibleRows = 3
	}
	if visibleRows > len(m.traces) {
		visibleRows = len(m.traces)
	}

	for i := 0; i < visibleRows; i++ {
		trace := m.traces[i]
		isSelected := i == m.cursor
		b.WriteString(m.renderTraceRow(trace, isSelected, w))
		b.WriteString("\n")
	}

	// Pagination info
	b.WriteString("\n")
	b.WriteString(m.renderPagination(w))
	b.WriteString("\n")

	// Footer help
	b.WriteString(m.renderFooter())
	b.WriteString("\n")

	return b.String()
}

func (m tracesModel) renderHeader(w int) string {
	title := lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render("Traces")
	sep := lipgloss.NewStyle().Foreground(cDim).Render("  ")

	var chips []string
	if m.filterService != "" {
		chips = append(chips, lipgloss.NewStyle().Foreground(cCyan).Render("[service: "+m.filterService+"]"))
	}
	if m.filterErrors {
		chips = append(chips, lipgloss.NewStyle().Foreground(cDanger).Render("[errors only]"))
	}
	if m.filterMinDurMs > 0 {
		chips = append(chips, lipgloss.NewStyle().Foreground(cWarning).Render(fmt.Sprintf("[>%dms]", m.filterMinDurMs)))
	}
	// Time window chip
	switch m.filterTimeWindow {
	case "1h":
		chips = append(chips, lipgloss.NewStyle().Foreground(cMuted).Render("[last 1h]"))
	case "6h":
		chips = append(chips, lipgloss.NewStyle().Foreground(cMuted).Render("[last 6h]"))
	case "24h":
		chips = append(chips, lipgloss.NewStyle().Foreground(cMuted).Render("[last 24h]"))
	default:
		chips = append(chips, lipgloss.NewStyle().Foreground(cMuted).Render("[all time]"))
	}

	if m.loading {
		chips = append(chips, lipgloss.NewStyle().Foreground(cMuted).Render("loading..."))
	}

	chipStr := ""
	if len(chips) > 0 {
		chipStr = sep + strings.Join(chips, " ")
	}

	return " " + title + chipStr
}

func (m tracesModel) renderTableHeader(w int) string {
	hdr := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	svcW := 20
	opW := w - 46 // status(3) + svc(20) + duration(8) + time(10) + gaps(5)
	if opW < 10 {
		opW = 10
	}

	return " " + hdr.Render(fmt.Sprintf(" %-3s %-*s %-*s %8s %10s", "", svcW, "SERVICE", opW, "OPERATION", "DURATION", "TIME"))
}

func (m tracesModel) renderTraceRow(trace client.CLITrace, isSelected bool, w int) string {
	svcW := 20
	opW := w - 46
	if opW < 10 {
		opW = 10
	}

	// Status indicator
	var status string
	if trace.HasError {
		status = lipgloss.NewStyle().Foreground(cDanger).Render("\u25cf")
	} else {
		status = lipgloss.NewStyle().Foreground(cSuccess).Render("\u25cf")
	}

	// Format fields
	service := trunc(trace.ServiceName, svcW)
	operation := trunc(trace.OperationName, opW)
	duration := fmtTraceDuration(trace.DurationMs)
	timeAgo := fmtTimeAgo(trace.StartTime)

	row := fmt.Sprintf(" %s %-*s %-*s %8s %10s",
		status, svcW, service, opW, operation, duration, timeAgo)

	if isSelected {
		return lipgloss.NewStyle().
			Background(cBrand).
			Foreground(lipgloss.Color("#ffffff")).
			Width(w).
			Render(row)
	}

	return lipgloss.NewStyle().Foreground(cText).Render(row)
}

func (m tracesModel) renderPagination(w int) string {
	if m.totalCount == 0 {
		return ""
	}
	start := m.offset + 1
	end := m.offset + len(m.traces)
	info := fmt.Sprintf("Showing %d-%d of %s", start, end, fmtNumber(m.totalCount))
	return lipgloss.NewStyle().Foreground(cDim).Padding(0, 1).Render(info)
}

func (m tracesModel) renderFooter() string {
	help := lipgloss.NewStyle().Foreground(cDim).Render(
		" j/k navigate | enter open | / service | e errors | d duration | t time range | r refresh | q quit")
	return help
}

func (m tracesModel) renderServicePicker(w int) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Select Service"))
	b.WriteString("\n\n")

	for i, opt := range m.serviceOptions {
		label := opt
		if label == "" {
			label = "All Services"
		}

		isSelected := i == m.serviceCursor
		style := lipgloss.NewStyle().Foreground(cText).Padding(0, 2)
		if isSelected {
			style = lipgloss.NewStyle().
				Background(cBrand).
				Foreground(lipgloss.Color("#ffffff")).
				Padding(0, 2).
				Width(w)
		}

		prefix := "  "
		if opt == m.filterService || (opt == "" && m.filterService == "") {
			prefix = lipgloss.NewStyle().Foreground(cSuccess).Render("\u2713 ")
		}

		if isSelected {
			b.WriteString(style.Render(prefix + label))
		} else {
			b.WriteString(style.Render(prefix + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("j/k navigate | enter select | esc cancel"))
	b.WriteString("\n")

	return b.String()
}

func (m tracesModel) renderDurationInput(w int) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Minimum Duration (ms)"))
	b.WriteString("\n\n")

	input := m.filterInput
	if input == "" {
		input = lipgloss.NewStyle().Foreground(cDim).Render("type a number...")
	} else {
		input = lipgloss.NewStyle().Foreground(cText).Bold(true).Render(input + " ms")
	}
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render("> " + input))
	b.WriteString("\n\n")

	hint := "enter to apply | esc to cancel | empty to clear filter"
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render(hint))
	b.WriteString("\n")

	return b.String()
}

// -- Helpers --

func fmtTraceDuration(ms int) string {
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func fmtTimeAgo(startTimeStr string) string {
	t, err := time.Parse(time.RFC3339Nano, startTimeStr)
	if err != nil {
		// Try other formats
		t, err = time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return startTimeStr
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
		days := int(dur.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// -- Runner --

func runTraces(cmd *cobra.Command, args []string) error {
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

	model := tracesModel{
		client:           c,
		limit:            30,
		filterTimeWindow: "24h",
		loading:          true,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("traces error: %w", err)
	}
	return nil
}
