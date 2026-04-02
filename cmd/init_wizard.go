package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/detector"
	"github.com/yourusername/context.io/cli/internal/sdk"
	"github.com/yourusername/context.io/cli/internal/utils"
)

// -- Init wizard Bubbletea messages --

type initFrameworkDetectedMsg struct {
	framework *detector.Framework
}

type initRegisteredMsg struct {
	resp *client.RegisterResponse
}

type initVerifiedMsg struct {
	resp *client.VerifyResponse
}

type initConfigSavedMsg struct{}

type initErrMsg struct {
	err error
}

type initHealthDetectedMsg struct {
	endpoints []string
}

type initAlertsCreatedMsg struct {
	count int
}

type initInstallStepDoneMsg struct {
	index int
	err   error
}

type initSpinnerTickMsg struct{}

// -- New types --

type alertSuggestion struct {
	name      string
	metric    string  // "error_rate", "p95_latency", "health_check"
	operator  string  // "greater_than", "equals"
	threshold float64
	severity  string // "warning", "critical"
}

type installStep struct {
	label string // "Saving configuration", "Sending test trace", "Installing SDK"
	done  bool
	err   string
}

// Spinner frames for animated install progress
var initSpinnerFrames = []string{"\xe2\xa0\x8b", "\xe2\xa0\x99", "\xe2\xa0\xb9", "\xe2\xa0\xb8", "\xe2\xa0\xbc", "\xe2\xa0\xb4", "\xe2\xa0\xa6", "\xe2\xa0\xa7", "\xe2\xa0\x87", "\xe2\xa0\x8f"}

// -- Init wizard model --

type initWizardModel struct {
	apiClient *client.Client
	width     int
	height    int
	step      int // 0=account, 1=verify, 2=framework, 3=alerts, 4=install, 5=complete
	quitting  bool
	err       error

	// Framework detection (runs on Init)
	framework   *detector.Framework
	serviceName string

	// Step 0: Account
	email string

	// Step 1: Verify
	sessionID string
	code      string

	// Step 2: Framework (health endpoint detection)
	verifyResp     *client.VerifyResponse
	healthEndpoints []string
	healthSelected  []bool
	healthCursor    int

	// Step 3: Alerts
	alertSuggestions []alertSuggestion
	alertSelected   []bool
	alertCursor     int
	alertsCreated   int

	// Step 4: Install
	installSteps   []installStep
	installCurrent int
	installDone    bool
	spinnerFrame   int

	// Step 5: Complete
	dashboardURL string
	apiKeyMasked string

	// Flags from cobra
	apiURL string
	useDev bool
}

var initWizardSteps = []string{"Account", "Verify", "Framework", "Alerts", "Install", "Complete"}

// -- Init --

func (m initWizardModel) Init() tea.Cmd {
	return func() tea.Msg {
		framework, err := detector.Detect()
		if err != nil {
			return initFrameworkDetectedMsg{framework: &detector.Framework{Name: "generic", Type: "unknown"}}
		}
		return initFrameworkDetectedMsg{framework: framework}
	}
}

// -- Update --

