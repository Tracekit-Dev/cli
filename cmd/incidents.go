package cmd

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

var incidentsCmd = &cobra.Command{
	Use:   "incidents",
	Short: "Triage incidents from the unified inbox",
	Long: `Display an interactive incident triage TUI with filtering and status transitions.

Key bindings:
  j/k or arrows   Navigate incident list
  a                Acknowledge selected incident
  i                Investigate selected incident
  r                Resolve (prompts for notes)
  z                Snooze (select duration)
  s                Toggle severity filter
  t                Toggle type filter
  w                Toggle team filter
  Esc              Back / clear filter
  q or Ctrl+C      Quit`,
	RunE: runIncidents,
}

func init() {
	rootCmd.AddCommand(incidentsCmd)
	incidentsCmd.Flags().String("api-key", "", "TraceKit API key (overrides .env)")
	incidentsCmd.Flags().String("url", "", "API base URL")
	incidentsCmd.Flags().Bool("dev", false, "")
	incidentsCmd.Flags().MarkHidden("dev")
}

// -- Message types --

type incidentsLoadedMsg struct {
	items      []client.CLITriageItem
	totalCount int
}

type incidentTransitionMsg struct {
	success bool
	message string
}

type incidentTransitionErrMsg struct {
	err error
}

type incidentFlashClearMsg struct{}

// -- Commands --

func fetchTriageInbox(c *client.Client, severity, entityType, team string) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetTriageInbox(severity, entityType, "", team)
		if err != nil {
			return incidentTransitionErrMsg{err: err}
		}
		return incidentsLoadedMsg{items: resp.Items, totalCount: resp.TotalCount}
	}
}

func acknowledgeIncident(c *client.Client, itemID, entityType string) tea.Cmd {
	return func() tea.Msg {
		err := c.AcknowledgeIncident(itemID, entityType)
		if err != nil {
			return incidentTransitionErrMsg{err: err}
		}
		return incidentTransitionMsg{success: true, message: "Acknowledged"}
	}
}

func investigateIncident(c *client.Client, itemID, entityType string) tea.Cmd {
	return func() tea.Msg {
		err := c.InvestigateIncident(itemID, entityType)
		if err != nil {
			return incidentTransitionErrMsg{err: err}
		}
		return incidentTransitionMsg{success: true, message: "Investigating"}
	}
}

func resolveIncident(c *client.Client, itemID, entityType, note string) tea.Cmd {
	return func() tea.Msg {
		err := c.ResolveIncident(itemID, entityType, note)
		if err != nil {
			return incidentTransitionErrMsg{err: err}
		}
		return incidentTransitionMsg{success: true, message: "Resolved"}
	}
}

func snoozeIncident(c *client.Client, itemID, entityType, duration string) tea.Cmd {
	return func() tea.Msg {
		err := c.SnoozeIncident(itemID, entityType, duration)
		if err != nil {
			return incidentTransitionErrMsg{err: err}
		}
		return incidentTransitionMsg{success: true, message: fmt.Sprintf("Snoozed for %s", duration)}
	}
}

func clearIncidentFlash() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return incidentFlashClearMsg{}
	})
}

// -- Severity/type filter cycles --

var severityFilterCycle = []string{"", "critical", "warning", "info", "low"}
var typeFilterCycle = []string{"", "alert", "anomaly", "security", "ddos"}
var snoozeDurations = []string{"1h", "4h", "8h", "24h"}

// -- Model --

type incidentsModel struct {
	client     *client.Client
	items      []client.CLITriageItem
	cursor     int
	totalCount int
	loading    bool
	err        error
	quitting   bool

	// View state: "list", "resolve", "snooze"
	view string

	// Filters
	severityFilter string
	typeFilter     string
	teamFilter     string
	teamNames      []string // unique team names extracted from items

	// Resolve note input
	resolveInput string

	// Snooze picker
	snoozeCursor int

	// Flash message
	flash      string
	flashStyle lipgloss.Style

	width, height int
}

