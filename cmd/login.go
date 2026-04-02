package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
	"github.com/yourusername/context.io/cli/internal/utils"
)

// -- Login Bubbletea messages --

type loginRegisteredMsg struct {
	resp *client.RegisterResponse
}

type loginVerifiedMsg struct {
	resp *client.VerifyResponse
}

type loginSavedMsg struct{}

type loginErrMsg struct {
	err error
}

// -- Login wizard model --

type loginWizardModel struct {
	apiClient *client.Client
	width     int
	height    int
	step      int // 0=email, 1=verify, 2=complete
	quitting  bool
	err       error

	// Step 0: Email
	email       string
	serviceName string

	// Step 1: Verify
	sessionID string
	code      string

	// Step 2: Complete
	verifyResp *client.VerifyResponse
	savePath   string
	saved      bool

	// Flags
	apiURL string
	useDev bool
}

var loginSteps = []string{"Email", "Verify", "Complete"}

func (m loginWizardModel) Init() tea.Cmd {
	return nil
}

// -- Update --

func (m loginWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case loginRegisteredMsg:
		m.sessionID = msg.resp.SessionID
		m.err = nil
		m.step = 1
		return m, nil

	case loginVerifiedMsg:
		m.verifyResp = msg.resp
		m.err = nil
		m.step = 2
		// Auto-save to global config
		return m, m.saveConfigCmd()

	case loginSavedMsg:
		m.saved = true
		return m, nil

	case loginErrMsg:
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m loginWizardModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		if m.step == 1 {
			m.err = nil
			m.code = ""
			m.step = 0
		}
		return m, nil
	}

	switch m.step {
	case 0:
		return m.handleEmailKey(msg)
	case 1:
		return m.handleVerifyKey(msg)
	case 2:
		return m.handleCompleteKey(msg)
	}

	return m, nil
}

func (m loginWizardModel) handleEmailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m loginWizardModel) handleVerifyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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

func (m loginWizardModel) handleCompleteKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q":
		m.quitting = true
		return m, tea.Quit
	}
	return m, nil
}

// -- Tea commands --

func (m loginWizardModel) registerCmd() tea.Cmd {
	return func() tea.Msg {
		req := &client.RegisterRequest{
			Email:            m.email,
			OrganizationName: m.serviceName,
			ServiceName:      m.serviceName,
			Source:           "cli_login",
			SourceMetadata: map[string]interface{}{
				"cli_version": CLIVersion,
				"platform":    runtime.GOOS + "_" + runtime.GOARCH,
			},
		}

		resp, err := m.apiClient.Register(req)
		if err != nil {
			return loginErrMsg{err: fmt.Errorf("login failed: %w", err)}
		}
		return loginRegisteredMsg{resp: resp}
	}
}

func (m loginWizardModel) verifyCmd() tea.Cmd {
	return func() tea.Msg {
		req := &client.VerifyRequest{
			SessionID: m.sessionID,
			Code:      m.code,
		}

		resp, err := m.apiClient.Verify(req)
		if err != nil {
			return loginErrMsg{err: fmt.Errorf("verification failed: %w", err)}
		}
		return loginVerifiedMsg{resp: resp}
	}
}

func (m loginWizardModel) saveConfigCmd() tea.Cmd {
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
			return loginErrMsg{err: fmt.Errorf("failed to save config: %w", err)}
		}
		return loginSavedMsg{}
	}
}

// -- View --

func (m loginWizardModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	w := m.width
	if w == 0 {
		w = 80
	}

	// ASCII banner
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

	// Step indicator
	b.WriteString(" " + renderLoginSteps(m.step, w) + "\n\n")

	// Step content
	switch m.step {
	case 0:
		b.WriteString(m.viewEmail(w))
	case 1:
		b.WriteString(m.viewVerify(w))
	case 2:
		b.WriteString(m.viewComplete(w))
	}

	// Error
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444"))
		b.WriteString("\n " + errStyle.Render(m.err.Error()))
		b.WriteString("\n " + lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render("Press r to retry"))
	}

	// Footer
	b.WriteString("\n\n")
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	if m.step < 2 {
		b.WriteString(" " + footer.Render("enter submit  esc back  ctrl+c quit"))
	} else {
		b.WriteString(" " + footer.Render("enter/q quit"))
	}
	b.WriteString("\n")

	return b.String()
}

