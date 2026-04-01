package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/yourusername/context.io/cli/internal/client"
	"github.com/yourusername/context.io/cli/internal/config"
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a natural language question about your observability data",
	Long: `Query your traces, services, and metrics using natural language.

The AI copilot analyzes your observability data and streams a response
with terminal-styled markdown formatting.

Examples:
  tracekit ask "why is latency high?"
  tracekit ask "show me errors in auth-service"
  tracekit ask "what happened in the last hour?"`,
	Args: cobra.ExactArgs(1),
	RunE: runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)
	askCmd.Flags().String("api-key", "", "TraceKit API key (overrides .env)")
	askCmd.Flags().String("url", "", "API base URL")
	askCmd.Flags().Bool("dev", false, "")
	askCmd.Flags().MarkHidden("dev")
}

// traceRef holds a parsed trace reference from tool_result events.
type traceRef struct {
	TraceID  string
	Service  string
	Duration string
}

func runAsk(cmd *cobra.Command, args []string) error {
	question := args[0]

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

	// Start spinner goroutine
	firstToken := make(chan struct{}, 1)
	var spinnerDone sync.WaitGroup
	spinnerDone.Add(1)
	go func() {
		defer spinnerDone.Done()
		frames := []string{"|", "/", "-", "\\"}
		thinkStyle := lipgloss.NewStyle().Foreground(cCyan)
		i := 0
		for {
			select {
			case <-firstToken:
				// Clear spinner line
				fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", 40))
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s", thinkStyle.Render(fmt.Sprintf(" %s Thinking...", frames[i%len(frames)])))
				i++
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	// Call copilot API
	resp, err := c.AskCopilot(cmd.Context(), question)
	if err != nil {
		close(firstToken)
		spinnerDone.Wait()
		return fmt.Errorf("copilot error: %w", err)
	}
	defer resp.Body.Close()

	// Parse SSE stream
	scanner := bufio.NewScanner(resp.Body)
	var eventType string
	var fullText strings.Builder
	var traceRefs []traceRef
	tokenClosed := false
	rawLines := 0

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
				// Decode JSON string token
				var token string
				if err := json.Unmarshal([]byte(dataLine), &token); err != nil {
					// Fallback: use raw data
					token = dataLine
				}

				// Signal first token to stop spinner
				if !tokenClosed {
					close(firstToken)
					spinnerDone.Wait()
					tokenClosed = true
				}

				fullText.WriteString(token)
				fmt.Print(token)
				rawLines += strings.Count(token, "\n")

			case "tool_result":
				// Parse tool result for trace references
				refs := parseToolResultTraces(dataLine)
				traceRefs = append(traceRefs, refs...)

			case "error":
				if !tokenClosed {
					close(firstToken)
					spinnerDone.Wait()
					tokenClosed = true
				}
				var errData map[string]string
				if err := json.Unmarshal([]byte(dataLine), &errData); err == nil {
					errMsg := errData["message"]
					if errMsg == "" {
						errMsg = dataLine
					}
					errStyle := lipgloss.NewStyle().Foreground(cDanger)
					fmt.Fprintln(os.Stderr, errStyle.Render("Error: "+errMsg))
					return fmt.Errorf("copilot error: %s", errMsg)
				}
				return fmt.Errorf("copilot error: %s", dataLine)

			case "done":
				goto streamDone
			}

			eventType = ""
			continue
		}
	}

streamDone:
	if !tokenClosed {
		close(firstToken)
		spinnerDone.Wait()
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("stream read error: %w", err)
	}

	text := fullText.String()
	if text == "" {
		return nil
	}

	// Re-render with glamour for polished markdown formatting
	fmt.Println() // Newline after raw stream

	rendered, err := renderMarkdown(text)
	if err == nil && rendered != "" {
		// Clear raw streamed output: move cursor up N+1 lines and clear
		clearLines := rawLines + 2
		fmt.Printf("\033[%dA\033[J", clearLines)
		fmt.Print(rendered)
	}

	// Colorize trace IDs in output (32-char hex strings)
	// This is handled inline via the glamour output

	// Print referenced traces section if any were collected
	if len(traceRefs) > 0 {
		fmt.Println()
		headerStyle := lipgloss.NewStyle().Foreground(cText).Bold(true)
		fmt.Println(headerStyle.Render("Referenced Traces:"))

		traceIDStyle := lipgloss.NewStyle().Foreground(cCyan)
		svcStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
		durStyle := lipgloss.NewStyle().Foreground(cWarning)

		for _, ref := range traceRefs {
			fmt.Printf("  [trace: %s | %s | %s]\n",
				traceIDStyle.Render(ref.TraceID),
				svcStyle.Render(ref.Service),
				durStyle.Render(ref.Duration),
			)
		}
	}

	return nil
}

// renderMarkdown renders markdown text with glamour terminal styling.
func renderMarkdown(text string) (string, error) {
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(80),
	)
	if err != nil {
		return "", err
	}

	out, err := r.Render(text)
	if err != nil {
		return "", err
	}

	// Post-process: colorize 32-char hex trace IDs with cyan
	traceIDRe := regexp.MustCompile(`\b([a-f0-9]{32})\b`)
	traceStyle := lipgloss.NewStyle().Foreground(cCyan)
	out = traceIDRe.ReplaceAllStringFunc(out, func(id string) string {
		return traceStyle.Render(id)
	})

	return out, nil
}

// parseToolResultTraces extracts trace references from a tool_result SSE event.
// The data is JSON: {"tool_id": "...", "result": "..."}
func parseToolResultTraces(data string) []traceRef {
	var payload struct {
		ToolID string `json:"tool_id"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		return nil
	}

	var refs []traceRef

	// Look for trace ID patterns in the result text
	traceIDRe := regexp.MustCompile(`([a-f0-9]{32})`)
	// Look for service names near "service" keyword
	serviceRe := regexp.MustCompile(`(?i)service[:\s]+([a-zA-Z0-9_-]+)`)
	// Look for duration patterns
	durationRe := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(ms|s|sec|milliseconds?)`)

	traceIDs := traceIDRe.FindAllString(payload.Result, -1)
	services := serviceRe.FindAllStringSubmatch(payload.Result, -1)
	durations := durationRe.FindAllString(payload.Result, -1)

	for i, id := range traceIDs {
		ref := traceRef{
			TraceID: id[:8], // Show abbreviated trace ID
		}
		if i < len(services) {
			ref.Service = services[i][1]
		} else {
			ref.Service = "unknown"
		}
		if i < len(durations) {
			ref.Duration = durations[i]
		} else {
			ref.Duration = "---"
		}
		refs = append(refs, ref)
	}

	return refs
}