func (m initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case initFrameworkDetectedMsg:
		m.framework = msg.framework
		return m, nil

	case initRegisteredMsg:
		m.sessionID = msg.resp.SessionID
		m.err = nil
		m.step = 1
		return m, nil

	case initVerifiedMsg:
		m.verifyResp = msg.resp
		m.err = nil
		m.step = 2
		// Detect health endpoints for the framework step
		return m, m.detectHealthCmd()

	case initHealthDetectedMsg:
		m.healthEndpoints = msg.endpoints
		m.healthSelected = make([]bool, len(msg.endpoints))
		for i := range m.healthSelected {
			m.healthSelected[i] = true // pre-select all
		}
		m.healthCursor = 0
		return m, nil

	case initAlertsCreatedMsg:
		m.alertsCreated = msg.count
		m.step = 4
		// Initialize install steps and start install
		m.installSteps = []installStep{
			{label: "Saving configuration"},
			{label: "Sending test trace"},
			{label: "Installing SDK"},
		}
		m.installCurrent = 0
		return m, tea.Batch(m.runInstallStepCmd(0), m.spinnerTickCmd())

	case initInstallStepDoneMsg:
		if msg.index >= 0 && msg.index < len(m.installSteps) {
			m.installSteps[msg.index].done = true
			if msg.err != nil {
				m.installSteps[msg.index].err = msg.err.Error()
			}
		}
		next := msg.index + 1
		if next < len(m.installSteps) {
			m.installCurrent = next
			return m, m.runInstallStepCmd(next)
		}
		// All install steps done
		m.installDone = true
		m.dashboardURL = m.verifyResp.DashboardURL
		m.apiKeyMasked = utils.MaskAPIKey(m.verifyResp.APIKey)
		m.step = 5
		return m, nil

	case initSpinnerTickMsg:
		if m.step == 4 && !m.installDone {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(initSpinnerFrames)
			return m, m.spinnerTickCmd()
		}
		return m, nil

	case initConfigSavedMsg:
		// Legacy message -- no longer used in new flow but kept for compatibility
		return m, nil

	case initErrMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m initWizardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global keys
	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		// Back navigation: allowed on steps 1-3 (verify, framework, alerts)
		// Not allowed on install (step 4) or complete (step 5)
		if m.step >= 1 && m.step <= 3 {
			m.err = nil
			m.step--
		}
		return m, nil
	}

	switch m.step {
	case 0:
		return m.handleAccountKey(msg)
	case 1:
		return m.handleVerifyKey(msg)
	case 2:
		return m.handleFrameworkKey(msg)
	case 3:
		return m.handleAlertsKey(msg)
	case 4:
		// Install step is auto-executing, no key handling
		return m, nil
	case 5:
		return m.handleCompleteKey(msg)
	}

	return m, nil
}

func (m initWizardModel) handleAccountKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		if strings.TrimSpace(m.email) == "" {
			m.err = fmt.Errorf("email is required")
			return m, nil
		}
		m.err = nil
		return m, m.registerCmd()
	case "backspace":
		if len(m.email) > 0 {
			m.email = m.email[:len(m.email)-1]
		}
	case "r":
		if m.err != nil {
			m.err = nil
			return m, m.registerCmd()
		}
		m.email += key
	default:
		if len(key) == 1 {
			m.email += key
		}
	}

	return m, nil
}

func (m initWizardModel) handleVerifyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		if strings.TrimSpace(m.code) == "" {
			m.err = fmt.Errorf("verification code is required")
			return m, nil
		}
		m.err = nil
		return m, m.verifyCmd()
	case "backspace":
		if len(m.code) > 0 {
			m.code = m.code[:len(m.code)-1]
		}
	case "r":
		if m.err != nil {
			m.err = nil
			return m, m.verifyCmd()
		}
		if len(key) == 1 && len(m.code) < 6 {
			m.code += key
		}
	default:
		if len(key) == 1 && len(m.code) < 6 {
			m.code += key
		}
	}

	return m, nil
}

func (m initWizardModel) handleFrameworkKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		// Advance to alerts step
		m.step = 3
		m.populateAlertSuggestions()
		return m, nil
	case "j", "down":
		if len(m.healthEndpoints) > 0 && m.healthCursor < len(m.healthEndpoints)-1 {
			m.healthCursor++
		}
	case "k", "up":
		if m.healthCursor > 0 {
			m.healthCursor--
		}
	case " ":
		if len(m.healthSelected) > 0 && m.healthCursor < len(m.healthSelected) {
			m.healthSelected[m.healthCursor] = !m.healthSelected[m.healthCursor]
		}
	}

	return m, nil
}

