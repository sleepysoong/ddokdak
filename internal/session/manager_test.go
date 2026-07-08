package session

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCreateSession(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	threadID := "thread-123"

	s := m.CreateSession(threadID)

	if s == nil {
		t.Fatal("expected non-nil session")
	}
	if s.ThreadID != threadID {
		t.Errorf("expected ThreadID %q, got %q", threadID, s.ThreadID)
	}
	if s.ID == "" {
		t.Error("expected non-empty session ID")
	}
	if s.ConversationID != "" {
		t.Errorf("expected empty ConversationID, got %q", s.ConversationID)
	}
	if s.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
	if s.LastActiveAt.IsZero() {
		t.Error("expected LastActiveAt to be set")
	}
}

func TestGetSession(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	threadID := "thread-456"

	// Should not exist yet.
	_, ok := m.GetSession(threadID)
	if ok {
		t.Error("expected session to not exist before creation")
	}

	created := m.CreateSession(threadID)

	got, ok := m.GetSession(threadID)
	if !ok {
		t.Fatal("expected session to exist after creation")
	}
	if got.ID != created.ID {
		t.Errorf("expected session ID %q, got %q", created.ID, got.ID)
	}
}

func TestRemoveSession(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	threadID := "thread-789"

	m.CreateSession(threadID)

	m.RemoveSession(threadID)

	_, ok := m.GetSession(threadID)
	if ok {
		t.Error("expected session to not exist after removal")
	}

	// Removing a non-existent session should not panic.
	m.RemoveSession("non-existent")
}

func TestSetConversationID(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	threadID := "thread-conv"

	s := m.CreateSession(threadID)

	if got := s.GetConversationID(); got != "" {
		t.Errorf("expected empty ConversationID initially, got %q", got)
	}

	convID := "conv-abc-123"
	s.SetConversationID(convID)

	if got := s.GetConversationID(); got != convID {
		t.Errorf("expected ConversationID %q, got %q", convID, got)
	}
}

func TestUpdateLastActive(t *testing.T) {
	s := NewSession("thread-active")
	initialTime := s.LastActiveAt

	// Small sleep to ensure time difference.
	time.Sleep(10 * time.Millisecond)

	s.UpdateLastActive()

	if !s.LastActiveAt.After(initialTime) {
		t.Error("expected LastActiveAt to be updated to a later time")
	}
}

func TestUUIDFormat(t *testing.T) {
	s := NewSession("thread-uuid")

	// UUID v4 format: 8-4-4-4-12 hex chars.
	id := s.ID
	if len(id) != 36 {
		t.Errorf("expected UUID length 36, got %d: %q", len(id), id)
	}
	if id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		t.Errorf("expected UUID format xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx, got %q", id)
	}
	// Version nibble should be '4'.
	if id[14] != '4' {
		t.Errorf("expected UUID version 4 (char at index 14 = '4'), got %q in %q", string(id[14]), id)
	}
}

func TestUUIDUniqueness(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		s := NewSession(fmt.Sprintf("thread-%d", i))
		if _, exists := seen[s.ID]; exists {
			t.Fatalf("duplicate UUID detected: %s", s.ID)
		}
		seen[s.ID] = struct{}{}
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewSessionManager(t.TempDir())
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			threadID := fmt.Sprintf("thread-%d", i)
			m.CreateSession(threadID)
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			threadID := fmt.Sprintf("thread-%d", i)
			m.GetSession(threadID)
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			threadID := fmt.Sprintf("thread-%d", i)
			m.RemoveSession(threadID)
		}(i)
	}

	wg.Wait()
}

func TestConcurrentSessionMethods(t *testing.T) {
	s := NewSession("thread-concurrent")
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			s.SetConversationID(fmt.Sprintf("conv-%d", i))
		}(i)
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.GetConversationID()
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			s.UpdateLastActive()
		}()
	}

	wg.Wait()

	// After all goroutines finish, ConversationID should be one of the set values.
	got := s.GetConversationID()
	if got == "" {
		t.Error("expected ConversationID to be set after concurrent writes")
	}
}
