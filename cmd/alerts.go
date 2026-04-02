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
)

var alertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "Manage alert rules and view firing history",
	Long: `Display an interactive alert management TUI with rule list, create wizard, and history.

Key bindings:
  j/k or arrows   Navigate alert list
  n                Create new alert rule
  d                Delete selected rule
  t                Toggle enabled/disabled
  h                View alert history
  r                Refresh
  q or Ctrl+C      Quit`,
	RunE: runAlerts,
}

func init() {
	rootCmd.AddCommand(alertsCmd)
	alertsCmd.Flags().String("url", "", "API base URL")
	alertsCmd.Flags().Bool("dev", false, "")
	alertsCmd.Flags().MarkHidden("dev")
}

// -- Messages --

type alertRulesLoadedMsg struct {
	rules []client.CLIAlertRule
}

type alertHistoryLoadedMsg struct {
	history []client.CLIAlertHistory
}

type alertChannelsLoadedMsg struct {
	channels []client.CLIChannel
}

type alertRuleCreatedMsg struct {
	rule *client.CLIAlertRule
}

type alertRuleDeletedMsg struct{}

type alertRuleToggledMsg struct{}

type alertErrMsg struct {
	err error
}

type alertStatusClearMsg struct{}

// -- Commands --

func fetchAlertRules(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetAlertRules()
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertRulesLoadedMsg{rules: resp.Rules}
	}
}

func fetchAlertHistory(c *client.Client, ruleID string) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetAlertHistory(ruleID)
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertHistoryLoadedMsg{history: resp.History}
	}
}

func fetchAlertChannels(c *client.Client) tea.Cmd {
	return func() tea.Msg {
		resp, err := c.GetChannels()
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertChannelsLoadedMsg{channels: resp.Channels}
	}
}

func createAlertRule(c *client.Client, req client.CreateAlertRuleRequest) tea.Cmd {
	return func() tea.Msg {
		rule, err := c.CreateAlertRule(req)
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertRuleCreatedMsg{rule: rule}
	}
}

func deleteAlertRule(c *client.Client, ruleID string) tea.Cmd {
	return func() tea.Msg {
		err := c.DeleteAlertRule(ruleID)
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertRuleDeletedMsg{}
	}
}

func toggleAlertRule(c *client.Client, ruleID string, enabled bool) tea.Cmd {
	return func() tea.Msg {
		err := c.ToggleAlertRule(ruleID, enabled)
		if err != nil {
			return alertErrMsg{err: err}
		}
		return alertRuleToggledMsg{}
	}
}

func clearAlertStatus() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return alertStatusClearMsg{}
	})
}

// -- Metric/Operator lists --

var alertMetrics = []string{"error_rate", "latency", "throughput", "health_score"}
var alertOperators = []string{">", "<", ">=", "<="}
var alertSeverities = []string{"warning", "critical", "info"}

// -- Model --

type alertsModel struct {
	apiClient *client.Client
	width     int
	height    int
	nav       navModel
	navTarget string

	// List view
	rules    []client.CLIAlertRule
	cursor   int
	loading  bool
	err      error
	quitting bool
	view     string // "list", "history", "create", "confirm-delete"

	// History view
	history       []client.CLIAlertHistory
	historyCursor int

	// Create wizard
	wizardStep      int // 0=name, 1=metric, 2=operator+threshold, 3=channels, 4=confirm
	wizardName      string
	wizardMetric    int    // cursor in metric list
	wizardOp        int    // cursor in operator list
	wizardThreshold string // text input
	wizardFocus     int    // 0=operator, 1=threshold in step 2
	channels        []client.CLIChannel
	channelSelected []bool // multi-select
	wizardSeverity  int    // cursor in severity list
	wizardErr       string

	// Status message (flash)
	statusMsg string
}

func (m alertsModel) Init() tea.Cmd {
	return fetchAlertRules(m.apiClient)
}

