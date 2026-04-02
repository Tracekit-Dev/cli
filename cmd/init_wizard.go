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

// -- Types --

type alertSuggestion struct {
	name      string
	metric    string
	operator  string
	threshold float64
	severity  string
}

type installStep struct {
	label   string
	done    bool
	err     string
	skipped bool
}

var initSpinnerFrames = []string{"\xe2\xa0\x8b", "\xe2\xa0\x99", "\xe2\xa0\xb9", "\xe2\xa0\xb8", "\xe2\xa0\xbc", "\xe2\xa0\xb4", "\xe2\xa0\xa6", "\xe2\xa0\xa7", "\xe2\xa0\x87", "\xe2\xa0\x8f"}

// -- Init wizard model --
// Flow: 0=account -> 1=verify -> 2=extras (optional) -> 3=setup (auto) -> 4=complete

type initWizardModel struct {
	apiClient *client.Client
	width     int
	height    int
	step      int // 0=account, 1=verify, 2=extras, 3=setup, 4=complete
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

	// Verify response (available after step 1)
	verifyResp *client.VerifyResponse

	// Step 2: Extras (optional -- user can skip all with 's')
	healthEndpoints []string
	healthSelected  []bool
	healthCursor    int
	alertSuggestions []alertSuggestion
	alertSelected   []bool
	alertCursor     int
	alertsCreated   int
	extrasView      string // "health", "alerts" -- sub-views within extras step
	configSaved     bool   // true once we've saved to ~/.tracekitconfig

	// Step 3: Setup (auto-executing)
	installSteps   []installStep
	installCurrent int
	installDone    bool
	spinnerFrame   int
	skipExtras     bool // true if user skipped health/alerts/SDK

	// Step 4: Complete
	dashboardURL string
	apiKeyMasked string
	nav          navModel
	navTarget    string

	// Flags from cobra
	apiURL string
	useDev bool
}

var initWizardSteps = []string{"Account", "Verify", "Options", "Setup", "Done"}

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
	case NavSwitchMsg:
		m.navTarget = msg.Command
		m.quitting = true
		return m, tea.Quit

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
		// Immediately save config to ~/.tracekitconfig (core requirement)
		return m, m.saveConfigCmd()

	case initConfigSavedMsg:
		m.configSaved = true
		// Now go to extras step with health detection
		m.step = 2
		m.extrasView = "health"
		return m, m.detectHealthCmd()

	case initHealthDetectedMsg:
		m.healthEndpoints = msg.endpoints
		m.healthSelected = make([]bool, len(msg.endpoints))
		for i := range m.healthSelected {
			m.healthSelected[i] = true
		}
		m.healthCursor = 0
		return m, nil

	case initAlertsCreatedMsg:
		m.alertsCreated = msg.count
		// Start setup step (test trace + SDK)
		m.step = 3
		m.buildInstallSteps()
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
		// Skip steps that are marked skipped
		for next < len(m.installSteps) && m.installSteps[next].skipped {
			m.installSteps[next].done = true
			next++
		}
		if next < len(m.installSteps) {
			m.installCurrent = next
			return m, m.runInstallStepCmd(next)
		}
		// All done
		m.installDone = true
		m.dashboardURL = m.verifyResp.DashboardURL
		m.apiKeyMasked = utils.MaskAPIKey(m.verifyResp.APIKey)
		m.step = 4
		return m, nil

	case initSpinnerTickMsg:
		if m.step == 3 && !m.installDone {
			m.spinnerFrame = (m.spinnerFrame + 1) % len(initSpinnerFrames)
			return m, m.spinnerTickCmd()
		}
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

	// Nav overlay on complete step
	if m.step == 4 {
		nav, consumed, navCmd := m.nav.HandleNavKey(msg)
		m.nav = nav
		if consumed {
			return m, navCmd
		}
	}

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		if m.step == 1 {
			m.err = nil
			m.code = ""
			m.step = 0
		} else if m.step == 2 && m.extrasView == "alerts" {
			m.extrasView = "health"
		}
		return m, nil
	}

	switch m.step {
	case 0:
		return m.handleAccountKey(msg)
	case 1:
		return m.handleVerifyKey(msg)
	case 2:
		return m.handleExtrasKey(msg)
	case 3:
		// Auto-executing, no keys
		return m, nil
	case 4:
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

func (m initWizardModel) handleExtrasKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Skip everything -- jump straight to setup with only test trace
	if key == "s" {
		m.skipExtras = true
		m.step = 3
		m.buildInstallSteps()
		m.installCurrent = 0
		return m, tea.Batch(m.runInstallStepCmd(0), m.spinnerTickCmd())
	}

	if m.extrasView == "health" {
		return m.handleHealthKey(key)
	}
	return m.handleAlertsKey(key)
}