func (m initWizardModel) handleAlertsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		// Create selected alerts and advance
		return m, m.createAlertsCmd()
	case "j", "down":
		if len(m.alertSuggestions) > 0 && m.alertCursor < len(m.alertSuggestions)-1 {
			m.alertCursor++
		}
	case "k", "up":
		if m.alertCursor > 0 {
			m.alertCursor--
		}
	case " ":
		if len(m.alertSelected) > 0 && m.alertCursor < len(m.alertSelected) {
			m.alertSelected[m.alertCursor] = !m.alertSelected[m.alertCursor]
		}
	}

	return m, nil
}

func (m initWizardModel) handleCompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter", "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// -- Populate helpers --

func (m *initWizardModel) populateAlertSuggestions() {
	fwType := ""
	fwName := ""
	if m.framework != nil {
		fwType = m.framework.Type
		fwName = m.framework.Name
	}

	switch {
	case fwType == "node" || fwName == "express" || fwName == "nestjs" || fwName == "nextjs":
		m.alertSuggestions = []alertSuggestion{
			{name: "High Error Rate", metric: "error_rate", operator: "greater_than", threshold: 5, severity: "warning"},
			{name: "Slow P95 Latency", metric: "p95_latency", operator: "greater_than", threshold: 2000, severity: "warning"},
		}
	case fwType == "python" || fwName == "django" || fwName == "flask" || fwName == "fastapi":
		m.alertSuggestions = []alertSuggestion{
			{name: "Slow Query Detection", metric: "p95_latency", operator: "greater_than", threshold: 2000, severity: "warning"},
			{name: "High Error Rate", metric: "error_rate", operator: "greater_than", threshold: 5, severity: "warning"},
		}
	case fwType == "go":
		m.alertSuggestions = []alertSuggestion{
			{name: "Health Check Down", metric: "health_check", operator: "equals", threshold: 0, severity: "critical"},
			{name: "High P95 Latency", metric: "p95_latency", operator: "greater_than", threshold: 1000, severity: "warning"},
		}
	default:
		m.alertSuggestions = []alertSuggestion{
			{name: "High Error Rate", metric: "error_rate", operator: "greater_than", threshold: 5, severity: "warning"},
		}
	}

	// Pre-select all
	m.alertSelected = make([]bool, len(m.alertSuggestions))
	for i := range m.alertSelected {
		m.alertSelected[i] = true
	}
	m.alertCursor = 0
}

// -- Tea commands --

func (m initWizardModel) registerCmd() tea.Cmd {
	return func() tea.Msg {
		source := "generic"
		fwVersion := ""
		if m.framework != nil {
			source = m.framework.Name
			fwVersion = m.framework.Version
		}

		req := &client.RegisterRequest{
			Email:            m.email,
			OrganizationName: "",
			ServiceName:      m.serviceName,
			Source:           source,
			SourceMetadata: map[string]interface{}{
				"cli_version":       CLIVersion,
				"framework_version": fwVersion,
				"platform":          runtime.GOOS + "_" + runtime.GOARCH,
			},
		}

		resp, err := m.apiClient.Register(req)
		if err != nil {
			return initErrMsg{err: fmt.Errorf("registration failed: %w", err)}
		}
		return initRegisteredMsg{resp: resp}
	}
}

func (m initWizardModel) verifyCmd() tea.Cmd {
	return func() tea.Msg {
		req := &client.VerifyRequest{
			SessionID: m.sessionID,
			Code:      m.code,
		}

		resp, err := m.apiClient.Verify(req)
		if err != nil {
			return initErrMsg{err: fmt.Errorf("verification failed: %w", err)}
		}
		return initVerifiedMsg{resp: resp}
	}
}

func (m initWizardModel) detectHealthCmd() tea.Cmd {
	return func() tea.Msg {
		endpoints := detector.DetectHealthEndpoints(m.framework)
		return initHealthDetectedMsg{endpoints: endpoints}
	}
}