func (m incidentsModel) Init() tea.Cmd {
	return fetchTriageInbox(m.client, m.severityFilter, m.typeFilter, m.teamFilter)
}

func (m incidentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case incidentsLoadedMsg:
		m.loading = false
		m.items = sortTriageItems(msg.items)
		m.totalCount = msg.totalCount
		m.err = nil
		m.teamNames = extractTeamNames(m.items)
		if m.cursor >= len(m.items) {
			m.cursor = max(0, len(m.items)-1)
		}
		return m, nil

	case incidentTransitionMsg:
		m.loading = false
		m.flash = msg.message
		m.flashStyle = incidentFlashStyleForMessage(msg.message)
		m.view = "list"
		m.resolveInput = ""
		return m, tea.Batch(
			fetchTriageInbox(m.client, m.severityFilter, m.typeFilter, m.teamFilter),
			clearIncidentFlash(),
		)

	case incidentTransitionErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case incidentFlashClearMsg:
		m.flash = ""
		return m, nil

	case tea.KeyMsg:
		return m.handleIncidentKey(msg)
	}

	return m, nil
}

// -- Key handling --

func (m incidentsModel) handleIncidentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.view {
	case "resolve":
		return m.handleResolveKey(msg)
	case "snooze":
		return m.handleSnoozeKey(key)
	default:
		return m.handleListKey(key)
	}
}

func (m incidentsModel) handleListKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if len(m.items) > 0 {
			m.cursor = (m.cursor + 1) % len(m.items)
		}

	case "k", "up":
		if len(m.items) > 0 {
			m.cursor = (m.cursor - 1 + len(m.items)) % len(m.items)
		}

	case "s":
		// Cycle severity filter
		m.severityFilter = nextInCycle(severityFilterCycle, m.severityFilter)
		m.loading = true
		m.cursor = 0
		return m, fetchTriageInbox(m.client, m.severityFilter, m.typeFilter, m.teamFilter)

	case "t":
		// Cycle type filter
		m.typeFilter = nextInCycle(typeFilterCycle, m.typeFilter)
		m.loading = true
		m.cursor = 0
		return m, fetchTriageInbox(m.client, m.severityFilter, m.typeFilter, m.teamFilter)

	case "w":
		// Cycle team filter through unique team names
		if len(m.teamNames) > 0 {
			m.teamFilter = nextInCycle(append([]string{""}, m.teamNames...), m.teamFilter)
			m.loading = true
			m.cursor = 0
			return m, fetchTriageInbox(m.client, m.severityFilter, m.typeFilter, m.teamFilter)
		}

	case "esc":
		// Clear all filters
		if m.severityFilter != "" || m.typeFilter != "" || m.teamFilter != "" {
			m.severityFilter = ""
			m.typeFilter = ""
			m.teamFilter = ""
			m.loading = true
			m.cursor = 0
			return m, fetchTriageInbox(m.client, "", "", "")
		}

	case "a":
		// Acknowledge
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if canTransition(item.Status, "acknowledge") {
				m.loading = true
				return m, acknowledgeIncident(m.client, item.ID, item.EntityType)
			}
		}

	case "i":
		// Investigate
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if canTransition(item.Status, "investigate") {
				m.loading = true
				return m, investigateIncident(m.client, item.ID, item.EntityType)
			}
		}

	case "r":
		// Resolve - switch to resolve view
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if canTransition(item.Status, "resolve") {
				m.view = "resolve"
				m.resolveInput = ""
			}
		}

	case "z":
		// Snooze - switch to snooze view
		if len(m.items) > 0 && m.cursor < len(m.items) {
			item := m.items[m.cursor]
			if canTransition(item.Status, "snooze") {
				m.view = "snooze"
				m.snoozeCursor = 0
			}
		}
	}

	return m, nil
}

func (m incidentsModel) handleResolveKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.view = "list"
		m.resolveInput = ""
		return m, nil

	case "enter":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			m.loading = true
			return m, resolveIncident(m.client, item.ID, item.EntityType, m.resolveInput)
		}

	case "backspace":
		if len(m.resolveInput) > 0 {
			m.resolveInput = m.resolveInput[:len(m.resolveInput)-1]
		}

	default:
		if len(key) == 1 {
			m.resolveInput += key
		} else if key == "space" {
			m.resolveInput += " "
		}
	}

	return m, nil
}

