package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "AI copilot chat for your observability data",
	Long: `Interactive AI chat that analyzes your traces, services, and metrics.

Start a conversation with a question, or launch without arguments for interactive mode.

Examples:
  tracekit ask "why is latency high?"
  tracekit ask                          # interactive chat mode`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)
	askCmd.Flags().String("url", "", "API base URL")
	askCmd.Flags().Bool("dev", false, "")
	askCmd.Flags().MarkHidden("dev")
}

// -- Messages --

type chatConvCreatedMsg struct {
	convID string
}

type chatStreamStartMsg struct{}

type chatTokenMsg struct {
	token string
}

type chatStreamDoneMsg struct {
	fullText string
}

type chatErrMsg struct {
	err error
}

// -- Model --

type chatMessage struct {
	role     string // "user" or "assistant"
	content  string
	rendered string // glamour-rendered markdown (for assistant messages)
}

type askModel struct {
	apiClient      *client.Client
	width          int
	height         int
	quitting       bool
	nav            navModel
	navTarget      string

	// Conversation state
	convID         string
	messages       []chatMessage
	input          string
	streaming      bool
	streamBuffer   strings.Builder
	err            error

	// Scroll
	scrollOffset   int
}

func (m askModel) Init() tea.Cmd {
	return m.createConversationCmd()
}

func (m askModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case NavSwitchMsg:
		m.navTarget = msg.Command
		m.quitting = true
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case chatConvCreatedMsg:
		m.convID = msg.convID
		// If we have a pre-filled first question, send it
		if m.input != "" {
			question := m.input
			m.input = ""
			m.messages = append(m.messages, chatMessage{role: "user", content: question})
			m.streaming = true
			m.streamBuffer.Reset()
			return m, m.sendMessageCmd(question)
		}
		return m, nil

	case chatTokenMsg:
		m.streamBuffer.WriteString(msg.token)
		// Update the last assistant message in-place
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "assistant" {
			m.messages[len(m.messages)-1].content = m.streamBuffer.String()
		} else {
			m.messages = append(m.messages, chatMessage{role: "assistant", content: m.streamBuffer.String()})
		}
		// Auto-scroll to bottom
		m.scrollOffset = 0
		return m, nil

	case chatStreamDoneMsg:
		m.streaming = false
		// Render final markdown
		if len(m.messages) > 0 && m.messages[len(m.messages)-1].role == "assistant" {
			text := m.messages[len(m.messages)-1].content
			rendered, err := renderChatMarkdown(text)
			if err == nil {
				m.messages[len(m.messages)-1].rendered = rendered
			}
		}
		return m, nil

	case chatErrMsg:
		m.streaming = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		nav, consumed, navCmd := m.nav.HandleNavKey(msg)
		m.nav = nav
		if consumed {
			return m, navCmd
		}
		return m.handleKey(msg)
	}

	return m, nil
}

func (m askModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "esc":
		if m.streaming {
			return m, nil // can't cancel mid-stream
		}
		m.quitting = true
		return m, tea.Quit
	}

	// Don't accept input while streaming
	if m.streaming {
		return m, nil
	}

	// Don't accept input until conversation is created
	if m.convID == "" {
		return m, nil
	}

	switch key {
	case "enter":
		question := strings.TrimSpace(m.input)
		if question == "" {
			return m, nil
		}
		m.input = ""
		m.err = nil
		m.messages = append(m.messages, chatMessage{role: "user", content: question})
		m.streaming = true
		m.streamBuffer.Reset()
		m.scrollOffset = 0
		return m, m.sendMessageCmd(question)
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case "up":
		m.scrollOffset++
	case "down":
		if m.scrollOffset > 0 {
			m.scrollOffset--
		}
	default:
		if len(key) == 1 {
			m.input += key
		} else if key == "space" {
			m.input += " "
		}
	}

	return m, nil
}

// -- Commands --

func (m askModel) createConversationCmd() tea.Cmd {
	return func() tea.Msg {
		convID, err := m.apiClient.CreateConversation(context.Background())
		if err != nil {
			return chatErrMsg{err: err}
		}
		return chatConvCreatedMsg{convID: convID}
	}
}

func (m askModel) sendMessageCmd(content string) tea.Cmd {
	return func() tea.Msg {
		resp, err := m.apiClient.SendChatMessage(context.Background(), m.convID, content)
		if err != nil {
			return chatErrMsg{err: err}
		}

		// Parse SSE stream in a goroutine -- send tokens via Program
		// But since Bubbletea doesn't support channel-based streaming easily,
		// we'll collect the full response and send it as one message.
		// For perceived speed, we stream token-by-token via a sub-command loop.
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		var eventType string
		var fullText strings.Builder

		for scanner.Scan() {
			line := scanner.Text()

			if strings.HasPrefix(line, "event: ") {
				eventType = strings.TrimPrefix(line, "event: ")
				continue
			}

			if strings.HasPrefix(line, "data: ") {
				dataLine := strings.TrimPrefix(line, "data: ")

				switch eventType {
				case "text":
					var token string
					if err := json.Unmarshal([]byte(dataLine), &token); err != nil {
						token = dataLine
					}
					fullText.WriteString(token)
				case "error":
					var errData map[string]string
					if err := json.Unmarshal([]byte(dataLine), &errData); err == nil {
						if msg := errData["message"]; msg != "" {
							return chatErrMsg{err: fmt.Errorf(msg)}
						}
					}
					return chatErrMsg{err: fmt.Errorf(dataLine)}
				case "done":
					return chatStreamDoneMsg{fullText: fullText.String()}
				}
				eventType = ""
			}
		}

		return chatStreamDoneMsg{fullText: fullText.String()}
	}
}

