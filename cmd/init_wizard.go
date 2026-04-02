package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/detector"
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

// -- Init wizard model --

type initWizardModel struct {
	apiClient *client.Client
	width     int
	height    int
	step      int // 0=account, 1=verify, 2=config, 3=complete
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

	// Step 2: Config (auto-runs: save config, test trace)
	verifyResp *client.VerifyResponse
	configDone bool
	testDone   bool
	configErr  string

	// Step 3: Complete
	dashboardURL string
	apiKeyMasked string

	// Flags from cobra
	apiURL string
	useDev bool
}

var initWizardSteps = []string{"Account", "Verify", "Config", "Complete"}

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
		// Auto-trigger config save
		return m, m.saveConfigCmd()

	case initConfigSavedMsg:
		m.configDone = true
		m.testDone = true
		m.dashboardURL = m.verifyResp.DashboardURL
		m.apiKeyMasked = utils.MaskAPIKey(m.verifyResp.APIKey)
		m.step = 3
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
		if m.step > 0 && m.step < 3 {
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
		// Config step is auto-executing, no key handling needed
		return m, nil
	case 3:
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

func (m initWizardModel) handleCompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "enter", "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
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
			Endpoint:              m.apiClient.BaseURL,
			ServiceName:           m.serviceName,
			Enabled:               "true",
			CodeMonitoringEnabled: "true",
		}

		if err := config.Save(cfg); err != nil {
			return initErrMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}

		if err := sendTestTraceInternal(cfg); err != nil {
			// Non-fatal: config saved but test trace failed
			return initConfigSavedMsg{}
		}

		return initConfigSavedMsg{}
	}
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
		b.WriteString(m.renderConfigStep())
	case 3:
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
	if m.step < 3 {
		footer := lipgloss.NewStyle().Foreground(cDim).Render(" Esc back | Ctrl+C quit")
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

func (m initWizardModel) renderConfigStep() string {
	var b strings.Builder

	checkDone := lipgloss.NewStyle().Foreground(cSuccess).Render("*")
	checkPending := lipgloss.NewStyle().Foreground(cMuted).Render("...")

	// Config save status
	if m.configDone {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(checkDone + " Configuration saved"))
	} else {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(checkPending + " Saving configuration..."))
	}
	b.WriteString("\n")

	// Test trace status
	if m.testDone {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(checkDone + " Test trace sent"))
	} else if m.configDone {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(checkPending + " Sending test trace..."))
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

	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Dashboard:  ") + valStyle.Render(m.dashboardURL)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("API Key:    ") + valStyle.Render(m.apiKeyMasked)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
		dimLabel.Render("Service:    ") + valStyle.Render(m.serviceName)))
	b.WriteString("\n")

	if m.framework != nil && m.framework.Name != "generic" {
		b.WriteString(lipgloss.NewStyle().Padding(0, 2).Render(
			dimLabel.Render("Framework:  ") + valStyle.Render(m.framework.Name)))
		b.WriteString("\n")
	}

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
