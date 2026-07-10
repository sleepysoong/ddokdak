package agy

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// SessionInfo represents a summarized past conversation
type SessionInfo struct {
	ID    string
	Title string
	MTime int64
}

// GetRecentSessions returns up to 'limit' recent agy sessions from the local computer.
func GetRecentSessions(limit int) ([]SessionInfo, error) {
	// Usually stored in ~/.gemini/antigravity-cli/brain
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	brainDir := filepath.Join(homeDir, ".gemini", "antigravity-cli", "brain")

	entries, err := os.ReadDir(brainDir)
	if err != nil {
		return nil, err
	}

	var sessions []SessionInfo
	tagRegex := regexp.MustCompile(`<[^>]+>`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		uuid := entry.Name()
		if len(uuid) != 36 { // Not a valid UUID length
			continue
		}

		logPath := filepath.Join(brainDir, uuid, ".system_generated", "logs", "transcript.jsonl")
		file, err := os.Open(logPath)
		if err != nil {
			continue
		}

		var title string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			var step struct {
				Type    string `json:"type"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(scanner.Bytes(), &step); err == nil {
				if step.Type == "USER_INPUT" {
					title = step.Content
					title = tagRegex.ReplaceAllString(title, "")
					title = strings.TrimSpace(title)
					if title == "" {
						title = "빈 대화"
					}
					
					// truncate safely
					runes := []rune(title)
					if len(runes) > 40 {
						title = string(runes[:37]) + "..."
					}
					break
				}
			}
		}
		file.Close()

		if title == "" {
			title = "알 수 없는 대화"
		}

		info, err := entry.Info()
		if err == nil {
			sessions = append(sessions, SessionInfo{
				ID:    uuid,
				Title: title,
				MTime: info.ModTime().Unix(),
			})
		}
	}

	// Sort descending by mod time
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].MTime > sessions[j].MTime
	})

	if len(sessions) > limit {
		sessions = sessions[:limit]
	}

	return sessions, nil
}
