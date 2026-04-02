package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Stream live traces in real-time",
	Long: `Stream live traces from your services in real-time using Server-Sent Events.

Traces appear as they are ingested, with color-coded status and duration.

Key bindings:
  j/k or arrows   Scroll through log history
  g/G              Jump to top/bottom
  q or Ctrl+C      Quit

Examples:
  tracekit logs
  tracekit logs --service my-api
  tracekit logs --errors
  tracekit logs --service my-api --errors`,
	RunE: runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
	logsCmd.Flags().String("api-key", "", "TraceKit API key (overrides .env)")
	logsCmd.Flags().String("url", "", "API base URL")
	logsCmd.Flags().Bool("dev", false, "")
	logsCmd.Flags().MarkHidden("dev")
	logsCmd.Flags().String("service", "", "Filter by service name")
	logsCmd.Flags().Bool("errors", false, "Show only error traces")
}

// -- Messages --

type traceReceivedMsg struct {
	trace client.CLITrace
}

type streamErrorMsg struct {
	err error
}

type streamStartedMsg struct {
	tracesCh <-chan client.CLITrace
	errCh    <-chan error
}

// -- Log line --

type logLine struct {
	text      string
	timestamp time.Time
}

// -- Model --

const maxLogLines = 1000

type logsModel struct {
	client      *client.Client
	service     string
	errorsOnly  bool
	lines       []logLine
	scrollOff   int // 0 = pinned to bottom, >0 = scrolled up N lines from bottom
	width       int
	height      int
	connected   bool
	err         error
	tracesCh    <-chan client.CLITrace
	errCh       <-chan error
	ctx         context.Context
	cancel      context.CancelFunc
	traceCount  int
	quitting    bool
}

func (m logsModel) Init() tea.Cmd {
	return func() tea.Msg {
		tracesCh, errCh, err := m.client.StreamTraces(m.ctx, m.service, m.errorsOnly)
		if err != nil {
			return streamErrorMsg{err: err}
		}
		return streamStartedMsg{tracesCh: tracesCh, errCh: errCh}
	}
}

// listenForTraces returns a command that waits for the next trace or error from the SSE channels.
func listenForTraces(tracesCh <-chan client.CLITrace, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case trace, ok := <-tracesCh:
			if !ok {
				return streamErrorMsg{err: fmt.Errorf("stream closed")}
			}
			return traceReceivedMsg{trace: trace}
		case err, ok := <-errCh:
			if !ok {
				return streamErrorMsg{err: fmt.Errorf("stream closed")}
			}
			return streamErrorMsg{err: err}
		}
	}
}

func (m logsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case streamStartedMsg:
		m.connected = true
		m.tracesCh = msg.tracesCh
		m.errCh = msg.errCh
		return m, listenForTraces(m.tracesCh, m.errCh)

	case traceReceivedMsg:
		m.traceCount++
		line := formatTraceLine(msg.trace, m.width)
		m.lines = append(m.lines, line)
		if len(m.lines) > maxLogLines {
			m.lines = m.lines[len(m.lines)-maxLogLines:]
		}
		return m, listenForTraces(m.tracesCh, m.errCh)

	case streamErrorMsg:
		m.err = msg.err
		m.connected = false
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			m.cancel()
			return m, tea.Quit
		case "up", "k":
			m.scrollOff++
			maxScroll := len(m.lines) - m.viewableLines()
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.scrollOff > maxScroll {
				m.scrollOff = maxScroll
			}
		case "down", "j":
			if m.scrollOff > 0 {
				m.scrollOff--
			}
		case "g":
			m.scrollOff = len(m.lines) - m.viewableLines()
			if m.scrollOff < 0 {
				m.scrollOff = 0
			}
		case "G":
			m.scrollOff = 0
		}
	}
	return m, nil
}

func (m logsModel) viewableLines() int {
	h := m.height - 3 // header + footer + padding
	if h < 1 {
		h = 1
	}
	return h
}

func (m logsModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 100
	}
	h := m.height
	if h == 0 {
		h = 30
	}

	var b strings.Builder

	// -- Header --
	b.WriteString(m.renderLogHeader(w))
	b.WriteString("\n")

	// -- Log lines area --
	viewH := h - 3 // header(1) + footer(1) + margin(1)
	if viewH < 1 {
		viewH = 1
	}

	if !m.connected && m.err != nil {
		errMsg := lipgloss.NewStyle().Foreground(cDanger).Padding(1, 2).Render(
			fmt.Sprintf("Connection error: %s", m.err.Error()))
		b.WriteString(errMsg)
		b.WriteString("\n")
	} else if !m.connected && len(m.lines) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(1, 2).Render("Connecting..."))
		b.WriteString("\n")
	} else if len(m.lines) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(1, 2).Render("Waiting for traces..."))
		b.WriteString("\n")
	} else {
		// Calculate visible window
		totalLines := len(m.lines)
		endIdx := totalLines - m.scrollOff
		if endIdx < 0 {
			endIdx = 0
		}
		startIdx := endIdx - viewH
		if startIdx < 0 {
			startIdx = 0
		}

		visible := m.lines[startIdx:endIdx]
		for _, line := range visible {
			b.WriteString(line.text)
			b.WriteString("\n")
		}

		// Fill remaining empty lines
		for i := len(visible); i < viewH; i++ {
			b.WriteString("\n")
		}
	}

	// -- Footer --
	b.WriteString(m.renderLogFooter())

	return b.String()
}