func (m alertsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NavSwitchMsg:
		m.navTarget = msg.Command
		m.quitting = true
		return m, tea.Quit
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case alertRulesLoadedMsg:
		m.loading = false
		m.rules = sortAlertRules(msg.rules)
		m.err = nil
		if m.cursor >= len(m.rules) {
			m.cursor = max(0, len(m.rules)-1)
		}
		return m, nil

	case alertHistoryLoadedMsg:
		m.loading = false
		m.history = msg.history
		m.historyCursor = 0
		m.view = "history"
		return m, nil

	case alertChannelsLoadedMsg:
		m.channels = msg.channels
		m.channelSelected = make([]bool, len(msg.channels))
		m.loading = false
		return m, nil

	case alertRuleCreatedMsg:
		m.loading = false
		m.view = "list"
		m.statusMsg = "Rule created"
		m.resetWizard()
		return m, tea.Batch(fetchAlertRules(m.apiClient), clearAlertStatus())

	case alertRuleDeletedMsg:
		m.loading = false
		m.view = "list"
		m.statusMsg = "Rule deleted"
		return m, tea.Batch(fetchAlertRules(m.apiClient), clearAlertStatus())

	case alertRuleToggledMsg:
		m.loading = false
		return m, tea.Batch(fetchAlertRules(m.apiClient), clearAlertStatus())

	case alertStatusClearMsg:
		m.statusMsg = ""
		return m, nil

	case alertErrMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Nav overlay gets first crack at keys
		nav, consumed, navCmd := m.nav.HandleNavKey(msg)
		m.nav = nav
		if consumed {
			return m, navCmd
		}
		return m.handleAlertKey(msg)
	}

	return m, nil
}

func (m *alertsModel) resetWizard() {
	m.wizardStep = 0
	m.wizardName = ""
	m.wizardMetric = 0
	m.wizardOp = 0
	m.wizardThreshold = ""
	m.wizardFocus = 0
	m.wizardSeverity = 0
	m.wizardErr = ""
	m.channels = nil
	m.channelSelected = nil
}

// -- Key Handling --

func (m alertsModel) handleAlertKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.view {
	case "history":
		return m.handleHistoryKey(key)
	case "create":
		return m.handleCreateKey(msg)
	case "confirm-delete":
		return m.handleDeleteKey(key)
	default:
		return m.handleListKey(key)
	}
}

func (m alertsModel) handleListKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "j", "down":
		if m.cursor < len(m.rules)-1 {
			m.cursor++
		}

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}

	case "n":
		m.view = "create"
		m.resetWizard()
		m.loading = true
		return m, fetchAlertChannels(m.apiClient)

	case "d":
		if len(m.rules) > 0 && m.cursor < len(m.rules) {
			m.view = "confirm-delete"
		}

	case "t":
		if len(m.rules) > 0 && m.cursor < len(m.rules) {
			rule := m.rules[m.cursor]
			newEnabled := !rule.Enabled
			if newEnabled {
				m.statusMsg = "Rule enabled"
			} else {
				m.statusMsg = "Rule disabled"
			}
			m.loading = true
			return m, toggleAlertRule(m.apiClient, rule.ID, newEnabled)
		}

	case "h":
		if len(m.rules) > 0 && m.cursor < len(m.rules) {
			rule := m.rules[m.cursor]
			m.loading = true
			return m, fetchAlertHistory(m.apiClient, rule.ID)
		}

	case "r":
		m.loading = true
		return m, fetchAlertRules(m.apiClient)
	}

	return m, nil
}

func (m alertsModel) handleHistoryKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.view = "list"
		m.history = nil
		m.historyCursor = 0

	case "j", "down":
		if m.historyCursor < len(m.history)-1 {
			m.historyCursor++
		}

	case "k", "up":
		if m.historyCursor > 0 {
			m.historyCursor--
		}

	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	}

	return m, nil
}

func (m alertsModel) handleCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Esc cancels at any step
	if key == "esc" {
		m.view = "list"
		m.resetWizard()
		return m, nil
	}

	switch m.wizardStep {
	case 0: // Name input
		return m.handleWizardNameKey(msg)
	case 1: // Metric selection
		return m.handleWizardMetricKey(key)
	case 2: // Operator + threshold
		return m.handleWizardConditionKey(msg)
	case 3: // Channel selection
		return m.handleWizardChannelKey(key)
	case 4: // Confirm
		return m.handleWizardConfirmKey(key)
	}

	return m, nil
}

