// Package session provides session management for Discord bot threads.
package session

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Session represents a single conversation session tied to a Discord thread.
type Session struct {
	// ID is a unique identifier for the session (UUID v4).
	ID string
	// ThreadID is the Discord thread ID associated with this session.
	ThreadID string
	// ConversationID is the Antigravity CLI conversation ID.
	// It starts as an empty string and is set later.
	ConversationID string
	// Model is the specific model to use for this session.
	// If empty, the global model will be used.
	Model string
	// MsgChan is used to queue messages for debouncing.
	MsgChan chan string
	// CreatedAt is the time the session was created.
	CreatedAt time.Time
	// LastActiveAt is the time the session was last active.
	LastActiveAt time.Time

	mu sync.Mutex
}

// NewSession creates a new Session for the given Discord thread ID.
func NewSession(threadID string) *Session {
	now := time.Now()
	return &Session{
		ID:             generateUUID(),
		ThreadID:       threadID,
		ConversationID: "",
		Model:          "",
		MsgChan:        make(chan string, 100),
		CreatedAt:      now,
		LastActiveAt:   now,
	}
}

// UpdateLastActive updates the session's last active timestamp to now.
func (s *Session) UpdateLastActive() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LastActiveAt = time.Now()
}

// SetConversationID sets the Antigravity CLI conversation ID for this session.
func (s *Session) SetConversationID(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ConversationID = id
}

// GetConversationID returns the current Antigravity CLI conversation ID.
func (s *Session) GetConversationID() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ConversationID
}

// SetModel sets the AI model for this session.
func (s *Session) SetModel(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Model = model
}

// GetModel returns the AI model for this session.
func (s *Session) GetModel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Model
}

// generateUUID generates a UUID v4 string using crypto/rand.
func generateUUID() string {
	var uuid [16]byte
	if _, err := rand.Read(uuid[:]); err != nil {
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}

	// Set version 4 (bits 12-15 of time_hi_and_version).
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	// Set variant bits (bits 6-7 of clock_seq_hi_and_reserved).
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4],
		uuid[4:6],
		uuid[6:8],
		uuid[8:10],
		uuid[10:16],
	)
}