func renderLoginSteps(current int, w int) string {
	var parts []string
	for i, name := range loginSteps {
		style := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
		if i < current {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
			parts = append(parts, style.Render("["+fmt.Sprintf("%d", i+1)+"] "+name+" ✓"))
		} else if i == current {
			style = lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1")).Bold(true)
			parts = append(parts, style.Render("["+fmt.Sprintf("%d", i+1)+"] "+name))
		} else {
			parts = append(parts, style.Render("["+fmt.Sprintf("%d", i+1)+"] "+name))
		}
	}
	sep := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280")).Render(" > ")
	return strings.Join(parts, sep)
}

func (m loginWizardModel) viewEmail(w int) string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	b.WriteString(" " + text.Bold(true).Render("Enter your email address") + "\n\n")
	b.WriteString(" " + dim.Render("Email: ") + text.Render(m.email) + text.Render("_") + "\n")

	return b.String()
}

func (m loginWizardModel) viewVerify(w int) string {
	var b strings.Builder
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))

	b.WriteString(" " + success.Render("Verification code sent to "+m.email) + "\n\n")
	b.WriteString(" " + text.Bold(true).Render("Enter the 6-digit code from your email") + "\n\n")

	// Render code boxes
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
	b.WriteString(" " + dim.Render("Code: ") + boxes + "\n")

	return b.String()
}

func (m loginWizardModel) viewComplete(w int) string {
	var b strings.Builder
	success := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e"))
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	brand := lipgloss.NewStyle().Foreground(lipgloss.Color("#6366f1"))

	b.WriteString(" " + success.Bold(true).Render("Login successful!") + "\n\n")

	if m.verifyResp != nil {
		b.WriteString(" " + dim.Render("Dashboard:  ") + brand.Render(m.verifyResp.DashboardURL) + "\n")
		b.WriteString(" " + dim.Render("API Key:    ") + text.Render(utils.MaskAPIKey(m.verifyResp.APIKey)) + "\n")
		b.WriteString(" " + dim.Render("Service:    ") + text.Render(m.verifyResp.ServiceName) + "\n")
	}

	if m.saved {
		b.WriteString("\n " + success.Render("Saved to "+m.savePath) + "\n")
	}

	b.WriteString("\n " + dim.Render("Next steps:") + "\n")
	b.WriteString(" " + text.Render("  tracekit dashboard") + dim.Render("   -- view live dashboard") + "\n")
	b.WriteString(" " + text.Render("  tracekit traces") + dim.Render("      -- browse traces") + "\n")
	b.WriteString(" " + text.Render("  tracekit alerts") + dim.Render("      -- manage alerts") + "\n")

	return b.String()
}

// -- Cobra command --

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to existing TraceKit account",
	Long: `Login to your existing TraceKit account and generate a new API key.

This command will:
  1. Verify your email with a verification code
  2. Generate a new API key for your organization
  3. Save configuration to ~/.tracekitconfig

All CLI commands will automatically find your API key from ~/.tracekitconfig,
so you don't need to pass it every time.

Example:
  tracekit login
  tracekit login --email=dev@example.com`,
	RunE: runLogin,
}

func init() {
	rootCmd.AddCommand(loginCmd)
	loginCmd.Flags().String("email", "", "Your email address")
	loginCmd.Flags().String("api-url", "", "API base URL (default: https://app.tracekit.dev)")
	loginCmd.Flags().Bool("dev", false, "")
	loginCmd.Flags().MarkHidden("dev")
}

func runLogin(cmd *cobra.Command, args []string) error {
	email, _ := cmd.Flags().GetString("email")
	apiURL, _ := cmd.Flags().GetString("api-url")
	useDev, _ := cmd.Flags().GetBool("dev")

	if useDev {
		apiURL = client.DevBaseURL
	}

	apiClient := client.NewClient(apiURL)

	cwd, _ := os.Getwd()
	serviceName := strings.ToLower(strings.ReplaceAll(filepath.Base(cwd), " ", "-"))

	model := loginWizardModel{
		apiClient:   apiClient,
		email:       email,
		serviceName: serviceName,
		savePath:    config.GlobalConfigPath(),
		apiURL:      apiURL,
		useDev:      useDev,
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("login error: %w", err)
	}

	return nil
}