func (m alertsModel) handleWizardNameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		if strings.TrimSpace(m.wizardName) == "" {
			m.wizardErr = "Name cannot be empty"
			return m, nil
		}
		m.wizardErr = ""
		m.wizardStep = 1
	case "backspace":
		if len(m.wizardName) > 0 {
			m.wizardName = m.wizardName[:len(m.wizardName)-1]
		}
	default:
		if len(key) == 1 {
			m.wizardName += key
		} else if key == "space" {
			m.wizardName += " "
		}
	}

	return m, nil
}

func (m alertsModel) handleWizardMetricKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if m.wizardMetric < len(alertMetrics)-1 {
			m.wizardMetric++
		}
	case "k", "up":
		if m.wizardMetric > 0 {
			m.wizardMetric--
		}
	case "enter":
		m.wizardStep = 2
	}

	return m, nil
}

func (m alertsModel) handleWizardConditionKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "tab":
		m.wizardFocus = 1 - m.wizardFocus // toggle between 0 and 1
	case "enter":
		// Validate threshold
		val, err := strconv.ParseFloat(m.wizardThreshold, 64)
		if err != nil || val <= 0 {
			m.wizardErr = "Threshold must be a positive number"
			return m, nil
		}
		m.wizardErr = ""
		m.wizardStep = 3
	default:
		if m.wizardFocus == 0 {
			// Operator selection
			switch key {
			case "j", "down":
				if m.wizardOp < len(alertOperators)-1 {
					m.wizardOp++
				}
			case "k", "up":
				if m.wizardOp > 0 {
					m.wizardOp--
				}
			}
		} else {
			// Threshold text input
			switch key {
			case "backspace":
				if len(m.wizardThreshold) > 0 {
					m.wizardThreshold = m.wizardThreshold[:len(m.wizardThreshold)-1]
				}
			default:
				if len(key) == 1 && (key[0] >= '0' && key[0] <= '9' || key[0] == '.') {
					m.wizardThreshold += key
				}
			}
		}
	}

	return m, nil
}

func (m alertsModel) handleWizardChannelKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "j", "down":
		if len(m.channels) > 0 && m.wizardSeverity < len(m.channels)-1 {
			m.wizardSeverity++
		}
	case "k", "up":
		if m.wizardSeverity > 0 {
			m.wizardSeverity--
		}
	case " ":
		if len(m.channelSelected) > 0 && m.wizardSeverity < len(m.channelSelected) {
			m.channelSelected[m.wizardSeverity] = !m.channelSelected[m.wizardSeverity]
		}
	case "enter":
		m.wizardStep = 4
		m.wizardSeverity = 0 // reset for severity if we add it later
	}

	return m, nil
}

func (m alertsModel) handleWizardConfirmKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		threshold, _ := strconv.ParseFloat(m.wizardThreshold, 64)

		var channelIDs []string
		for i, ch := range m.channels {
			if i < len(m.channelSelected) && m.channelSelected[i] {
				channelIDs = append(channelIDs, ch.ID)
			}
		}

		req := client.CreateAlertRuleRequest{
			Name:       m.wizardName,
			AlertType:  "metric",
			ScopeType:  "global",
			Metric:     alertMetrics[m.wizardMetric],
			Operator:   alertOperators[m.wizardOp],
			Threshold:  threshold,
			TimeWindow: 5,
			Cooldown:   5,
			Severity:   "warning",
			ChannelIDs: channelIDs,
		}

		m.loading = true
		return m, createAlertRule(m.apiClient, req)
	}

	return m, nil
}

func (m alertsModel) handleDeleteKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		if m.cursor < len(m.rules) {
			rule := m.rules[m.cursor]
			m.loading = true
			return m, deleteAlertRule(m.apiClient, rule.ID)
		}
	case "n", "N", "esc":
		m.view = "list"
	}

	return m, nil
}

// -- View --

func (m alertsModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 100
	}

	switch m.view {
	case "history":
		return m.renderHistoryView(w)
	case "create":
		return m.renderCreateView(w)
	case "confirm-delete":
		return m.renderDeleteView(w)
	default:
		return m.renderAlertListView(w)
	}
}

// -- List View --