func (m logsModel) renderLogHeader(w int) string {
	title := lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render("Live Trace Tail")
	sep := lipgloss.NewStyle().Foreground(cDim).Render("  ")

	var chips []string
	if m.service != "" {
		chips = append(chips, lipgloss.NewStyle().Foreground(cCyan).Render("[service: "+m.service+"]"))
	}
	if m.errorsOnly {
		chips = append(chips, lipgloss.NewStyle().Foreground(cDanger).Render("[errors only]"))
	}

	chipStr := ""
	if len(chips) > 0 {
		chipStr = sep + strings.Join(chips, " ")
	}

	return " " + title + chipStr
}

func (m logsModel) renderLogFooter() string {
	var parts []string

	parts = append(parts, lipgloss.NewStyle().Foreground(cDim).Render("q quit"))
	parts = append(parts, lipgloss.NewStyle().Foreground(cDim).Render("j/k scroll"))

	status := "streaming..."
	statusColor := cSuccess
	if !m.connected {
		status = "disconnected"
		statusColor = cDanger
	}
	parts = append(parts, lipgloss.NewStyle().Foreground(statusColor).Render(status))

	countStr := lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("%d traces", m.traceCount))
	parts = append(parts, countStr)

	if m.scrollOff > 0 {
		scrollStr := lipgloss.NewStyle().Foreground(cWarning).Render(fmt.Sprintf("scrolled +%d", m.scrollOff))
		parts = append(parts, scrollStr)
	}

	return " " + strings.Join(parts, lipgloss.NewStyle().Foreground(cDim).Render(" | "))
}

func formatTraceLine(trace client.CLITrace, termWidth int) logLine {
	now := time.Now()

	// Parse start time for display
	ts := now
	if t, err := time.Parse(time.RFC3339Nano, trace.StartTime); err == nil {
		ts = t
	} else if t, err := time.Parse(time.RFC3339, trace.StartTime); err == nil {
		ts = t
	}

	// Timestamp: HH:MM:SS
	timeStr := lipgloss.NewStyle().Foreground(cDim).Render(ts.Format("15:04:05"))

	// Status icon
	var statusIcon string
	if trace.HasError {
		statusIcon = lipgloss.NewStyle().Foreground(cDanger).Render("\u25cf")
	} else {
		statusIcon = lipgloss.NewStyle().Foreground(cSuccess).Render("\u25cf")
	}

	// Service name: left-padded to 20 chars
	svcName := trace.ServiceName
	if len(svcName) > 20 {
		svcName = svcName[:19] + "\u2026"
	}
	svcStr := lipgloss.NewStyle().Foreground(cCyan).Render(fmt.Sprintf("%-20s", svcName))

	// Operation name: bright white
	opName := trace.OperationName
	if termWidth > 0 {
		maxOp := termWidth - 55 // timestamp(8) + status(2) + svc(20) + dur(10) + gaps(15)
		if maxOp < 10 {
			maxOp = 10
		}
		if len(opName) > maxOp {
			opName = opName[:maxOp-1] + "\u2026"
		}
	}
	opStr := lipgloss.NewStyle().Foreground(cText).Render(opName)

	// Duration: colored by threshold
	durMs := trace.DurationMs
	var durStr string
	if durMs < 1000 {
		durStr = fmt.Sprintf("%dms", durMs)
	} else {
		durStr = fmt.Sprintf("%.1fs", float64(durMs)/1000)
	}

	var durColor lipgloss.TerminalColor
	if durMs < 200 {
		durColor = cSuccess
	} else if durMs <= 1000 {
		durColor = cWarning
	} else {
		durColor = cDanger
	}
	durRendered := lipgloss.NewStyle().Foreground(durColor).Render(fmt.Sprintf("%8s", durStr))

	text := fmt.Sprintf(" %s %s %s %s %s", timeStr, statusIcon, svcStr, opStr, durRendered)

	return logLine{
		text:      text,
		timestamp: ts,
	}
}

// -- Runner --

func runLogs(cmd *cobra.Command, args []string) error {
	apiKey, _ := cmd.Flags().GetString("api-key")
	customURL, _ := cmd.Flags().GetString("url")
	isDev, _ := cmd.Flags().GetBool("dev")
	service, _ := cmd.Flags().GetString("service")
	errorsOnly, _ := cmd.Flags().GetBool("errors")

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

	ctx, cancel := context.WithCancel(context.Background())

	model := logsModel{
		client:     c,
		service:    service,
		errorsOnly: errorsOnly,
		lines:      make([]logLine, 0, maxLogLines),
		ctx:        ctx,
		cancel:     cancel,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		cancel()
		return fmt.Errorf("logs error: %w", err)
	}
	return nil
}
