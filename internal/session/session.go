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

	pendingMu sync.Mutex
	pending   []QueuedMessage
	notify    chan struct{}

	// CreatedAt is the time the session was created.
	CreatedAt time.Time
	// LastActiveAt is the time the session was last active.
	LastActiveAt time.Time

	mu               sync.Mutex
	processorStarted bool
}

// QueuedMessage represents a message waiting to be processed.
type QueuedMessage struct {
	ID      string
	Content string
}

// NewSession creates a new Session for the given Discord thread ID.
func NewSession(threadID string) *Session {
	now := time.Now()
	return &Session{
		ID:           generateUUID(),
		ThreadID:     threadID,
		pending:      make([]QueuedMessage, 0),
		notify:       make(chan struct{}, 1),
		CreatedAt:    now,
		LastActiveAt: now,
	}
}

// NotifyChan returns the notification channel.
func (s *Session) NotifyChan() <-chan struct{} {
	return s.notify
}

// Enqueue adds a message to the queue and triggers a notification.
func (s *Session) Enqueue(msgID, content string) {
	s.pendingMu.Lock()
	s.pending = append(s.pending, QueuedMessage{ID: msgID, Content: content})
	s.pendingMu.Unlock()

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// RemoveMessage removes a message from the queue by ID and triggers a notification.
func (s *Session) RemoveMessage(msgID string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	for i, msg := range s.pending {
		if msg.ID == msgID {
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			break
		}
	}

	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// GetAndClearPending returns all pending messages and clears the queue.
func (s *Session) GetAndClearPending() []QueuedMessage {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	res := s.pending
	s.pending = make([]QueuedMessage, 0)
	return res
}

// GetPendingCount returns the number of pending messages.
func (s *Session) GetPendingCount() int {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	return len(s.pending)
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

// GetModel returns the session's specific model, or empty if using global.
func (s *Session) GetModel() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Model
}

// EnsureProcessorStarted starts the session processor if it hasn't been started yet.
func (s *Session) EnsureProcessorStarted(startFunc func(sess *Session)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.processorStarted {
		s.processorStarted = true
		go startFunc(s)
	}
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