func (m alertsModel) renderAlertListView(w int) string {
	if m.loading && len(m.rules) == 0 {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading alert rules...") + "\n"
	}

	if m.err != nil {
		return "\n" + lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(fmt.Sprintf("Error: %s", m.err.Error())) + "\n"
	}

	if len(m.rules) == 0 {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No alert rules configured. Press n to create one.") + "\n"
	}

	var b strings.Builder

	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-3s %-28s %-24s %-10s", "", "NAME", "CONDITION", "SEVERITY")
	rows = append(rows, headerStyle.Render(hdr))

	for i, rule := range m.rules {
		isSelected := i == m.cursor
		dot := alertStatusDot(rule)
		name := trunc(rule.Name, 28)
		condition := fmt.Sprintf("%s %s %s", rule.Metric, rule.Operator, formatThreshold(rule.Threshold))
		condition = trunc(condition, 24)
		severity := alertSeverityBadge(rule.Severity)

		row := fmt.Sprintf("  %s %-28s %-24s %s", dot, name, condition, severity)

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
	title := fmt.Sprintf("Alert Rules (%d)", len(m.rules))
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n\n")

	// Status message
	if m.statusMsg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Padding(0, 1).Render(m.statusMsg))
		b.WriteString("\n")
	}

	// Footer
	footer := lipgloss.NewStyle().Foreground(cDim).Render(
		" n new | d delete | t toggle | h history | r refresh | q quit") + " " + NavHint()
	b.WriteString(footer)
	b.WriteString("\n")

	// Nav overlay (rendered on top if active)
	if overlay := m.nav.ViewNav(w); overlay != "" {
		b.WriteString("\n" + overlay + "\n")
	}

	return b.String()
}

// -- History View --

func (m alertsModel) renderHistoryView(w int) string {
	var b strings.Builder

	ruleName := ""
	if m.cursor < len(m.rules) {
		ruleName = m.rules[m.cursor].Name
	}

	if len(m.history) == 0 {
		content := lipgloss.NewStyle().Foreground(cMuted).Render("No alert firings recorded for this rule.")
		title := fmt.Sprintf("Alert History: %s", ruleName)
		b.WriteString("\n")
		b.WriteString(titledPanel(title, content, w-2))
		b.WriteString("\n\n")
		footer := lipgloss.NewStyle().Foreground(cDim).Render(" esc back | q quit")
		b.WriteString(footer)
		b.WriteString("\n")
		return b.String()
	}

	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-16s %-10s %12s %12s  %-30s", "TIMESTAMP", "STATUS", "VALUE", "THRESHOLD", "MESSAGE")
	rows = append(rows, headerStyle.Render(hdr))

	// Calculate visible window
	visibleRows := m.height - 10
	if visibleRows < 5 {
		visibleRows = 5
	}

	startIdx := 0
	if m.historyCursor >= visibleRows {
		startIdx = m.historyCursor - visibleRows + 1
	}
	endIdx := startIdx + visibleRows
	if endIdx > len(m.history) {
		endIdx = len(m.history)
	}

	for i := startIdx; i < endIdx; i++ {
		h := m.history[i]
		isSelected := i == m.historyCursor
		ts := h.TriggeredAt.Format("Jan 02 15:04")
		status := alertHistoryStatusBadge(h.Status)
		val := formatThreshold(h.CurrentValue)
		thresh := formatThreshold(h.Threshold)
		msgW := w - 60
		if msgW < 10 {
			msgW = 30
		}
		msg := trunc(h.Message, msgW)

		row := fmt.Sprintf("  %-16s %-10s %12s %12s  %-30s", ts, status, val, thresh, msg)

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
	title := fmt.Sprintf("Alert History: %s", ruleName)
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n")

	// Scroll indicator
	if len(m.history) > visibleRows {
		scrollInfo := fmt.Sprintf(" [%d/%d]", m.historyCursor+1, len(m.history))
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Render(scrollInfo))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	footer := lipgloss.NewStyle().Foreground(cDim).Render(" j/k scroll | esc back | q quit")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Create Wizard View --

func (m alertsModel) renderCreateView(w int) string {
	var b strings.Builder

	if m.loading {
		return "\n" + lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("Loading channels...") + "\n"
	}

	steps := []string{"Name", "Metric", "Condition", "Channels", "Confirm"}
	stepBar := renderWizardSteps(steps, m.wizardStep)

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 1).Render("New Alert Rule"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 1).Render(stepBar))
	b.WriteString("\n\n")

	switch m.wizardStep {
	case 0:
		b.WriteString(m.renderWizardName())
	case 1:
		b.WriteString(m.renderWizardMetric())
	case 2:
		b.WriteString(m.renderWizardCondition())
	case 3:
		b.WriteString(m.renderWizardChannels())
	case 4:
		b.WriteString(m.renderWizardConfirm())
	}

	if m.wizardErr != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(m.wizardErr))
	}

	b.WriteString("\n\n")
	footer := lipgloss.NewStyle().Foreground(cDim).Render(" esc cancel")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