func (m incidentsModel) handleSnoozeKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.view = "list"
		return m, nil

	case "j", "down":
		if m.snoozeCursor < len(snoozeDurations)-1 {
			m.snoozeCursor++
		}

	case "k", "up":
		if m.snoozeCursor > 0 {
			m.snoozeCursor--
		}

	case "enter":
		if m.cursor < len(m.items) {
			item := m.items[m.cursor]
			duration := snoozeDurations[m.snoozeCursor]
			m.loading = true
			return m, snoozeIncident(m.client, item.ID, item.EntityType, duration)
		}

	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

// -- View --

func (m incidentsModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 100
	}

	switch m.view {
	case "resolve":
		return m.renderResolveView(w)
	case "snooze":
		return m.renderSnoozeView(w)
	default:
		return m.renderIncidentListView(w)
	}
}

// -- List View --

func (m incidentsModel) renderIncidentListView(w int) string {
	if m.loading && len(m.items) == 0 {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading incidents...") + "\n"
	}

	if m.err != nil {
		return "\n" + lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(fmt.Sprintf("Error: %s", m.err.Error())) + "\n"
	}

	var b strings.Builder

	// Filter bar
	filterBar := m.renderFilterBar()
	if filterBar != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(filterBar))
		b.WriteString("\n")
	}

	if len(m.items) == 0 {
		content := lipgloss.NewStyle().Foreground(cMuted).Render("No incidents found. Adjust filters or check back later.")
		title := "Triage Inbox (0)"
		b.WriteString("\n")
		b.WriteString(titledPanel(title, content, w-2))
		b.WriteString("\n\n")
		footer := lipgloss.NewStyle().Foreground(cDim).Render(" s severity | t type | w team | Esc clear | q quit")
		b.WriteString(footer)
		b.WriteString("\n")
		return b.String()
	}

	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-4s %-2s %-30s %-14s %-12s", "SEV", "", "TITLE", "STATUS", "TEAM")
	rows = append(rows, headerStyle.Render(hdr))

	// Calculate visible window
	visibleRows := m.height - 12
	if visibleRows < 5 {
		visibleRows = 5
	}

	startIdx := 0
	if m.cursor >= visibleRows {
		startIdx = m.cursor - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	for idx := startIdx; idx < endIdx; idx++ {
		item := m.items[idx]
		isSelected := idx == m.cursor

		sev := incidentSeverityBadge(item.Severity)
		typeIcon := incidentTypeIcon(item.EntityType)
		title := trunc(item.Title, 30)
		status := incidentStatusBadge(item.Status)
		teamName := "---"
		if item.TeamName != nil {
			teamName = trunc(*item.TeamName, 12)
		}

		row := fmt.Sprintf("  %s %s %-30s %s  %-12s", sev, typeIcon, title, status, teamName)

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
	title := fmt.Sprintf("Triage Inbox (%d)", m.totalCount)
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n")

	// Scroll indicator
	if len(m.items) > visibleRows {
		scrollInfo := fmt.Sprintf(" [%d/%d]", m.cursor+1, len(m.items))
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(scrollInfo))
		b.WriteString("\n")
	}

	// Flash message
	if m.flash != "" {
		b.WriteString("\n")
		b.WriteString(m.flashStyle.Padding(0, 1).Render(m.flash))
		b.WriteString("\n")
	}

	// Context-sensitive help bar
	b.WriteString("\n")
	helpBar := m.buildHelpBar()
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(helpBar))
	b.WriteString("\n")

	return b.String()
}

// -- Filter bar --