func (m initWizardModel) handleHealthKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Advance to alerts sub-view
		m.extrasView = "alerts"
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

func (m initWizardModel) handleAlertsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		// Create selected alerts and advance to setup
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
	switch msg.String() {
	case "enter", "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// -- Helpers --

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

	m.alertSelected = make([]bool, len(m.alertSuggestions))
	for i := range m.alertSelected {
		m.alertSelected[i] = true
	}
	m.alertCursor = 0
}

func (m *initWizardModel) buildInstallSteps() {
	m.installSteps = []installStep{
		{label: "Sending test trace"},
	}
	if !m.skipExtras && m.framework != nil {
		recommended := sdk.GetRecommendedSDK(m.framework.Type, m.framework.Name)
		if recommended != nil {
			m.installSteps = append(m.installSteps, installStep{label: "Installing " + recommended.Name})
		}
	}
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

func (m initWizardModel) saveConfigCmd() tea.Cmd {
	return func() tea.Msg {
		cfg := &config.Config{
			APIKey:                m.verifyResp.APIKey,
			UserID:                m.verifyResp.UserID,
			Endpoint:              m.apiClient.BaseURL,
			ServiceName:           m.serviceName,
			Enabled:               "true",
			CodeMonitoringEnabled: "true",
		}
		if err := config.SaveGlobal(cfg); err != nil {
			return initErrMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		return initConfigSavedMsg{}
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
		if m.verifyResp != nil {
			apiCli.APIKey = m.verifyResp.APIKey
			apiCli.UserID = m.verifyResp.UserID
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
		if index >= len(m.installSteps) {
			return initInstallStepDoneMsg{index: index, err: nil}
		}

		switch index {
		case 0:
			// Send test trace
			cfg := &config.Config{
				APIKey:      m.verifyResp.APIKey,
				Endpoint:    m.apiClient.BaseURL,
				ServiceName: m.serviceName,
				Enabled:     "true",
			}
			err := sendTestTraceInternal(cfg)
			return initInstallStepDoneMsg{index: 0, err: err}

		case 1:
			// Install SDK (only if not skipped)
			if m.framework != nil {
				recommended := sdk.GetRecommendedSDK(m.framework.Type, m.framework.Name)
				if recommended != nil {
					err := sdk.Install(*recommended)
					return initInstallStepDoneMsg{index: 1, err: err}
				}
			}
			return initInstallStepDoneMsg{index: 1, err: nil}
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

	// Banner
	brand := lipgloss.Color("#6366f1")
	logo := lipgloss.NewStyle().Foreground(brand).Bold(true).Render(
		"  ████████╗██████╗  █████╗  ██████╗███████╗██╗  ██╗██╗████████╗\n" +
			"  ╚══██╔══╝██╔══██╗██╔══██╗██╔════╝██╔════╝██║ ██╔╝██║╚══██╔══╝\n" +
			"     ██║   ██████╔╝███████║██║     █████╗  █████╔╝ ██║   ██║\n" +
			"     ██║   ██╔══██╗██╔══██║██║     ██╔══╝  ██╔═██╗ ██║   ██║\n" +
			"     ██║   ██║  ██║██║  ██║╚██████╗███████╗██║  ██╗██║   ██║\n" +
			"     ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝╚══════╝╚═╝  ╚═╝╚═╝   ╚═╝")
	tagline := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#818cf8")).
		Italic(true).
		Render("        Zero-friction APM for modern applications")
	b.WriteString("\n" + logo + "\n\n" + tagline + "\n\n")

	// Step bar
	stepBar := renderInitWizardSteps(initWizardSteps, m.step)
	b.WriteString(" " + stepBar + "\n\n")

	// Step content
	switch m.step {
	case 0:
		b.WriteString(m.renderAccountStep())
	case 1:
		b.WriteString(m.renderVerifyStep())
	case 2:
		b.WriteString(m.renderExtrasStep())
	case 3:
		b.WriteString(m.renderSetupStep())
	case 4:
		b.WriteString(m.renderCompleteStep())
	}

	// Error
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Padding(0, 2).Render("Error: " + m.err.Error()))
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Padding(0, 2).Render("Press r to retry"))
	}

	// Footer
	b.WriteString("\n\n")
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	switch m.step {
	case 0, 1:
		b.WriteString(" " + dim.Render("enter submit  esc back  ctrl+c quit"))
	case 2:
		b.WriteString(" " + dim.Render("enter continue  s skip all  esc back  ctrl+c quit"))
	case 3:
		b.WriteString(" " + dim.Render("setting up...  ctrl+c quit"))
	case 4:
		b.WriteString(" " + dim.Render("q quit  ") + NavHint())
	}
	b.WriteString("\n")

	// Nav overlay on complete step
	if m.step == 4 {
		if overlay := m.nav.ViewNav(w); overlay != "" {
			b.WriteString("\n" + overlay + "\n")
		}
	}

	return b.String()
}

func (m initWizardModel) renderAccountStep() string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))

	// Framework detection result
	if m.framework != nil {
		if m.framework.Name != "generic" {
			b.WriteString("  " + success.Render("Detected: "+m.framework.Name+" ("+m.framework.Type+")") + "\n\n")
		} else {
			b.WriteString("  " + dim.Render("No framework detected, continuing with generic setup") + "\n\n")
		}
	}

	b.WriteString("  " + text.Bold(true).Render("Enter your email address") + "\n\n")
	b.WriteString("  " + dim.Render("Email: ") + text.Render(m.email) + lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1")).Render("_") + "\n")

	return b.String()
}