func (m initWizardModel) createAlertsCmd() tea.Cmd {
	return func() tea.Msg {
		created := 0
		apiCli := m.apiClient
		// Set API key from verify response so CreateAlertRule works
		if m.verifyResp != nil {
			apiCli.APIKey = m.verifyResp.APIKey
		}

		for i, suggestion := range m.alertSuggestions {
			if i >= len(m.alertSelected) || !m.alertSelected[i] {
				continue
			}

			req := client.CreateAlertRuleRequest{
				Name:       suggestion.name,
				AlertType:  "metric",
				ScopeType:  "service",
				ScopeValue: m.serviceName,
				Metric:     suggestion.metric,
				Operator:   suggestion.operator,
				Threshold:  suggestion.threshold,
				TimeWindow: 300,
				Cooldown:   600,
				Severity:   suggestion.severity,
				ChannelIDs: []string{},
			}

			_, err := apiCli.CreateAlertRule(req)
			if err == nil {
				created++
			}
		}

		return initAlertsCreatedMsg{count: created}
	}
}

func (m initWizardModel) runInstallStepCmd(index int) tea.Cmd {
	return func() tea.Msg {
		switch index {
		case 0:
			// Save configuration
			cfg := &config.Config{
				APIKey:                m.verifyResp.APIKey,
				Endpoint:              m.apiClient.BaseURL,
				ServiceName:           m.serviceName,
				Enabled:               "true",
				CodeMonitoringEnabled: "true",
			}
			err := config.Save(cfg)
			return initInstallStepDoneMsg{index: 0, err: err}

		case 1:
			// Send test trace
			cfg := &config.Config{
				APIKey:                m.verifyResp.APIKey,
				Endpoint:              m.apiClient.BaseURL,
				ServiceName:           m.serviceName,
				Enabled:               "true",
				CodeMonitoringEnabled: "true",
			}
			err := sendTestTraceInternal(cfg)
			return initInstallStepDoneMsg{index: 1, err: err}

		case 2:
			// Install SDK
			if m.framework != nil {
				recommended := sdk.GetRecommendedSDK(m.framework.Type, m.framework.Name)
				if recommended != nil {
					err := sdk.Install(*recommended)
					return initInstallStepDoneMsg{index: 2, err: err}
				}
			}
			return initInstallStepDoneMsg{index: 2, err: nil}
		}

		return initInstallStepDoneMsg{index: index, err: nil}
	}
}

func (m initWizardModel) spinnerTickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return initSpinnerTickMsg{}
	})
}

// -- View --

func (m initWizardModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	var b strings.Builder

	// Header
	b.WriteString("\n")
	header := lipgloss.NewStyle().Foreground(cBrand).Bold(true).Padding(0, 1).Render("TraceKit Setup")
	b.WriteString(header)
	b.WriteString("\n")

	// Step bar
	stepBar := renderInitWizardSteps(initWizardSteps, m.step)
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 1).Render(stepBar))
	b.WriteString("\n\n")

	// Step content
	switch m.step {
	case 0:
		b.WriteString(m.renderAccountStep())
	case 1:
		b.WriteString(m.renderVerifyStep())
	case 2:
		b.WriteString(m.renderFrameworkStep())
	case 3:
		b.WriteString(m.renderAlertsStep())
	case 4:
		b.WriteString(m.renderInstallStep())
	case 5:
		b.WriteString(m.renderCompleteStep())
	}

	// Error display
	if m.err != nil {
		b.WriteString("\n")
		errMsg := lipgloss.NewStyle().Foreground(cDanger).Padding(0, 2).Render(fmt.Sprintf("Error: %s", m.err.Error()))
		b.WriteString(errMsg)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Press r to retry"))
	}

	// Footer
	b.WriteString("\n\n")
	if m.step < 5 && m.step != 4 {
		footer := lipgloss.NewStyle().Foreground(cDim).Render(" Esc back | Ctrl+C quit")
		b.WriteString(footer)
	} else if m.step == 4 {
		footer := lipgloss.NewStyle().Foreground(cDim).Render(" Installing... | Ctrl+C quit")
		b.WriteString(footer)
	}
	b.WriteString("\n")

	return b.String()
}