func (m incidentsModel) renderFilterBar() string {
	var parts []string
	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Background(lipgloss.Color("#374151")).
		Padding(0, 1)

	if m.severityFilter != "" {
		parts = append(parts, filterStyle.Render(fmt.Sprintf("Severity: %s", m.severityFilter)))
	}
	if m.typeFilter != "" {
		parts = append(parts, filterStyle.Render(fmt.Sprintf("Type: %s", m.typeFilter)))
	}
	if m.teamFilter != "" {
		parts = append(parts, filterStyle.Render(fmt.Sprintf("Team: %s", m.teamFilter)))
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// -- Resolve View --

func (m incidentsModel) renderResolveView(w int) string {
	var b strings.Builder

	// Render dimmed list
	dimStyle := lipgloss.NewStyle().Foreground(cDim)

	var rows []string
	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-4s %-2s %-30s %-14s %-12s", "SEV", "", "TITLE", "STATUS", "TEAM")
	rows = append(rows, headerStyle.Render(hdr))

	maxVisible := 8
	if maxVisible > len(m.items) {
		maxVisible = len(m.items)
	}

	startIdx := 0
	if m.cursor >= maxVisible {
		startIdx = m.cursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > len(m.items) {
		endIdx = len(m.items)
	}

	for idx := startIdx; idx < endIdx; idx++ {
		item := m.items[idx]
		isSelected := idx == m.cursor

		title := trunc(item.Title, 30)
		teamName := "---"
		if item.TeamName != nil {
			teamName = trunc(*item.TeamName, 12)
		}

		row := fmt.Sprintf("  %-4s %-2s %-30s %-14s %-12s",
			strings.ToUpper(item.Severity[:4]),
			incidentTypeIconPlain(item.EntityType),
			title,
			item.Status,
			teamName,
		)

		if isSelected {
			row = lipgloss.NewStyle().
				Background(cBrand).
				Foreground(lipgloss.Color("#ffffff")).
				Width(w - 4).
				Render(row)
		} else {
			row = dimStyle.Render(row)
		}
		rows = append(rows, row)
	}

	content := strings.Join(rows, "\n")
	title := "Triage Inbox - Resolve"
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n\n")

	// Text input bar
	cursor := lipgloss.NewStyle().Foreground(cBrand).Render("|")
	inputLabel := lipgloss.NewStyle().Foreground(cText).Bold(true).Render("Resolution notes: ")
	inputValue := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(m.resolveInput)
	b.WriteString(lipgloss.NewStyle().Padding(0, 1).Render(inputLabel + inputValue + cursor))
	b.WriteString("\n\n")

	footer := lipgloss.NewStyle().Foreground(cDim).Render(" Enter confirm | Esc cancel")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Snooze View --

func (m incidentsModel) renderSnoozeView(w int) string {
	var b strings.Builder

	// Show the selected item info
	if m.cursor < len(m.items) {
		item := m.items[m.cursor]
		itemInfo := fmt.Sprintf("Snoozing: %s", trunc(item.Title, 50))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 1).Render(itemInfo))
		b.WriteString("\n\n")
	}

	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 1).Render("Snooze for:"))
	b.WriteString("\n\n")

	for i, dur := range snoozeDurations {
		label := formatSnoozeDuration(dur)
		if i == m.snoozeCursor {
			prefix := lipgloss.NewStyle().Foreground(cBrand).Render("> ")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 2).Render(prefix + label))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	footer := lipgloss.NewStyle().Foreground(cDim).Render(" j/k select | Enter confirm | Esc cancel")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Context-sensitive help bar --

func (m incidentsModel) buildHelpBar() string {
	var actions []string

	if len(m.items) > 0 && m.cursor < len(m.items) {
		status := m.items[m.cursor].Status
		switch status {
		case "open":
			actions = append(actions, "a ack", "i investigate", "r resolve", "z snooze")
		case "acknowledged":
			actions = append(actions, "i investigate", "r resolve", "z snooze")
		case "investigating":
			actions = append(actions, "r resolve", "z snooze")
		case "snoozed":
			actions = append(actions, "a ack", "i investigate", "r resolve")
		case "resolved":
			// No transitions
		}
	}

	actions = append(actions, "s severity", "t type", "w team", "Esc clear", "q quit")
	return " " + strings.Join(actions, " | ")
}

// -- Helpers --

func sortTriageItems(items []client.CLITriageItem) []client.CLITriageItem {
	sort.SliceStable(items, func(i, j int) bool {
		iScore := severityScore(items[i].Severity)
		jScore := severityScore(items[j].Severity)
		if iScore != jScore {
			return iScore > jScore
		}
		return items[i].Timestamp.After(items[j].Timestamp)
	})
	return items
}

func severityScore(severity string) int {
	switch severity {
	case "critical":
		return 4
	case "warning":
		return 3
	case "info":
		return 2
	case "low":
		return 1
	default:
		return 0
	}
}

func extractTeamNames(items []client.CLITriageItem) []string {
	seen := make(map[string]bool)
	var names []string
	for _, item := range items {
		if item.TeamName != nil && !seen[*item.TeamName] {
			seen[*item.TeamName] = true
			names = append(names, *item.TeamName)
		}
	}
	sort.Strings(names)
	return names
}

func nextInCycle(cycle []string, current string) string {
	for i, v := range cycle {
		if v == current {
			return cycle[(i+1)%len(cycle)]
		}
	}
	return cycle[0]
}

func canTransition(status, action string) bool {
	switch action {
	case "acknowledge":
		return status == "open" || status == "snoozed"
	case "investigate":
		return status == "open" || status == "acknowledged" || status == "snoozed"
	case "resolve":
		return status == "open" || status == "acknowledged" || status == "investigating" || status == "snoozed"
	case "snooze":
		return status == "open" || status == "acknowledged" || status == "investigating"
	}
	return false
}

func incidentSeverityBadge(severity string) string {
	switch severity {
	case "critical":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true).Render("CRIT")
	case "warning":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true).Render("WARN")
	case "info":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("INFO")
	case "low":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("LOW ")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render("--- ")
	}
}

