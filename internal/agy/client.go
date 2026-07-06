// Package agy provides a client for interacting with the Antigravity CLI (agy).
package agy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Client wraps the agy CLI to send prompts and receive AI responses.
type Client struct {
	timeout string
	logDir  string
}

// NewClient creates a new agy CLI client.
//
// model is the AI model name to use (e.g. "gemini-2.5-pro").
// timeout is the print timeout duration string (e.g. "120").
// logDir is the directory where per-thread log files are stored.
func NewClient(timeout, logDir string) *Client {
	return &Client{
		timeout: timeout,
		logDir:  logDir,
	}
}

// buildCommand constructs the exec.Cmd for the agy CLI invocation.
// threadID is used to derive the log file name.
// If conversationID is non-empty, the --conversation flag is appended.
func (c *Client) buildCommand(ctx context.Context, prompt, model, threadID, conversationID string) *exec.Cmd {
	logFile := filepath.Join(c.logDir, threadID+".log")

	args := []string{
		"--model", model,
		"--print-timeout", c.timeout,
		"--log-file", logFile,
		"--dangerously-skip-permissions",
	}

	if conversationID != "" {
		args = append(args, "--conversation", conversationID)
	}

	args = append(args, "-p", prompt)

	return exec.CommandContext(ctx, "agy", args...)
}

// parseResponse extracts the AI response text from stdout output.
// In -p (--print) mode agy writes only the response to stdout,
// so this simply trims surrounding whitespace.
func parseResponse(stdout string) string {
	return strings.TrimSpace(stdout)
}

// Execute runs an agy CLI invocation and returns the AI response.
//
// prompt is the user message to send.
// conversationID should be empty for a new conversation, or a previously
// returned conversation ID to continue an existing one.
// threadID is used to name the per-thread log file.
//
// It returns the response text, the conversation ID (equal to threadID for
// tracking purposes), and any error that occurred.
func (c *Client) Execute(ctx context.Context, prompt, model, conversationID, threadID string) (response string, newConversationID string, err error) {
	if prompt == "" {
		return "", "", fmt.Errorf("agy: prompt must not be empty")
	}

	if threadID == "" {
		return "", "", fmt.Errorf("agy: threadID must not be empty")
	}

	cmd := c.buildCommand(ctx, prompt, model, threadID, conversationID)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderrBuf.String())
		if stderrText != "" {
			return "", "", fmt.Errorf("agy: command failed: %w: %s", err, stderrText)
		}
		return "", "", fmt.Errorf("agy: command failed: %w", err)
	}

	response = parseResponse(stdoutBuf.String())
	if response == "" {
		return "", "", fmt.Errorf("agy: received empty response")
	}

	// Use threadID as the conversation identifier for log-based tracking.
	newConversationID = threadID

	return response, newConversationID, nil
}

// ExecuteWithContinuation runs an agy CLI invocation that continues
// an existing conversation. It is a convenience wrapper around Execute
// that requires a non-empty conversationID.
func (c *Client) ExecuteWithContinuation(ctx context.Context, prompt, model, conversationID, threadID string) (string, string, error) {
	if conversationID == "" {
		return "", "", fmt.Errorf("agy: conversationID is required for continuation")
	}

	return c.Execute(ctx, prompt, model, conversationID, threadID)
}