func (m initWizardModel) renderAccountStep() string {
	var b strings.Builder

	// Framework detection result
	if m.framework != nil {
		if m.framework.Name != "generic" {
			fwText := fmt.Sprintf("Detected: %s (%s)", m.framework.Name, m.framework.Type)
			b.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Padding(0, 2).Render("* " + fwText))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No framework detected, continuing with generic setup"))
		}
		b.WriteString("\n\n")
	}

	// Email input
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Email: "))
	cursor := lipgloss.NewStyle().Foreground(cBrand).Render("|")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(m.email))
	b.WriteString(cursor)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Enter your email and press Enter"))

	return b.String()
}

func (m initWizardModel) renderVerifyStep() string {
	var b strings.Builder

	info := fmt.Sprintf("Verification code sent to %s", m.email)
	b.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Padding(0, 2).Render("* " + info))
	b.WriteString("\n\n")

	// Code input
	b.WriteString(lipgloss.NewStyle().Foreground(cText).Padding(0, 2).Render("Code: "))
	cursor := lipgloss.NewStyle().Foreground(cBrand).Render("|")
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Render(m.code))
	b.WriteString(cursor)
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Enter 6-digit verification code and press Enter"))

	return b.String()
}

func (m initWizardModel) renderFrameworkStep() string {
	var b strings.Builder

	// Framework info
	if m.framework != nil && m.framework.Name != "generic" {
		fwText := fmt.Sprintf("Detected: %s (%s)", m.framework.Name, m.framework.Type)
		b.WriteString(lipgloss.NewStyle().Foreground(cSuccess).Padding(0, 2).Render("* " + fwText))
		b.WriteString("\n\n")
	}

	// Health endpoints
	if len(m.healthEndpoints) > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Health check endpoints found:"))
		b.WriteString("\n\n")

		for i, ep := range m.healthEndpoints {
			checkbox := "[ ] "
			if i < len(m.healthSelected) && m.healthSelected[i] {
				checkbox = "[x] "
			}

			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Padding(0, 2)
			if i == m.healthCursor {
				style = style.Foreground(cBrand).Bold(true)
				checkbox = "> " + checkbox
			} else {
				checkbox = "  " + checkbox
			}

			b.WriteString(style.Render(checkbox + ep))
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("j/k navigate | space toggle | enter continue"))
	} else {
		b.WriteString(lipgloss.NewStyle().Foreground(cMuted).Padding(0, 2).Render("No health endpoints detected"))
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Press enter to continue"))
	}

	return b.String()
}

func (m initWizardModel) renderAlertsStep() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Suggested alert rules:"))
	b.WriteString("\n\n")

	for i, suggestion := range m.alertSuggestions {
		checkbox := "[ ] "
		if i < len(m.alertSelected) && m.alertSelected[i] {
			checkbox = "[x] "
		}

		label := fmt.Sprintf("%s (%s %s %.0f, %s)",
			suggestion.name, suggestion.metric, suggestion.operator, suggestion.threshold, suggestion.severity)

		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff")).Padding(0, 2)
		if i == m.alertCursor {
			style = style.Foreground(cBrand).Bold(true)
			checkbox = "> " + checkbox
		} else {
			checkbox = "  " + checkbox
		}

		b.WriteString(style.Render(checkbox + label))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("j/k navigate | space toggle | enter confirm"))

	return b.String()
}

func (m initWizardModel) renderInstallStep() string {
	var b strings.Builder

	b.WriteString(lipgloss.NewStyle().Foreground(cText).Bold(true).Padding(0, 2).Render("Setting up your project..."))
	b.WriteString("\n\n")

	for i, step := range m.installSteps {
		var icon string
		if step.done {
			if step.err != "" {
				// Red X for failed step
				icon = lipgloss.NewStyle().Foreground(cDanger).Render("\xe2\x9c\x97")
			} else {
				// Green checkmark for completed step
				icon = lipgloss.NewStyle().Foreground(cSuccess).Render("\xe2\x9c\x93")
			}
		} else if i == m.installCurrent {
			// Animated spinner for active step
			frame := m.spinnerFrame % len(initSpinnerFrames)
			icon = lipgloss.NewStyle().Foreground(cBrand).Render(initSpinnerFrames[frame])
		} else {
			// Dim circle for pending step
			icon = lipgloss.NewStyle().Foreground(cMuted).Render("\xe2\x97\x8b")
		}

		label := step.label
		labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
		if step.done && step.err != "" {
			label += " - " + step.err
			labelStyle = labelStyle.Foreground(cDanger)
		}

		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(icon + " " + labelStyle.Render(label)))
		b.WriteString("\n")
	}

	return b.String()
}