func incidentStatusBadge(status string) string {
	switch status {
	case "open":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Render("OPEN         ")
	case "acknowledged":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Render("ACKNOWLEDGED ")
	case "investigating":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12")).Render("INVESTIGATING")
	case "resolved":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#10b981")).Render("RESOLVED     ")
	case "snoozed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("SNOOZED      ")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render(fmt.Sprintf("%-13s", status))
	}
}

func incidentTypeIcon(entityType string) string {
	switch entityType {
	case "alert":
		return lipgloss.NewStyle().Foreground(cWarning).Render("A")
	case "anomaly":
		return lipgloss.NewStyle().Foreground(cCyan).Render("N")
	case "security":
		return lipgloss.NewStyle().Foreground(cDanger).Render("S")
	case "ddos":
		return lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("D")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render("?")
	}
}

func incidentTypeIconPlain(entityType string) string {
	switch entityType {
	case "alert":
		return "A"
	case "anomaly":
		return "N"
	case "security":
		return "S"
	case "ddos":
		return "D"
	default:
		return "?"
	}
}

func incidentFlashStyleForMessage(message string) lipgloss.Style {
	switch {
	case strings.Contains(message, "Acknowledged"):
		return lipgloss.NewStyle().Foreground(cSuccess)
	case strings.Contains(message, "Investigating"):
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	case strings.Contains(message, "Resolved"):
		return lipgloss.NewStyle().Foreground(cSuccess)
	case strings.Contains(message, "Snoozed"):
		return lipgloss.NewStyle().Foreground(cWarning)
	default:
		return lipgloss.NewStyle().Foreground(cText)
	}
}

func formatSnoozeDuration(d string) string {
	switch d {
	case "1h":
		return "1 hour"
	case "4h":
		return "4 hours"
	case "8h":
		return "8 hours"
	case "24h":
		return "24 hours"
	default:
		return d
	}
}

// -- Runner --

func runIncidents(cmd *cobra.Command, args []string) error {
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

	model := incidentsModel{
		client:          c,
		view:            "list",
		loading:         true,
		snoozeCursor:    0,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("incidents error: %w", err)
	}
	return nil
}
