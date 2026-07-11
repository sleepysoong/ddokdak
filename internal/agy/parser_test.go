package agy

import (
	"encoding/json"
	"os"
	"testing"
)

func TestParseToolExecutionsFromFile(t *testing.T) {
	// Create mock transcript.jsonl
	tmpFile, err := os.CreateTemp("", "transcript_test_*.jsonl")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	mockSteps := []Step{
		{
			StepIndex: 0,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			Content:   "첫 질문",
		},
		{
			StepIndex: 1,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			ToolCalls: []ToolCall{
				{
					Name: "view_file",
					Args: map[string]interface{}{"path": "/tmp/a"},
				},
			},
		},
		{
			StepIndex: 2,
			Source:    "MODEL",
			Type:      "VIEW_FILE",
			Status:    "DONE",
			Content:   "첫 질문의 파일 데이터",
		},
		{
			StepIndex: 3,
			Source:    "USER_EXPLICIT",
			Type:      "USER_INPUT",
			Status:    "DONE",
			Content:   "두 번째 질문",
		},
		{
			StepIndex: 4,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
			ToolCalls: []ToolCall{
				{
					Name: "run_command",
					Args: map[string]interface{}{"cmd": "ls"},
				},
			},
		},
		{
			StepIndex: 5,
			Source:    "MODEL",
			Type:      "RUN_COMMAND",
			Status:    "DONE",
			Content:   "bin\nsrc",
		},
		{
			StepIndex: 6,
			Source:    "MODEL",
			Type:      "PLANNER_RESPONSE",
			Status:    "DONE",
		},
	}

	for _, step := range mockSteps {
		data, err := json.Marshal(step)
		if err != nil {
			t.Fatalf("failed to marshal step: %v", err)
		}
		if _, err := tmpFile.Write(append(data, '\n')); err != nil {
			t.Fatalf("failed to write step to temp file: %v", err)
		}
	}

	executions, err := parseToolExecutionsFromFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ParseToolExecutions failed: %v", err)
	}

	if len(executions) != 1 {
		t.Fatalf("expected 1 execution, got %d", len(executions))
	}

	exec := executions[0]
	if exec.ToolName != "run_command" {
		t.Errorf("expected tool_name 'run_command', got %q", exec.ToolName)
	}

	cmdArg, ok := exec.Args["cmd"].(string)
	if !ok || cmdArg != "ls" {
		t.Errorf("expected arg cmd 'ls', got %v", exec.Args["cmd"])
	}

	if exec.Output != "bin\nsrc" {
		t.Errorf("expected output 'bin\nsrc', got %q", exec.Output)
	}

	if !exec.Success {
		t.Errorf("expected success to be true")
	}
}
