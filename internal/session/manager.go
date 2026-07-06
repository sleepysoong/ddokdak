package session

import "sync"

// SessionManager manages all active sessions, keyed by Discord thread ID.
type SessionManager struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewSessionManager creates and returns a new SessionManager.
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session for the given thread ID and stores it.
// If a session already exists for the thread ID, it is replaced.
func (m *SessionManager) CreateSession(threadID string) *Session {
	s := NewSession(threadID)

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[threadID] = s

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
	defer m.mu.Unlock()
	delete(m.sessions, threadID)
}
