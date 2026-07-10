package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type Step struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type SessionInfo struct {
	UUID  string
	Title string
	MTime int64
}

func main() {
	brainDir := "/root/.gemini/antigravity-cli/brain"
	entries, err := os.ReadDir(brainDir)
	if err != nil {
		panic(err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		uuid := entry.Name()
		if len(uuid) != 36 { // Not a UUID
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
			var step Step
			if err := json.Unmarshal(scanner.Bytes(), &step); err == nil {
				if step.Type == "USER_INPUT" {
					title = step.Content
					if len(title) > 50 {
						title = title[:47] + "..."
					}
					break
				}
			}
		}
		file.Close()
		
		info, err := entry.Info()
		if err == nil {
			sessions = append(sessions, SessionInfo{
				UUID:  uuid,
				Title: title,
				MTime: info.ModTime().Unix(),
			})
		}
	}
	
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].MTime > sessions[j].MTime
	})
	
	for i, s := range sessions {
		if i > 5 { break }
		fmt.Printf("[%s] %s\n", s.UUID, s.Title)
	}
}