func (m alertsModel) renderWizardName() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Rule name: "))
	cursor := lipgloss.NewStyle().Foreground(cBrand).Render("|")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(m.wizardName))
	b.WriteString(cursor)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Type a name and press Enter"))
	return b.String()
}

func (m alertsModel) renderWizardMetric() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Select metric:"))
	b.WriteString("\n\n")

	for i, metric := range alertMetrics {
		prefix := "  "
		if i == m.wizardMetric {
			prefix = lipgloss.NewStyle().Foreground(cBrand).Render("> ")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Bold(true).Padding(0, 2).Render(prefix + metric))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render(prefix + metric))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("j/k select, Enter confirm"))
	return b.String()
}

func (m alertsModel) renderWizardCondition() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Set condition:"))
	b.WriteString("\n\n")

	// Operator selection
	opLabel := "Operator"
	if m.wizardFocus == 0 {
		opLabel = lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render("Operator")
	} else {
		opLabel = lipgloss.NewStyle().Foreground(cDim).Render("Operator")
	}
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(opLabel))
	b.WriteString("\n")

	for i, op := range alertOperators {
		prefix := "  "
		if i == m.wizardOp {
			prefix = lipgloss.NewStyle().Foreground(cBrand).Render("> ")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Padding(0, 4).Render(prefix + op))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 4).Render(prefix + op))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Threshold input
	thLabel := "Threshold"
	if m.wizardFocus == 1 {
		thLabel = lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render("Threshold")
	} else {
		thLabel = lipgloss.NewStyle().Foreground(cDim).Render("Threshold")
	}
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(thLabel))
	b.WriteString("\n")

	cursor := ""
	if m.wizardFocus == 1 {
		cursor = lipgloss.NewStyle().Foreground(cBrand).Render("|")
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Padding(0, 4).Render(m.wizardThreshold + cursor))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Tab switch focus, j/k select operator, Enter confirm"))
	return b.String()
}

func (m alertsModel) renderWizardChannels() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Select notification channels:"))
	b.WriteString("\n\n")

	if len(m.channels) == 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No channels configured"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Press Enter to continue without channels"))
		return b.String()
	}

	for i, ch := range m.channels {
		check := "[ ]"
		if i < len(m.channelSelected) && m.channelSelected[i] {
			check = lipgloss.NewStyle().Foreground(cSuccess).Render("[x]")
		}

		label := fmt.Sprintf("%s  %s (%s)", check, ch.Name, ch.Type)

		if i == m.wizardSeverity {
			prefix := lipgloss.NewStyle().Foreground(cBrand).Render("> ")
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Padding(0, 2).Render(prefix + label))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("j/k navigate, Space toggle, Enter confirm"))
	return b.String()
}

func (m alertsModel) renderWizardConfirm() string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Review and create:"))
	b.WriteString("\n\n")

	dimLabel := lipgloss.NewStyle().Foreground(cDim)
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Name:      ") + valStyle.Render(m.wizardName)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Metric:    ") + valStyle.Render(alertMetrics[m.wizardMetric])))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Condition: ") + valStyle.Render(fmt.Sprintf("%s %s", alertOperators[m.wizardOp], m.wizardThreshold))))
	b.WriteString("\n")

	// Selected channels
	var selectedNames []string
	for i, ch := range m.channels {
		if i < len(m.channelSelected) && m.channelSelected[i] {
			selectedNames = append(selectedNames, ch.Name)
		}
	}
	channelStr := "None"
	if len(selectedNames) > 0 {
		channelStr = strings.Join(selectedNames, ", ")
	}
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Channels:  ") + valStyle.Render(channelStr)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Severity:  ") + valStyle.Render("warning")))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Enter create | Esc cancel"))
	return b.String()
}

// -- Delete Confirmation View --