func (m initWizardModel) renderVerifyStep() string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))

	b.WriteString("  " + success.Render("Verification code sent to "+m.email) + "\n\n")
	b.WriteString("  " + text.Bold(true).Render("Enter the 6-digit code from your email") + "\n\n")

	// Code display
	boxes := ""
	for i := 0; i < 6; i++ {
		if i < len(m.code) {
			boxes += text.Bold(true).Render(" " + string(m.code[i]) + " ")
		} else if i == len(m.code) {
			boxes += lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1")).Render(" _ ")
		} else {
			boxes += dim.Render(" . ")
		}
	}
	b.WriteString("  " + dim.Render("Code: ") + boxes + "\n")

	return b.String()
}

func (m initWizardModel) renderExtrasStep() string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	brand := lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1"))

	b.WriteString("  " + success.Render("Account verified! Config saved to ~/.tracekitconfig") + "\n\n")

	if m.extrasView == "health" {
		b.WriteString("  " + text.Bold(true).Render("Health Check Endpoints") + "\n\n")

		if len(m.healthEndpoints) > 0 {
			for i, ep := range m.healthEndpoints {
				checkbox := "[ ] "
				if i < len(m.healthSelected) && m.healthSelected[i] {
					checkbox = "[x] "
				}
				style := text
				prefix := "    "
				if i == m.healthCursor {
					style = brand.Bold(true)
					prefix = "  > "
				}
				b.WriteString(prefix + style.Render(checkbox+ep) + "\n")
			}
			b.WriteString("\n  " + dim.Render("j/k navigate  space toggle  enter next  s skip all"))
		} else {
			b.WriteString("  " + dim.Render("No health endpoints detected") + "\n\n")
			b.WriteString("  " + dim.Render("enter next  s skip all"))
		}
	} else {
		// Alerts sub-view
		b.WriteString("  " + text.Bold(true).Render("Suggested Alert Rules") + "\n\n")

		for i, suggestion := range m.alertSuggestions {
			checkbox := "[ ] "
			if i < len(m.alertSelected) && m.alertSelected[i] {
				checkbox = "[x] "
			}
			label := fmt.Sprintf("%s (%s %s %.0f)", suggestion.name, suggestion.metric, suggestion.operator, suggestion.threshold)
			style := text
			prefix := "    "
			if i == m.alertCursor {
				style = brand.Bold(true)
				prefix = "  > "
			}
			b.WriteString(prefix + style.Render(checkbox+label) + "\n")
		}
		b.WriteString("\n  " + dim.Render("j/k navigate  space toggle  enter create & continue  s skip all"))
	}

	return b.String()
}

