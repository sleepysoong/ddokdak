package agy

import (
	"context"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient("120", "/tmp/logs")
	if c.timeout != "120" {
		t.Errorf("expected timeout '120', got %q", c.timeout)
	}
	if c.logDir != "/tmp/logs" {
		t.Errorf("expected logDir '/tmp/logs', got %q", c.logDir)
	}
}

func TestBuildCommand_NewConversation(t *testing.T) {
	c := NewClient("120", "/var/log/ddokdak")
	ctx := context.Background()

	cmd := c.buildCommand(ctx, "Hello, world!", "gemini-2.5-pro", "thread-abc-123", "")

	args := cmd.Args
	// args[0] is the binary path; the rest are CLI flags.
	joined := strings.Join(args[1:], " ")

	expected := []string{
		"--model", "gemini-2.5-pro",
		"--print-timeout", "120",
		"--log-file", "/var/log/ddokdak/thread-abc-123.log",
		"--dangerously-skip-permissions",
		"-p", "Hello, world!",
	}
	expectedStr := strings.Join(expected, " ")

	if joined != expectedStr {
		t.Errorf("unexpected command args:\n  got:  %s\n  want: %s", joined, expectedStr)
	}

	// --conversation should NOT be present for a new conversation.
	for _, a := range args {
		if a == "--conversation" {
			t.Error("--conversation flag should not be present when conversationID is empty")
		}
	}
}

func TestBuildCommand_WithConversation(t *testing.T) {
	c := NewClient("60", "/logs")
	ctx := context.Background()

	cmd := c.buildCommand(ctx, "Follow-up question", "gemini-2.5-flash", "thread-xyz", "conv-id-999")

	args := cmd.Args

	// Verify --conversation flag is present with correct value.
	foundConv := false
	for i, a := range args {
		if a == "--conversation" {
			if i+1 >= len(args) {
				t.Fatal("--conversation flag present but no value follows")
			}
			if args[i+1] != "conv-id-999" {
				t.Errorf("expected conversation ID 'conv-id-999', got %q", args[i+1])
			}
			foundConv = true
			break
		}
	}
	if !foundConv {
		t.Error("--conversation flag not found when conversationID is provided")
	}

	// Verify prompt is present.
	foundPrompt := false
	for i, a := range args {
		if a == "-p" {
			if i+1 >= len(args) {
				t.Fatal("-p flag present but no value follows")
			}
			if args[i+1] != "Follow-up question" {
				t.Errorf("expected prompt 'Follow-up question', got %q", args[i+1])
			}
			foundPrompt = true
			break
		}
	}
	if !foundPrompt {
		t.Error("-p flag not found")
	}
}

func TestBuildCommand_LogFilePath(t *testing.T) {
	c := NewClient("30", "/data/agy-logs")
	ctx := context.Background()

	cmd := c.buildCommand(ctx, "test prompt", "model-x", "my-thread-id", "")

	args := cmd.Args
	foundLogFile := false
	for i, a := range args {
		if a == "--log-file" {
			if i+1 >= len(args) {
				t.Fatal("--log-file flag present but no value follows")
			}
			expected := "/data/agy-logs/my-thread-id.log"
			if args[i+1] != expected {
				t.Errorf("expected log file %q, got %q", expected, args[i+1])
			}
			foundLogFile = true
			break
		}
	}
	if !foundLogFile {
		t.Error("--log-file flag not found")
	}
}

func TestParseResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean response",
			input:    "This is the AI response.",
			expected: "This is the AI response.",
		},
		{
			name:     "response with leading/trailing whitespace",
			input:    "\n  Hello, I can help with that.\n\n",
			expected: "Hello, I can help with that.",
		},
		{
			name:     "multi-line response",
			input:    "Line 1\nLine 2\nLine 3",
			expected: "Line 1\nLine 2\nLine 3",
		},
		{
			name:     "empty response",
			input:    "",
			expected: "",
		},
		{
			name:     "only whitespace",
			input:    "   \n\t  \n  ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseResponse(tt.input)
			if got != tt.expected {
				t.Errorf("parseResponse(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestExecute_EmptyPrompt(t *testing.T) {
	c := NewClient("60", "/logs")
	ctx := context.Background()

	_, _, _, err := c.Execute(ctx, "", "model", "", "thread-1")
	if err == nil {
		t.Error("expected error for empty prompt, got nil")
	}
	if !strings.Contains(err.Error(), "prompt must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExecute_EmptyThreadID(t *testing.T) {
	c := NewClient("60", "/logs")
	ctx := context.Background()

	_, _, _, err := c.Execute(ctx, "hello", "model", "", "")
	if err == nil {
		t.Error("expected error for empty threadID, got nil")
	}
	if !strings.Contains(err.Error(), "threadID must not be empty") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExecuteWithContinuation_EmptyConversationID(t *testing.T) {
	c := NewClient("60", "/logs")
	ctx := context.Background()

	_, _, _, err := c.ExecuteWithContinuation(ctx, "hello", "model", "", "thread-1")
	if err == nil {
		t.Error("expected error for empty conversationID, got nil")
	}
	if !strings.Contains(err.Error(), "conversationID is required") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBuildCommand_BinaryName(t *testing.T) {
	c := NewClient("60", "/logs")
	ctx := context.Background()

	cmd := c.buildCommand(ctx, "test", "model", "thread-1", "")

	// The binary should be "agy".
	if cmd.Path == "" {
		// Path may be empty if agy is not installed; check Args[0] instead.
		if len(cmd.Args) == 0 || cmd.Args[0] != "agy" {
			t.Errorf("expected command binary 'agy', got args: %v", cmd.Args)
		}
	}
}

func TestBuildCommand_FlagOrder(t *testing.T) {
	c := NewClient("90", "/var/logs")
	ctx := context.Background()

	cmd := c.buildCommand(ctx, "my prompt", "test-model", "tid", "cid")
	args := cmd.Args[1:] // skip binary

	// --model should come before -p
	modelIdx := -1
	pIdx := -1
	convIdx := -1
	for i, a := range args {
		switch a {
		case "--model":
			modelIdx = i
		case "-p":
			pIdx = i
		case "--conversation":
			convIdx = i
		}
	}

	if modelIdx == -1 {
		t.Fatal("--model flag not found")
	}
	if pIdx == -1 {
		t.Fatal("-p flag not found")
	}
	if convIdx == -1 {
		t.Fatal("--conversation flag not found")
	}
	if modelIdx > pIdx {
		t.Error("--model should come before -p")
	}
	if convIdx > pIdx {
		t.Error("--conversation should come before -p")
	}
}