// -- View --

func (m askModel) appendNavOverlay(content string) string {
	if overlay := m.nav.ViewNav(m.width); overlay != "" {
		return content + "\n" + overlay + "\n"
	}
	return content
}

func (m askModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}

	var b strings.Builder
	brand := lipgloss.Color("#6366f1")
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	userStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22c55e")).Bold(true)
	assistantStyle := lipgloss.NewStyle().Foreground(brand).Bold(true)
	text := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))

	// Header
	header := lipgloss.NewStyle().Foreground(brand).Bold(true).Render("  TraceKit Copilot")
	if m.convID != "" {
		header += dim.Render("  (conversation active)")
	} else {
		header += dim.Render("  (connecting...)")
	}
	b.WriteString("\n" + header + "\n")
	b.WriteString("  " + dim.Render(strings.Repeat("-", min(w-4, 70))) + "\n")

	// Messages area
	if len(m.messages) == 0 && !m.streaming && m.err == nil {
		b.WriteString("\n")
		b.WriteString("  " + dim.Render("Ask a question about your observability data.") + "\n")
		b.WriteString("  " + dim.Render("The AI has context about your traces, services, and metrics.") + "\n")
		b.WriteString("\n")
		b.WriteString("  " + dim.Render("Examples:") + "\n")
		b.WriteString("  " + text.Render("  \"why is auth-service latency increasing?\"") + "\n")
		b.WriteString("  " + text.Render("  \"show me errors in the last hour\"") + "\n")
		b.WriteString("  " + text.Render("  \"which services have the highest error rate?\"") + "\n")
	}

	for _, msg := range m.messages {
		b.WriteString("\n")
		if msg.role == "user" {
			b.WriteString("  " + userStyle.Render("You: ") + text.Render(msg.content) + "\n")
		} else {
			b.WriteString("  " + assistantStyle.Render("Copilot:") + "\n")
			if msg.rendered != "" {
				// Use glamour-rendered output
				for _, line := range strings.Split(msg.rendered, "\n") {
					b.WriteString("  " + line + "\n")
				}
			} else {
				// Still streaming -- show raw text
				content := msg.content
				if m.streaming {
					content += lipgloss.NewStyle().Foreground(brand).Render("_")
				}
				for _, line := range strings.Split(content, "\n") {
					b.WriteString("  " + text.Render(line) + "\n")
				}
			}
		}
	}

	// Error
	if m.err != nil {
		b.WriteString("\n")
		b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#ef4444")).Render("Error: "+m.err.Error()) + "\n")
	}

	// Input bar
	b.WriteString("\n")
	b.WriteString("  " + dim.Render(strings.Repeat("-", min(w-4, 70))) + "\n")

	if m.streaming {
		b.WriteString("  " + dim.Render("Streaming response...") + "\n")
	} else if m.convID == "" {
		b.WriteString("  " + dim.Render("Connecting...") + "\n")
	} else {
		cursor := lipgloss.NewStyle().Foreground(brand).Render("_")
		prompt := lipgloss.NewStyle().Foreground(brand).Bold(true).Render("> ")
		b.WriteString("  " + prompt + text.Render(m.input) + cursor + "\n")
	}

	// Footer
	b.WriteString("\n  " + dim.Render("enter send  esc quit  ") + NavHint())
	b.WriteString("\n")

	return m.appendNavOverlay(b.String())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderChatMarkdown renders markdown with glamour and colorizes trace IDs.
func renderChatMarkdown(text string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(70),
	)
	if err != nil {
		return "", err
	}

	out, err := r.Render(text)
	if err != nil {
		return "", err
	}

	// Colorize 32-char hex trace IDs
	traceIDRe := regexp.MustCompile(`\b([a-f0-9]{32})\b`)
	traceStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#06b6d4"))
	out = traceIDRe.ReplaceAllStringFunc(out, func(id string) string {
		return traceStyle.Render(id)
	})

	return strings.TrimRight(out, "\n"), nil
}

// -- Runner --

func runAsk(cmd *cobra.Command, args []string) error {
	c, err := NewAuthenticatedClient(cmd)
	if err != nil {
		return err
	}

	model := askModel{
		apiClient: c,
		nav:       newNavModel("ask"),
	}

	// Pre-fill first question if provided as argument
	if len(args) > 0 {
		model.input = args[0]
	}

	p := tea.NewProgram(model, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return fmt.Errorf("chat error: %w", err)
	}

	if m, ok := result.(askModel); ok && m.navTarget != "" {
		return RunNavTarget(m.navTarget)
	}

	return nil
}
