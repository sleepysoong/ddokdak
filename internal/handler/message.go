// Package handler는 디스코드 메시지 및 쓰레드 이벤트를 처리합니다.
package handler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/agy"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
)

const (
	// maxThreadNameLength는 디스코드 쓰레드 이름의 최대 길이입니다.
	maxThreadNameLength = 100

	// typingInterval은 타이핑 인디케이터 갱신 간격입니다.
	typingInterval = 5 * time.Second

	// maxMessageLength는 디스코드 메시지 최대 길이입니다.
	maxMessageLength = 2000
)

// MessageHandler는 메시지 이벤트를 처리하는 핸들러입니다.
type MessageHandler struct {
	channelStore   store.ChannelStore
	sessionManager *session.SessionManager
	agyClient      *agy.Client
}

// NewMessageHandler는 새로운 메시지 핸들러를 생성합니다.
func NewMessageHandler(
	channelStore store.ChannelStore,
	sessionManager *session.SessionManager,
	agyClient *agy.Client,
) *MessageHandler {
	return &MessageHandler{
		channelStore:   channelStore,
		sessionManager: sessionManager,
		agyClient:      agyClient,
	}
}

// HandleMessage는 메시지 생성 이벤트를 처리합니다.
func (h *MessageHandler) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 봇 자신의 메시지 무시
	if m.Author.ID == s.State.User.ID {
		return
	}

	// 봇 메시지 무시
	if m.Author.Bot {
		return
	}

	// 쓰레드 내 메시지 처리
	if m.GuildID != "" && isThreadChannel(m.ChannelID, s) {
		h.handleThreadMessage(s, m)
		return
	}

	// 지정된 채널인지 확인
	if m.GuildID == "" || !h.channelStore.IsRegistered(m.GuildID, m.ChannelID) {
		return
	}

	// 새 쓰레드 생성 및 AI 응답
	h.handleNewConversation(s, m)
}

// handleNewConversation은 새로운 대화를 시작합니다.
func (h *MessageHandler) handleNewConversation(s *discordgo.Session, m *discordgo.MessageCreate) {
	// 쓰레드 이름 생성 (메시지 내용의 처음 부분 사용)
	threadName := truncateString(m.Content, maxThreadNameLength)
	if threadName == "" {
		threadName = "AI 대화"
	}

	// 쓰레드 생성 (만료되지 않도록 AutoArchiveDuration 최대값 설정)
	thread, err := s.MessageThreadStartComplex(m.ChannelID, m.ID, &discordgo.ThreadStart{
		Name:                threadName,
		AutoArchiveDuration: 10080, // 7일 (최대값)
		Type:                discordgo.ChannelTypeGuildPublicThread,
	})
	if err != nil {
		log.Printf("쓰레드 생성 실패: %v", err)
		return
	}

	// 세션 생성
	sess := h.sessionManager.CreateSession(thread.ID)
	log.Printf("새 세션 생성: ThreadID=%s, SessionID=%s", thread.ID, sess.ID)

	// AI 응답 처리 (비동기)
	go h.processAIResponse(s, thread.ID, m.Content, sess)
}

// handleThreadMessage는 쓰레드 내 메시지를 처리합니다.
func (h *MessageHandler) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	threadID := m.ChannelID

	// 세션 확인
	sess, exists := h.sessionManager.GetSession(threadID)
	if !exists {
		// 세션이 없는 쓰레드는 무시
		return
	}

	// 마지막 활동 시간 갱신
	sess.UpdateLastActive()

	// AI 응답 처리 (비동기)
	go h.processAIResponse(s, threadID, m.Content, sess)
}

// processAIResponse는 AI 응답을 생성하고 쓰레드에 전송합니다.
func (h *MessageHandler) processAIResponse(s *discordgo.Session, threadID string, prompt string, sess *session.Session) {
	// 타이핑 인디케이터 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.showTyping(ctx, s, threadID)

	// Antigravity CLI 호출
	conversationID := sess.GetConversationID()
	response, newConversationID, err := h.agyClient.Execute(ctx, prompt, conversationID, threadID)
	if err != nil {
		log.Printf("AI 응답 생성 실패: %v", err)
		h.sendErrorMessage(s, threadID, err)
		return
	}

	// 대화 ID 업데이트
	if newConversationID != "" && conversationID == "" {
		sess.SetConversationID(newConversationID)
		log.Printf("대화 ID 설정: ThreadID=%s, ConversationID=%s", threadID, newConversationID)
	}

	// 응답 전송
	h.sendResponse(s, threadID, response)
}

// showTyping은 타이핑 인디케이터를 표시합니다.
func (h *MessageHandler) showTyping(ctx context.Context, s *discordgo.Session, channelID string) {
	ticker := time.NewTicker(typingInterval)
	defer ticker.Stop()

	// 즉시 한 번 표시
	if err := s.ChannelTyping(channelID); err != nil {
		log.Printf("타이핑 인디케이터 표시 실패: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ChannelTyping(channelID); err != nil {
				log.Printf("타이핑 인디케이터 갱신 실패: %v", err)
				return
			}
		}
	}
}

// sendResponse는 응답을 채널에 전송합니다. 메시지가 길 경우 분할하여 전송합니다.
func (h *MessageHandler) sendResponse(s *discordgo.Session, channelID string, response string) {
	if response == "" {
		response = "⚠️ AI로부터 빈 응답을 받았습니다."
	}

	messages := splitMessage(response, maxMessageLength)
	for _, msg := range messages {
		if _, err := s.ChannelMessageSend(channelID, msg); err != nil {
			log.Printf("메시지 전송 실패: %v", err)
			return
		}
	}
}

// sendErrorMessage는 에러 메시지를 채널에 전송합니다.
func (h *MessageHandler) sendErrorMessage(s *discordgo.Session, channelID string, err error) {
	errorMsg := fmt.Sprintf("❌ AI 응답 생성 중 오류가 발생했습니다: %v", err)
	if _, sendErr := s.ChannelMessageSend(channelID, errorMsg); sendErr != nil {
		log.Printf("에러 메시지 전송 실패: %v", sendErr)
	}
}

// isThreadChannel은 채널이 쓰레드인지 확인합니다.
func isThreadChannel(channelID string, s *discordgo.Session) bool {
	channel, err := s.State.Channel(channelID)
	if err != nil {
		// 캐시에 없으면 API로 조회
		channel, err = s.Channel(channelID)
		if err != nil {
			return false
		}
	}
	return channel.IsThread()
}

// truncateString은 문자열을 지정된 최대 길이로 자릅니다.
// 유니코드 문자열을 올바르게 처리합니다.
func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen-3]) + "..."
}

// splitMessage는 긴 메시지를 최대 길이 단위로 분할합니다.
// 코드 블록을 고려하여 분할합니다.
func splitMessage(content string, maxLen int) []string {
	if len(content) <= maxLen {
		return []string{content}
	}

	var messages []string
	remaining := content

	for len(remaining) > 0 {
		if len(remaining) <= maxLen {
			messages = append(messages, remaining)
			break
		}

		// 최대 길이 이내에서 줄바꿈 위치 찾기
		cutAt := maxLen
		lastNewline := strings.LastIndex(remaining[:cutAt], "\n")
		if lastNewline > maxLen/2 {
			cutAt = lastNewline + 1
		}

		messages = append(messages, remaining[:cutAt])
		remaining = remaining[cutAt:]
	}

	return messages
}
