package session

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// SessionManager manages all active sessions, keyed by Discord thread ID.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	dataPath string
}

// sessionData is used for JSON serialization.
type sessionData struct {
	ID             string `json:"id"`
	ThreadID       string `json:"thread_id"`
	ConversationID string `json:"conversation_id"`
	Model          string `json:"model"`
}

// NewSessionManager creates and returns a new SessionManager.
// It also loads any existing sessions from disk.
func NewSessionManager(dataDir string) *SessionManager {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Failed to create data directory: %v", err)
	}
	
	m := &SessionManager{
		sessions: make(map[string]*Session),
		dataPath: filepath.Join(dataDir, "sessions.json"),
	}
	m.load()
	return m
}

// load reads sessions from the JSON file.
func (m *SessionManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.dataPath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to read sessions file: %v", err)
		}
		return
	}

	var stored map[string]sessionData
	if err := json.Unmarshal(data, &stored); err != nil {
		log.Printf("Failed to unmarshal sessions: %v", err)
		return
	}

	for threadID, sd := range stored {
		s := NewSession(threadID)
		s.ID = sd.ID
		s.ConversationID = sd.ConversationID
		s.Model = sd.Model
		m.sessions[threadID] = s
	}
	log.Printf("Loaded %d sessions from disk", len(stored))
}

// Save writes all current sessions to the JSON file.
func (m *SessionManager) Save() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stored := make(map[string]sessionData)
	for threadID, s := range m.sessions {
		// Acquire session lock briefly to read its data
		s.mu.Lock()
		stored[threadID] = sessionData{
			ID:             s.ID,
			ThreadID:       s.ThreadID,
			ConversationID: s.ConversationID,
			Model:          s.Model,
		}
		s.mu.Unlock()
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		log.Printf("Failed to marshal sessions: %v", err)
		return
	}

	if err := os.WriteFile(m.dataPath, data, 0644); err != nil {
		log.Printf("Failed to write sessions file: %v", err)
	}
}

// CreateSession creates a new session for the given thread ID and stores it.
// If a session already exists for the thread ID, it is replaced.
func (m *SessionManager) CreateSession(threadID string) *Session {
	s := NewSession(threadID)

	m.mu.Lock()
	m.sessions[threadID] = s
	m.mu.Unlock()

	m.Save()
	return s
}

// GetSession retrieves the session associated with the given thread ID.
// It returns the session and true if found, or nil and false otherwise.
func (m *SessionManager) GetSession(threadID string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[threadID]
	return s, ok
}

// RemoveSession removes the session associated with the given thread ID.
// If no session exists for the thread ID, this is a no-op.
func (m *SessionManager) RemoveSession(threadID string) {
	m.mu.Lock()
	delete(m.sessions, threadID)
	m.mu.Unlock()

	m.Save()
}

