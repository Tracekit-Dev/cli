package cmd

import (
	"fmt"
	"sort"
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
  enter            Open trace detail
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

type traceDetailLoadedMsg struct {
	detail *client.TraceDetailResponse
}

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

func fetchTraceDetail(c *client.Client, traceID string) tea.Cmd {
	return func() tea.Msg {
		detail, err := c.GetTrace(traceID)
		if err != nil {
			return tracesErrMsg{err: err}
		}
		return traceDetailLoadedMsg{detail: detail}
	}
}

// -- Span tree types --

type spanNode struct {
	span     client.CLISpan
	children []*spanNode
	depth    int
	isLast   bool
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
	selectedTraceID string

	// Detail view state
	detailTrace   *client.TraceDetailResponse
	detailLoading bool
	detailScroll  int
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

	case traceDetailLoadedMsg:
		m.detailLoading = false
		m.detailTrace = msg.detail
		m.detailScroll = 0
		return m, nil

	case tracesErrMsg:
		m.loading = false
		m.detailLoading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m tracesModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Detail view mode
	if m.selectedTraceID != "" && m.detailTrace != nil {
		return m.handleDetailKey(key)
	}

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
			m.detailLoading = true
			m.detailTrace = nil
			m.detailScroll = 0
			return m, fetchTraceDetail(m.client, m.selectedTraceID)
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

func (m tracesModel) handleDetailKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "q":
		m.selectedTraceID = ""
		m.detailTrace = nil
		m.detailScroll = 0
		m.err = nil
	case "j", "down":
		m.detailScroll++
	case "k", "up":
		if m.detailScroll > 0 {
			m.detailScroll--
		}
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
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

	// Detail loading state
	if m.detailLoading {
		var b strings.Builder
		b.WriteString("\n\n")
		msg := fmt.Sprintf("Loading trace %s...", truncID(m.selectedTraceID))
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render(msg))
		b.WriteString("\n")
		return b.String()
	}

	// Detail view
	if m.selectedTraceID != "" && m.detailTrace != nil {
		return m.renderDetailView()
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

// -- Detail View --

func (m tracesModel) renderDetailView() string {
	w := m.width
	if w == 0 {
		w = 100
	}
	h := m.height
	if h == 0 {
		h = 40
	}

	detail := m.detailTrace
	spans := detail.Spans
	trace := detail.Trace

	var b strings.Builder

	// Header panel: trace summary
	headerContent := m.renderTraceSummary(trace, spans)
	headerPanel := titledPanel("Trace Detail", headerContent, w-2)
	b.WriteString("\n")
	b.WriteString(headerPanel)
	b.WriteString("\n")

	// Build span tree and flatten
	flatNodes := buildSpanTree(spans)

	// Render waterfall rows
	waterfallRows := m.renderWaterfallRows(flatNodes, trace, w)

	// Apply scrolling
	footerHeight := 2
	headerLines := strings.Count(headerPanel, "\n") + 2 // +2 for surrounding newlines
	availableRows := h - headerLines - footerHeight
	if availableRows < 3 {
		availableRows = 3
	}

	// Clamp scroll
	maxScroll := len(waterfallRows) - availableRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	scroll := m.detailScroll
	if scroll > maxScroll {
		scroll = maxScroll
	}

	// Slice visible rows
	endIdx := scroll + availableRows
	if endIdx > len(waterfallRows) {
		endIdx = len(waterfallRows)
	}
	visibleRows := waterfallRows
	if scroll < len(waterfallRows) {
		visibleRows = waterfallRows[scroll:endIdx]
	} else {
		visibleRows = nil
	}

	for _, row := range visibleRows {
		b.WriteString(row)
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(waterfallRows) > availableRows {
		scrollInfo := fmt.Sprintf(" [%d/%d]", scroll+1, len(waterfallRows))
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(scrollInfo))
		b.WriteString("\n")
	}

	// Footer
	footer := fmt.Sprintf(" esc back | j/k scroll | trace %s", truncID(m.selectedTraceID))
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(footer))
	b.WriteString("\n")

	return b.String()
}

func (m tracesModel) renderTraceSummary(trace client.CLITrace, spans []client.CLISpan) string {
	var lines []string

	// Trace ID
	idLabel := lipgloss.NewStyle().Foreground(cDim).Render("Trace ID: ")
	idVal := lipgloss.NewStyle().Foreground(cText).Bold(true).Render(truncID(trace.TraceID))
	lines = append(lines, idLabel+idVal)

	// Service
	svcLabel := lipgloss.NewStyle().Foreground(cDim).Render("Service:  ")
	svcVal := lipgloss.NewStyle().Foreground(cCyan).Render(trace.ServiceName)
	lines = append(lines, svcLabel+svcVal)

	// Duration
	durLabel := lipgloss.NewStyle().Foreground(cDim).Render("Duration: ")
	durVal := lipgloss.NewStyle().Foreground(cText).Bold(true).Render(fmtTraceDuration(trace.DurationMs))
	lines = append(lines, durLabel+durVal)

	// Span count
	spanLabel := lipgloss.NewStyle().Foreground(cDim).Render("Spans:    ")
	spanVal := lipgloss.NewStyle().Foreground(cText).Render(fmt.Sprintf("%d", len(spans)))
	lines = append(lines, spanLabel+spanVal)

	// Status
	statusLabel := lipgloss.NewStyle().Foreground(cDim).Render("Status:   ")
	var statusVal string
	if trace.HasError {
		statusVal = lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("ERROR")
	} else {
		statusVal = lipgloss.NewStyle().Foreground(cSuccess).Bold(true).Render("OK")
	}
	lines = append(lines, statusLabel+statusVal)

	return strings.Join(lines, "\n")
}

// buildSpanTree creates a tree from flat spans and returns a DFS-flattened list with depth info.
func buildSpanTree(spans []client.CLISpan) []*spanNode {
	if len(spans) == 0 {
		return nil
	}

	// Create nodes map by SpanID
	nodeMap := make(map[string]*spanNode, len(spans))
	for _, s := range spans {
		nodeMap[s.SpanID] = &spanNode{span: s}
	}

	// Build parent-child relationships
	var roots []*spanNode
	for _, s := range spans {
		node := nodeMap[s.SpanID]
		if s.ParentSpanID != nil && *s.ParentSpanID != "" {
			if parent, ok := nodeMap[*s.ParentSpanID]; ok {
				parent.children = append(parent.children, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	// Sort children by start time
	sortChildren(roots)

	// Sort roots by start time
	sort.Slice(roots, func(i, j int) bool {
		return roots[i].span.StartTime < roots[j].span.StartTime
	})

	// Flatten via DFS
	var flat []*spanNode
	for i, root := range roots {
		root.isLast = i == len(roots)-1
		flattenTree(root, 0, &flat)
	}

	return flat
}

func sortChildren(nodes []*spanNode) {
	for _, node := range nodes {
		if len(node.children) > 0 {
			sort.Slice(node.children, func(i, j int) bool {
				return node.children[i].span.StartTime < node.children[j].span.StartTime
			})
			// Mark last child
			for i, child := range node.children {
				child.isLast = i == len(node.children)-1
			}
			sortChildren(node.children)
		}
	}
}

func flattenTree(node *spanNode, depth int, result *[]*spanNode) {
	node.depth = depth
	*result = append(*result, node)
	for _, child := range node.children {
		flattenTree(child, depth+1, result)
	}
}

func (m tracesModel) renderWaterfallRows(nodes []*spanNode, trace client.CLITrace, termWidth int) []string {
	if len(nodes) == 0 {
		return []string{lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("No spans found")}
	}

	// Parse trace start time for offset calculation
	traceStart := parseSpanTime(trace.StartTime)
	traceDurMs := trace.DurationMs
	if traceDurMs <= 0 {
		traceDurMs = 1 // avoid division by zero
	}

	// Calculate column widths
	// Layout: " INDENT STATUS OP (SVC) DURATION | BAR "
	maxIndent := 0
	for _, n := range nodes {
		indent := n.depth*2 + 3 // tree chars
		if indent > maxIndent {
			maxIndent = indent
		}
	}

	barWidth := 30
	textCols := maxIndent + 3 + 30 + 10 // indent + status + op/svc + duration
	if termWidth > textCols+20 {
		barWidth = termWidth - textCols - 4
	}
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 60 {
		barWidth = 60
	}

	var rows []string
	for _, node := range nodes {
		span := node.span

		// Tree indent with connectors
		indent := buildTreeIndent(node)

		// Status icon
		var statusIcon string
		if span.StatusCode == "ERROR" || span.StatusCode == "error" {
			statusIcon = lipgloss.NewStyle().Foreground(cDanger).Render("\u25cf")
		} else {
			statusIcon = lipgloss.NewStyle().Foreground(cSuccess).Render("\u25cf")
		}

		// Operation name (bold) and service (dim)
		opStyle := lipgloss.NewStyle().Foreground(cText).Bold(true)
		svcStyle := lipgloss.NewStyle().Foreground(cDim)
		kindStyle := lipgloss.NewStyle().Foreground(cDim)

		opName := trunc(span.OperationName, 25)
		svcName := svcStyle.Render("(" + trunc(span.ServiceName, 15) + ")")

		kindTag := ""
		if span.Kind != "" && span.Kind != "SPAN_KIND_UNSPECIFIED" {
			shortKind := strings.TrimPrefix(span.Kind, "SPAN_KIND_")
			kindTag = " " + kindStyle.Render("["+shortKind+"]")
		}

		// Duration
		durStr := fmtTraceDuration(span.DurationMs)
		durStyle := lipgloss.NewStyle().Foreground(cText)

		// Build the duration bar
		spanStart := parseSpanTime(span.StartTime)
		offsetMs := int(spanStart.Sub(traceStart).Milliseconds())
		if offsetMs < 0 {
			offsetMs = 0
		}

		bar := renderDurationBar(offsetMs, span.DurationMs, traceDurMs, barWidth, span.StatusCode)

		// Assemble row
		row := fmt.Sprintf(" %s %s %s %s%s %8s %s",
			indent, statusIcon, opStyle.Render(opName), svcName, kindTag, durStyle.Render(durStr), bar)

		rows = append(rows, row)
	}

	return rows
}

func buildTreeIndent(node *spanNode) string {
	if node.depth == 0 {
		return ""
	}

	var prefix string
	if node.isLast {
		prefix = "\u2514\u2500 " // corner connector
	} else {
		prefix = "\u251c\u2500 " // tee connector
	}

	// Add depth indentation
	indent := strings.Repeat("  ", node.depth-1)
	return lipgloss.NewStyle().Foreground(cDim).Render(indent + prefix)
}

func renderDurationBar(offsetMs, durationMs, totalMs, barWidth int, statusCode string) string {
	if totalMs <= 0 || barWidth <= 0 {
		return ""
	}

	// Calculate positions
	offsetFrac := float64(offsetMs) / float64(totalMs)
	durFrac := float64(durationMs) / float64(totalMs)

	offsetChars := int(offsetFrac * float64(barWidth))
	durChars := int(durFrac * float64(barWidth))
	if durChars < 1 {
		durChars = 1
	}
	if offsetChars+durChars > barWidth {
		durChars = barWidth - offsetChars
	}
	if durChars < 0 {
		durChars = 0
	}
	emptyAfter := barWidth - offsetChars - durChars
	if emptyAfter < 0 {
		emptyAfter = 0
	}

	emptyStyle := lipgloss.NewStyle().Foreground(cDim)
	var fillStyle lipgloss.Style
	if statusCode == "ERROR" || statusCode == "error" {
		fillStyle = lipgloss.NewStyle().Foreground(cDanger)
	} else {
		fillStyle = lipgloss.NewStyle().Foreground(cBrand)
	}

	bar := emptyStyle.Render(strings.Repeat("\u2591", offsetChars)) +
		fillStyle.Render(strings.Repeat("\u2588", durChars)) +
		emptyStyle.Render(strings.Repeat("\u2591", emptyAfter))

	return bar
}

func parseSpanTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339, s)
		if err != nil {
			return time.Time{}
		}
	}
	return t
}

func truncID(id string) string {
	if len(id) > 16 {
		return id[:16] + "..."
	}
	return id
}

// -- List View rendering --

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
		cfg, err := config.ReadWithFallback(EnvFlag)
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