func (m alertsModel) renderDeleteView(w int) string {
	// Render the list view but with an inline delete prompt
	if len(m.rules) == 0 || m.cursor >= len(m.rules) {
		m.view = "list"
		return m.renderAlertListView(w)
	}

	var b strings.Builder

	var rows []string

	headerStyle := lipgloss.NewStyle().Foreground(cMuted).Bold(true)
	hdr := fmt.Sprintf("  %-3s %-28s %-24s %-10s", "", "NAME", "CONDITION", "SEVERITY")
	rows = append(rows, headerStyle.Render(hdr))

	for i, rule := range m.rules {
		isSelected := i == m.cursor
		dot := alertStatusDot(rule)
		name := trunc(rule.Name, 28)
		condition := fmt.Sprintf("%s %s %s", rule.Metric, rule.Operator, formatThreshold(rule.Threshold))
		condition = trunc(condition, 24)
		severity := alertSeverityBadge(rule.Severity)

		row := fmt.Sprintf("  %s %-28s %-24s %s", dot, name, condition, severity)

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

		// Insert delete prompt after selected row
		if isSelected {
			prompt := fmt.Sprintf("  Delete rule '%s'? (y/N)", rule.Name)
			rows = append(rows, lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render(prompt))
		}
	}

	content := strings.Join(rows, "\n")
	title := fmt.Sprintf("Alert Rules (%d)", len(m.rules))
	b.WriteString("\n")
	b.WriteString(titledPanel(title, content, w-2))
	b.WriteString("\n\n")

	footer := lipgloss.NewStyle().Foreground(cDim).Render(
		" y confirm delete | n/esc cancel")
	b.WriteString(footer)
	b.WriteString("\n")

	return b.String()
}

// -- Helpers --

func sortAlertRules(rules []client.CLIAlertRule) []client.CLIAlertRule {
	sort.SliceStable(rules, func(i, j int) bool {
		iScore := alertSortScore(rules[i])
		jScore := alertSortScore(rules[j])
		return iScore > jScore
	})
	return rules
}

func alertSortScore(r client.CLIAlertRule) int {
	// Firing (has LastTriggeredAt and enabled) first, then enabled, then disabled
	if r.Enabled && r.LastTriggeredAt != nil {
		return 2
	}
	if r.Enabled {
		return 1
	}
	return 0
}

func alertStatusDot(rule client.CLIAlertRule) string {
	if rule.Enabled && rule.LastTriggeredAt != nil {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Render("\u25cf")
	}
	if rule.Enabled {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Render("\u25cf")
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("\u25cf")
}

func alertSeverityBadge(severity string) string {
	switch severity {
	case "critical":
		return lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("CRIT")
	case "warning":
		return lipgloss.NewStyle().Foreground(cWarning).Bold(true).Render("WARN")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render("INFO")
	}
}

func alertHistoryStatusBadge(status string) string {
	switch status {
	case "firing":
		return lipgloss.NewStyle().Foreground(cDanger).Bold(true).Render("FIRING")
	case "resolved":
		return lipgloss.NewStyle().Foreground(cSuccess).Render("RESOLVED")
	default:
		return lipgloss.NewStyle().Foreground(cMuted).Render(status)
	}
}

func formatThreshold(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%.0f", v)
	}
	return fmt.Sprintf("%.2f", v)
}

func renderWizardSteps(steps []string, current int) string {
	var parts []string
	for i, step := range steps {
		if i < current {
			parts = append(parts, lipgloss.NewStyle().Foreground(cSuccess).Render(fmt.Sprintf("%d.%s", i+1, step)))
		} else if i == current {
			parts = append(parts, lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render(fmt.Sprintf("%d.%s", i+1, step)))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(cDim).Render(fmt.Sprintf("%d.%s", i+1, step)))
		}
	}
	return strings.Join(parts, " > ")
}

// -- Runner --

func runAlerts(cmd *cobra.Command, args []string) error {
	c, err := NewAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	model := alertsModel{
		apiClient: c,
		view:      "list",
		loading:   true,
		nav:       newNavModel("alerts"),
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("alerts error: %w", err)
	}

	// Check if user wants to switch to another command
	if m, ok := result.(alertsModel); ok && m.navTarget != "" {
		return RunNavTarget(m.navTarget)
	}
	return nil
}