func (m initWizardModel) renderSetupStep() string {
	var b strings.Builder
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	b.WriteString("  " + text.Bold(true).Render("Setting up your project...") + "\n\n")

	for i, step := range m.installSteps {
		var icon string
		if step.skipped {
			icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("-")
		} else if step.done {
			if step.err != "" {
				icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Render("x")
			} else {
				icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Render("*")
			}
		} else if i == m.installCurrent {
			frame := m.spinnerFrame % len(initSpinnerFrames)
			icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1")).Render(initSpinnerFrames[frame])
		} else {
			icon = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("o")
		}

		label := step.label
		labelStyle := text
		if step.done && step.err != "" {
			label += " - " + step.err
			labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
		} else if step.skipped {
			label += " (skipped)"
			labelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
		}

		b.WriteString("  " + icon + " " + labelStyle.Render(label) + "\n")
	}

	return b.String()
}

func (m initWizardModel) renderCompleteStep() string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	brand := lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1"))

	b.WriteString("  " + success.Bold(true).Render("Setup Complete!") + "\n\n")

	b.WriteString("  " + dim.Render("Config:     ") + text.Render("~/.tracekitconfig") + "\n")

	if m.framework != nil && m.framework.Name != "generic" {
		b.WriteString("  " + dim.Render("Framework:  ") + text.Render(m.framework.Name+" ("+m.framework.Type+")") + "\n")
	}

	if m.alertsCreated > 0 {
		b.WriteString("  " + dim.Render("Alerts:     ") + text.Render(fmt.Sprintf("%d rules created", m.alertsCreated)) + "\n")
	}

	b.WriteString("  " + dim.Render("Dashboard:  ") + brand.Render(m.dashboardURL) + "\n")
	b.WriteString("  " + dim.Render("API Key:    ") + text.Render(m.apiKeyMasked) + "\n")

	b.WriteString("\n  " + dim.Render("Next steps:") + "\n")
	b.WriteString("  " + text.Render("  tracekit dashboard") + dim.Render("   -- view live dashboard") + "\n")
	b.WriteString("  " + text.Render("  tracekit traces") + dim.Render("      -- browse traces") + "\n")
	b.WriteString("  " + text.Render("  tracekit alerts") + dim.Render("      -- manage alerts") + "\n")

	return b.String()
}

func renderInitWizardSteps(steps []string, current int) string {
	var parts []string
	for i, step := range steps {
		label := fmt.Sprintf("[%d] %s", i+1, step)
		if i < current {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Render(label+" *"))
		} else if i == current {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1")).Bold(true).Render(label))
		} else {
			parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(label))
		}
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(" > ")
	return strings.Join(parts, sep)
}

func newInitWizardModel(apiClient *client.Client, apiURL string, useDev bool) initWizardModel {
	cwd, _ := os.Getwd()
	serviceName := filepath.Base(cwd)
	serviceName = strings.ToLower(strings.ReplaceAll(serviceName, " ", "-"))

	return initWizardModel{
		apiClient:   apiClient,
		serviceName: serviceName,
		apiURL:      apiURL,
		useDev:      useDev,
		nav:         newNavModel("init"),
	}
}
