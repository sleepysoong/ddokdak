package agy

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type Telemetry struct {
	Model        string `json:"model"`
	Pct          int    `json:"pct"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
}

// ParseTelemetry reads and decodes the telemetry file for a given conversation ID.
func ParseTelemetry(conversationID string) (*Telemetry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("user home dir: %w", err)
	}

	filePath := filepath.Join(home, ".gemini", "antigravity-cli", fmt.Sprintf("telemetry_%s.json", conversationID))
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open telemetry: %w", err)
	}
	defer file.Close()

	var t Telemetry
	if err := json.NewDecoder(file).Decode(&t); err != nil {
		return nil, fmt.Errorf("decode telemetry: %w", err)
	}
	return &t, nil
}

// FormatInline은 도구 실행 내역을 한 줄의 읽기 편한 문자열로 포맷팅합니다.
func (exec ToolExecution) FormatInline() string {
	// 우선순위가 높은 공통 인자 Key 목록
	priorityKeys := []string{
		"CommandLine", "command", "cmd",
		"AbsolutePath", "TargetFile", "Target", "path", "file", "filename",
		"Query", "query", "q",
		"Url", "url", "uri",
		"name", "Recipient",
	}

	var argVal interface{}
	// 1. 우선순위 목록에서 먼저 매칭되는 Key를 찾음
	for _, key := range priorityKeys {
		if val, exists := exec.Args[key]; exists {
			argVal = val
			break
		}
	}

	// 2. 만약 매칭되는 우선순위 Key가 없으면, 모든 인자를 key=value 형태로 결합 (시스템 인자 제외)
	if argVal == nil && len(exec.Args) > 0 {
		var parts []string
		for k, v := range exec.Args {
			if k == "toolAction" || k == "toolSummary" || k == "IsSkillFile" || k == "IsMock" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		if len(parts) > 0 {
			sort.Strings(parts)
			argVal = strings.Join(parts, ", ")
		}
	}

	if argVal != nil {
		valStr := stripQuotes(fmt.Sprintf("%v", argVal))
		if len(valStr) > 60 {
			valStr = valStr[:57] + "..."
		}
		return fmt.Sprintf("`%s(%s)`", exec.ToolName, valStr)
	}
	return fmt.Sprintf("`%s`", exec.ToolName)
}

func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

