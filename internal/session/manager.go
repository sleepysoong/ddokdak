package session

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// SessionManager manages all active sessions, keyed by Discord thread ID.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
	db       *sql.DB
}

// sessionData is used for JSON serialization.
type sessionData struct {
	ID             string `json:"id"`
	ThreadID       string `json:"thread_id"`
	ConversationID string `json:"conversation_id"`
	Model          string `json:"model"`
}

// NewSessionManager creates and returns a new SessionManager.
// It also loads any existing sessions from the SQLite database.
func NewSessionManager(dataDir string) *SessionManager {
	if dataDir == "" {
		dataDir = filepath.Join(".", "data")
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("Failed to create data directory: %v", err)
	}
	
	dbPath := filepath.Join(dataDir, "ddokdak.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			thread_id TEXT PRIMARY KEY,
			id TEXT,
			conversation_id TEXT,
			model TEXT
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create sessions table: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS token_usages (
			thread_id TEXT,
			model TEXT,
			calls INTEGER DEFAULT 0,
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			PRIMARY KEY(thread_id, model)
		)
	`)
	if err != nil {
		log.Fatalf("Failed to create token_usages table: %v", err)
	}

	m := &SessionManager{
		sessions: make(map[string]*Session),
		db:       db,
	}
	m.load()
	return m
}

// ModelTokenUsage represents token usage statistics for a specific model.
type ModelTokenUsage struct {
	ModelName    string
	CallCount    int64
	InputTokens  int64
	OutputTokens int64
}

// RecordTokenUsage records a model call and its token usage for a session.
func (m *SessionManager) RecordTokenUsage(threadID string, model string, inputTokens, outputTokens int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, err := m.db.Exec(`
		INSERT INTO token_usages (thread_id, model, calls, input_tokens, output_tokens)
		VALUES (?, ?, 1, ?, ?)
		ON CONFLICT(thread_id, model) DO UPDATE SET
			calls = calls + 1,
			input_tokens = input_tokens + excluded.input_tokens,
			output_tokens = output_tokens + excluded.output_tokens
	`, threadID, model, inputTokens, outputTokens)
	if err != nil {
		log.Printf("Failed to record token usage for thread %s: %v", threadID, err)
	}
}

// GetSessionTokenUsages returns the model token usage statistics for a specific session.
func (m *SessionManager) GetSessionTokenUsages(threadID string) ([]ModelTokenUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT model, calls, input_tokens, output_tokens 
		FROM token_usages 
		WHERE thread_id = ?
		ORDER BY calls DESC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []ModelTokenUsage
	for rows.Next() {
		var u ModelTokenUsage
		if err := rows.Scan(&u.ModelName, &u.CallCount, &u.InputTokens, &u.OutputTokens); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// GetGlobalTokenUsages returns the summed model token usage statistics across all sessions.
func (m *SessionManager) GetGlobalTokenUsages() ([]ModelTokenUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rows, err := m.db.Query(`
		SELECT model, SUM(calls), SUM(input_tokens), SUM(output_tokens) 
		FROM token_usages 
		GROUP BY model
		ORDER BY SUM(calls) DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var usages []ModelTokenUsage
	for rows.Next() {
		var u ModelTokenUsage
		if err := rows.Scan(&u.ModelName, &u.CallCount, &u.InputTokens, &u.OutputTokens); err != nil {
			return nil, err
		}
		usages = append(usages, u)
	}
	return usages, nil
}

// load reads sessions from the SQLite database.
func (m *SessionManager) load() {
	m.mu.Lock()
	defer m.mu.Unlock()

	rows, err := m.db.Query("SELECT thread_id, id, conversation_id, model FROM sessions")
	if err != nil {
		log.Printf("Failed to read sessions from db: %v", err)
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var threadID, id, conversationID, model string
		if err := rows.Scan(&threadID, &id, &conversationID, &model); err != nil {
			log.Printf("Failed to scan session row: %v", err)
			continue
		}

		s := NewSession(threadID)
		s.ID = id
		s.ConversationID = conversationID
		s.Model = model
		m.sessions[threadID] = s
		count++
	}
	log.Printf("Loaded %d sessions from disk", count)
}

// Save writes all current sessions to the SQLite database.
func (m *SessionManager) Save() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tx, err := m.db.Begin()
	if err != nil {
		log.Printf("Failed to begin transaction for save: %v", err)
		return
	}
	defer tx.Rollback()

	// Optionally, we could only update changed sessions, but since we're replacing Save(),
	// we'll just upsert them all or clear and insert. UPSERT is better.
	stmt, err := tx.Prepare(`
		INSERT INTO sessions (thread_id, id, conversation_id, model) 
		VALUES (?, ?, ?, ?)
		ON CONFLICT(thread_id) DO UPDATE SET 
			id=excluded.id,
			conversation_id=excluded.conversation_id,
			model=excluded.model
	`)
	if err != nil {
		log.Printf("Failed to prepare statement: %v", err)
		return
	}
	defer stmt.Close()

	for threadID, s := range m.sessions {
		s.mu.Lock()
		id, convID, model := s.ID, s.ConversationID, s.Model
		s.mu.Unlock()

		_, err := stmt.Exec(threadID, id, convID, model)
		if err != nil {
			log.Printf("Failed to save session for thread %s: %v", threadID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed to commit session save transaction: %v", err)
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

	_, err := m.db.Exec("DELETE FROM sessions WHERE thread_id = ?", threadID)
	if err != nil {
		log.Printf("Failed to delete session %s from db: %v", threadID, err)
	}
}

