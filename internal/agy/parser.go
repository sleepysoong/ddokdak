package agy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type ToolCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type Step struct {
	StepIndex int        `json:"step_index"`
	Source    string     `json:"source"`
	Type      string     `json:"type"`
	Status    string     `json:"status"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

type ToolExecution struct {
	ToolName string
	Args     map[string]interface{}
	Output   string
	Success  bool
}

// ParseToolExecutions parses the transcript.jsonl file for the given conversation ID
// and extracts the tool calls and their outputs for the most recent turn.
func ParseToolExecutions(conversationID string) ([]ToolExecution, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}

	filePath := filepath.Join(home, ".gemini", "antigravity-cli", "brain", conversationID, ".system_generated", "logs", "transcript.jsonl")
	return parseToolExecutionsFromFile(filePath)
}

func parseToolExecutionsFromFile(filePath string) ([]ToolExecution, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open transcript: %w", err)
	}
	defer file.Close()

	var steps []Step
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var s Step
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue
		}
		steps = append(steps, s)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read transcript: %w", err)
	}

	// Find the last USER_INPUT step index
	lastUserInputIdx := -1
	for i := len(steps) - 1; i >= 0; i-- {
		if steps[i].Type == "USER_INPUT" {
			lastUserInputIdx = i
			break
		}
	}

	if lastUserInputIdx == -1 {
		return nil, nil // No user input found in this transcript
	}

	var executions []ToolExecution
	var pendingCalls []ToolCall

	for i := lastUserInputIdx + 1; i < len(steps); i++ {
		step := steps[i]
		if step.Type == "PLANNER_RESPONSE" {
			if len(step.ToolCalls) > 0 {
				pendingCalls = append(pendingCalls, step.ToolCalls...)
			}
		} else if step.Source == "MODEL" && len(pendingCalls) > 0 {
			// This is a tool execution output
			call := pendingCalls[0]
			pendingCalls = pendingCalls[1:]

			success := step.Status == "DONE"
			executions = append(executions, ToolExecution{
				ToolName: call.Name,
				Args:     call.Args,
				Output:   step.Content,
				Success:  success,
			})
		}
	}

	// Add any tool calls that were proposed but not yet executed (or failed before output)
	for _, call := range pendingCalls {
		executions = append(executions, ToolExecution{
			ToolName: call.Name,
			Args:     call.Args,
			Output:   "No output (execution pending or interrupted)",
			Success:  false,
		})
	}

	return executions, nil
}
