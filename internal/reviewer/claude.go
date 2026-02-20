package reviewer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/internal/config"
	"github.com/gerrit-ai-review/gerrit-tools/internal/logger"
	codereview "github.com/gerrit-ai-review/gerrit-tools/skills/code-review"
)

// ClaudeExecutor handles execution of the configured AI CLI for code review
type ClaudeExecutor struct {
	workDir   string
	cfg       *config.Config
	debugMode bool
	log       *logger.Logger
}

// StreamEvent represents a single event in the stream-json output
type StreamEvent struct {
	Type  string          `json:"type"`
	Event json.RawMessage `json:"event,omitempty"`
}

// StreamEventInner represents the inner event structure
type StreamEventInner struct {
	Type         string          `json:"type"`
	Index        int             `json:"index,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	ContentBlock ContentBlock    `json:"content_block,omitempty"`
}

// ContentBlock represents a content block in the message
type ContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// ToolInput represents the input to a tool
type ToolInput struct {
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	// Add other fields as needed
}

// NewClaudeExecutor creates a new Claude executor
func NewClaudeExecutor(workDir string, cfg *config.Config) *ClaudeExecutor {
	return &ClaudeExecutor{
		workDir:   workDir,
		cfg:       cfg,
		debugMode: true,
		log:       logger.Get(),
	}
}

// ExecuteReview runs the configured review CLI with the review prompt and returns the output
func (c *ClaudeExecutor) ExecuteReview(ctx context.Context, prompt string) (string, error) {
	// Apply timeout
	timeout := time.Duration(c.cfg.Review.ClaudeTimeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch c.reviewCLI() {
	case "codex":
		return c.executeCodexReview(ctx, prompt, timeout)
	default:
		return c.executeClaudeReview(ctx, prompt, timeout)
	}
}

func (c *ClaudeExecutor) executeClaudeReview(ctx context.Context, prompt string, timeout time.Duration) (string, error) {
	// Stream log is opt-in only because raw stream output may contain sensitive data.
	var streamLog *os.File
	if os.Getenv("GERRIT_REVIEWER_SAVE_CLAUDE_STREAM") == "1" {
		var err error
		streamLog, err = os.CreateTemp("", "claude-review-*-stream.jsonl")
		if err != nil {
			return "", fmt.Errorf("failed to create stream log file: %w", err)
		}
		defer streamLog.Close()
		c.log.Infof("Claude stream log enabled: %s", streamLog.Name())
	}

	// Build command arguments with stream-json output
	args := c.buildClaudeArgs(prompt)

	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = c.workDir

	// Inherit parent environment and add Gerrit-specific vars for gerrit-cli tool
	// Remove CLAUDECODE to avoid nested session error
	env := filterEnv(os.Environ(), "CLAUDECODE")
	cmd.Env = append(env, c.cfg.GerritEnvVars()...)

	// Get stdout pipe for reading stream
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	// Get stderr pipe for error messages
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("failed to start claude: %w", err)
	}

	// Read stderr in background
	var stderrOutput strings.Builder
	go func() {
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			line := stderrScanner.Text()
			stderrOutput.WriteString(line + "\n")
		}
	}()

	// Process stream line by line
	var assistantText strings.Builder
	var toolCallCount int
	var bashCallCount int

	scanner := bufio.NewScanner(stdout)
	// Increase buffer size for large JSON lines
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	for scanner.Scan() {
		line := scanner.Text()

		// Write raw line only when explicitly enabled.
		if streamLog != nil {
			if _, err := streamLog.WriteString(line + "\n"); err != nil {
				c.log.Warnf("Failed to write to stream log: %v", err)
			}
		}

		// Try to parse as JSON event
		var event StreamEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			// Not a JSON line, skip
			continue
		}

		// Check if it's a stream_event wrapper
		if event.Type == "stream_event" && len(event.Event) > 0 {
			// Parse inner event
			var innerEvent StreamEventInner
			if err := json.Unmarshal(event.Event, &innerEvent); err != nil {
				continue
			}

			// Handle different event types
			switch innerEvent.Type {
			case "content_block_start":
				// Check if it's a tool use
				if innerEvent.ContentBlock.Type == "tool_use" {
					toolCallCount++
					if innerEvent.ContentBlock.Name == "Bash" {
						bashCallCount++
						// Try to parse input
						var toolInput ToolInput
						if err := json.Unmarshal(innerEvent.ContentBlock.Input, &toolInput); err == nil {
							c.log.Debugf("[Tool #%d] Bash: %s", toolCallCount, truncate(toolInput.Command, 100))
						}
					} else {
						c.log.Debugf("[Tool #%d] %s (ID: %s)", toolCallCount, innerEvent.ContentBlock.Name, innerEvent.ContentBlock.ID)
					}
				}

			case "content_block_delta":
				// Check if it's text delta
				var deltaData struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
				}
				if err := json.Unmarshal(innerEvent.Delta, &deltaData); err == nil {
					if deltaData.Type == "text_delta" {
						assistantText.WriteString(deltaData.Text)
					}
				}

			case "message_stop":
				// Message completed
				c.log.Debugf("Claude message completed")
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error reading claude output: %w", err)
	}

	// Wait for command to finish
	if err := cmd.Wait(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("claude execution timed out after %v", timeout)
		}
		stderrLen := len(strings.TrimSpace(stderrOutput.String()))
		if stderrLen > 0 {
			return "", fmt.Errorf("claude execution failed: %w (stderr length: %d)", err, stderrLen)
		}
		return "", fmt.Errorf("claude execution failed: %w (no stderr output)", err)
	}

	c.log.Infof("Claude execution completed: %d tool calls (%d Bash)", toolCallCount, bashCallCount)

	return assistantText.String(), nil
}

func (c *ClaudeExecutor) executeCodexReview(ctx context.Context, prompt string, timeout time.Duration) (string, error) {
	outputFile, err := os.CreateTemp("", "codex-review-*-last-message.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create codex output file: %w", err)
	}
	outputPath := outputFile.Name()
	if err := outputFile.Close(); err != nil {
		return "", fmt.Errorf("failed to close codex output file: %w", err)
	}
	defer os.Remove(outputPath)

	args := c.buildCodexArgs(prompt, outputPath)
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = c.workDir

	// Remove CLAUDECODE to avoid nested-session issues if this process is called from Claude Code.
	env := filterEnv(os.Environ(), "CLAUDECODE")
	cmd.Env = append(env, c.cfg.GerritEnvVars()...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("codex execution timed out after %v", timeout)
		}
		stderrLen := len(strings.TrimSpace(string(output)))
		if stderrLen > 0 {
			return "", fmt.Errorf("codex execution failed: %w (combined output length: %d)", err, stderrLen)
		}
		return "", fmt.Errorf("codex execution failed: %w (no output)", err)
	}

	finalOutput, err := os.ReadFile(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to read codex output file: %w", err)
	}

	text := strings.TrimSpace(string(finalOutput))
	if text == "" {
		text = strings.TrimSpace(string(output))
	}

	return text, nil
}

func (c *ClaudeExecutor) buildClaudeArgs(prompt string) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--include-partial-messages",
		"--verbose",
	}

	if c.cfg.Review.ClaudeSkipPermissionsCheck {
		c.log.Warnf("Claude permission checks are disabled via --dangerously-skip-permissions")
		args = append(args, "--dangerously-skip-permissions")
	}

	return args
}

func (c *ClaudeExecutor) buildCodexArgs(prompt string, outputPath string) []string {
	args := []string{
		"exec",
		"--skip-git-repo-check",
		"--color", "never",
		"--output-last-message", outputPath,
	}

	if c.cfg.Review.ClaudeSkipPermissionsCheck {
		c.log.Warnf("Codex approvals and sandbox are disabled via --dangerously-bypass-approvals-and-sandbox")
		args = append(args, "--dangerously-bypass-approvals-and-sandbox")
	} else {
		args = append(args, "--full-auto")
	}

	args = append(args, prompt)

	return args
}

func (c *ClaudeExecutor) reviewCLI() string {
	cli := strings.ToLower(strings.TrimSpace(c.cfg.Review.CLI))
	if cli == "" {
		return "claude"
	}
	return cli
}

// BuildPrompt constructs the review prompt with change information
func (c *ClaudeExecutor) BuildPrompt(changeInfo ChangeInfo) (string, error) {
	skillContent, err := c.loadSkillContent()
	if err != nil {
		return "", err
	}

	cliCmd := "gerrit-cli"
	c.log.Debugf("Using Gerrit CLI command from PATH: %s", cliCmd)

	prompt := fmt.Sprintf(`%s

---

## Your Task

Review Gerrit change **%d** (Patchset %d) in project **%s**.

The `+"`gerrit-cli`"+` tool is available in PATH as: `+"`%s`"+`

Follow the review workflow described above. Start with Phase 1:

`+"```bash"+`
%s summary %d
`+"```"+`
`,
		string(skillContent),
		changeInfo.ChangeNumber,
		changeInfo.PatchsetNumber,
		changeInfo.Project,
		cliCmd,
		cliCmd,
		changeInfo.ChangeNumber,
	)

	return prompt, nil
}

func (c *ClaudeExecutor) loadSkillContent() (string, error) {
	c.log.Debugf("Using embedded skill content")
	skillContent, err := codereview.Content()
	if err != nil {
		return "", err
	}
	return skillContent, nil
}

// ChangeInfo contains information about the change being reviewed
type ChangeInfo struct {
	Project        string
	ChangeNumber   int
	PatchsetNumber int
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// filterEnv removes specified environment variables from the environment list
func filterEnv(env []string, keysToRemove ...string) []string {
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		keep := true
		for _, key := range keysToRemove {
			if strings.HasPrefix(e, key+"=") {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
