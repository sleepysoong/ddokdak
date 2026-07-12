package handler

import (
	"testing"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "짧은 문자열은 그대로 반환",
			input:    "짧은 텍스트",
			maxLen:   20,
			expected: "짧은 텍스트",
		},
		{
			name:     "최대 길이와 같은 문자열",
			input:    "정확히 5자입니다",
			maxLen:   8,
			expected: "정확히 5자입니다",
		},
		{
			name:     "긴 문자열 자르기",
			input:    "이것은 매우 긴 텍스트 메시지입니다",
			maxLen:   10,
			expected: "이것은 매우 ....",
		},
		{
			name:     "빈 문자열",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "영문 문자열",
			input:    "Hello, World! This is a test message.",
			maxLen:   15,
			expected: "Hello, World...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			runeCount := len([]rune(result))
			if runeCount > tt.maxLen {
				t.Errorf("truncateString(%q, %d) = %q (길이: %d), 최대 길이 초과",
					tt.input, tt.maxLen, result, runeCount)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		maxLen         int
		expectedParts  int
		checkFirstPart string
	}{
		{
			name:           "짧은 메시지는 분할하지 않음",
			content:        "짧은 메시지",
			maxLen:         100,
			expectedParts:  1,
			checkFirstPart: "짧은 메시지",
		},
		{
			name:          "긴 메시지 분할",
			content:       "가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하",
			maxLen:        30,
			expectedParts: 2,
		},
		{
			name:           "빈 메시지",
			content:        "",
			maxLen:         100,
			expectedParts:  1,
			checkFirstPart: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitMessage(tt.content, tt.maxLen)
			if len(parts) < 1 {
				t.Errorf("splitMessage() 반환된 부분이 없습니다")
				return
			}
			if tt.checkFirstPart != "" && parts[0] != tt.checkFirstPart {
				t.Errorf("splitMessage() 첫 번째 부분 = %q, 기대값 %q",
					parts[0], tt.checkFirstPart)
			}
			for i, part := range parts {
				if len(part) > tt.maxLen {
					t.Errorf("splitMessage() 부분 %d 길이(%d)가 최대 길이(%d)를 초과",
						i, len(part), tt.maxLen)
				}
			}
		})
	}
}

func TestGetThreadDetails(t *testing.T) {
	s, err := discordgo.New("Bot TOKEN")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Mock guild
	guild := &discordgo.Guild{
		ID: "guild-789",
	}
	s.State.GuildAdd(guild)

	// Mock channel
	threadChan := &discordgo.Channel{
		ID:       "thread-123",
		GuildID:  "guild-789",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		ParentID: "parent-456",
	}
	s.State.ChannelAdd(threadChan)

	isThread, parentID := getThreadDetails("thread-123", s)
	if !isThread {
		t.Error("expected isThread to be true")
	}
	if parentID != "parent-456" {
		t.Errorf("expected parentID 'parent-456', got %q", parentID)
	}

	// Non-thread channel
	normalChan := &discordgo.Channel{
		ID:      "channel-789",
		GuildID: "guild-789",
		Type:    discordgo.ChannelTypeGuildText,
	}
	s.State.ChannelAdd(normalChan)

	isThread, parentID = getThreadDetails("channel-789", s)
	if isThread {
		t.Error("expected isThread to be false")
	}
	if parentID != "" {
		t.Errorf("expected parentID to be empty, got %q", parentID)
	}
}

func TestHandleMessage_FilterUnregisteredThreads(t *testing.T) {
	s, err := discordgo.New("Bot TOKEN")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	s.State.User = &discordgo.User{ID: "bot-id"}

	guild := &discordgo.Guild{ID: "guild-123"}
	s.State.GuildAdd(guild)

	// Unregistered thread
	threadChan := &discordgo.Channel{
		ID:       "thread-777",
		GuildID:  "guild-123",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		ParentID: "parent-999",
	}
	s.State.ChannelAdd(threadChan)

	channelStore := store.NewInMemoryChannelStore()
	sessionManager := session.NewSessionManager(t.TempDir())
	sess := sessionManager.CreateSession("thread-777") // Pre-create dirty session

	h := &MessageHandler{
		channelStore:   channelStore,
		sessionManager: sessionManager,
	}

	m := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg-001",
			ChannelID: "thread-777",
			GuildID:   "guild-123",
			Content:   "Hello AI",
			Author: &discordgo.User{
				ID:  "user-555",
				Bot: false,
			},
		},
	}

	// Should return early and NOT queue any message in the session
	h.HandleMessage(s, m)

	if sess.GetPendingCount() != 0 {
		t.Errorf("expected 0 pending messages in unregistered thread session, got %d", sess.GetPendingCount())
	}
}

func TestHandleMessage_AllowRegisteredThreads(t *testing.T) {
	s, err := discordgo.New("Bot TOKEN")
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	s.State.User = &discordgo.User{ID: "bot-id"}

	guild := &discordgo.Guild{ID: "guild-123"}
	s.State.GuildAdd(guild)

	// Registered thread
	threadChan := &discordgo.Channel{
		ID:       "thread-888",
		GuildID:  "guild-123",
		Type:     discordgo.ChannelTypeGuildPublicThread,
		ParentID: "parent-111",
	}
	s.State.ChannelAdd(threadChan)

	channelStore := store.NewInMemoryChannelStore()
	channelStore.AddChannel("guild-123", "parent-111") // Register parent channel

	sessionManager := session.NewSessionManager(t.TempDir())

	h := &MessageHandler{
		channelStore:   channelStore,
		sessionManager: sessionManager,
	}

	m := &discordgo.MessageCreate{
		Message: &discordgo.Message{
			ID:        "msg-002",
			ChannelID: "thread-888",
			GuildID:   "guild-123",
			Content:   "Hello AI",
			Author: &discordgo.User{
				ID:  "user-555",
				Bot: false,
			},
		},
	}

	h.HandleMessage(s, m)

	// Verify that the session was created and contains the message
	sess, exists := sessionManager.GetSession("thread-888")
	if !exists {
		t.Fatal("expected session to be created for registered thread")
	}
	if sess.GetPendingCount() != 1 {
		t.Errorf("expected 1 pending message in session queue, got %d", sess.GetPendingCount())
	}
}