func (m initWizardModel) renderCompleteStep() string {
	var b strings.Builder

	// Summary box
	title := lipgloss.NewStyle().Foreground(cSuccess).Bold(true).Padding(0, 2).Render("Setup Complete!")
	b.WriteString(title)
	b.WriteString("\n\n")

	dimLabel := lipgloss.NewStyle().Foreground(cDim)
	valStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	// Framework
	if m.framework != nil && m.framework.Name != "generic" {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			dimLabel.Render("Framework:  ") + valStyle.Render(fmt.Sprintf("%s (%s)", m.framework.Name, m.framework.Type))))
		b.WriteString("\n")
	}

	// Health endpoints
	healthCount := 0
	for i, sel := range m.healthSelected {
		if sel && i < len(m.healthEndpoints) {
			healthCount++
		}
	}
	if healthCount > 0 {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			dimLabel.Render("Health:     ") + valStyle.Render(fmt.Sprintf("%d endpoints configured", healthCount))))
	} else {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			dimLabel.Render("Health:     ") + valStyle.Render("none detected")))
	}
	b.WriteString("\n")

	// Alerts
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Alerts:     ") + valStyle.Render(fmt.Sprintf("%d rules created", m.alertsCreated))))
	b.WriteString("\n")

	// SDK status
	sdkStatus := "skipped"
	if m.framework != nil {
		recommended := sdk.GetRecommendedSDK(m.framework.Type, m.framework.Name)
		if recommended != nil {
			if len(m.installSteps) > 2 && m.installSteps[2].done && m.installSteps[2].err == "" {
				sdkStatus = "installed"
			} else if len(m.installSteps) > 2 && m.installSteps[2].err != "" {
				sdkStatus = "failed"
			}
		}
	}
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("SDK:        ") + valStyle.Render(sdkStatus)))
	b.WriteString("\n")

	// Dashboard
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Dashboard:  ") + valStyle.Render(m.dashboardURL)))
	b.WriteString("\n")

	// API Key
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("API Key:    ") + valStyle.Render(m.apiKeyMasked)))
	b.WriteString("\n")

	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cBrand).Padding(0, 2).Render(
		"Run `tracekit dashboard` to see your data"))
	b.WriteString("\n\n")
	b.WriteString(lipgloss.NewStyle().Foreground(cDim).Padding(0, 2).Render("Press q to exit"))

	return b.String()
}

// renderInitWizardSteps renders the step indicator bar for the init wizard.
func renderInitWizardSteps(steps []string, current int) string {
	var parts []string
	for i, step := range steps {
		label := fmt.Sprintf("%d.%s", i+1, step)
		if i < current {
			parts = append(parts, lipgloss.NewStyle().Foreground(cSuccess).Render(label))
		} else if i == current {
			parts = append(parts, lipgloss.NewStyle().Foreground(cBrand).Bold(true).Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(cDim).Render(label))
		}
	}
	return strings.Join(parts, " > ")
}

// newInitWizardModel creates a new init wizard model with derived service name.
func newInitWizardModel(apiClient *client.Client, apiURL string, useDev bool) initWizardModel {
	cwd, _ := os.Getwd()
	serviceName := filepath.Base(cwd)
	serviceName = strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))

	return initWizardModel{
		apiClient:   apiClient,
		serviceName: serviceName,
		apiURL:      apiURL,
		useDev:      useDev,
	}
}
