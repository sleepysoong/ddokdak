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
	"github.com/sleepysoong/ddokdak/internal/config"
	"github.com/sleepysoong/ddokdak/internal/downloader"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
	"github.com/sleepysoong/ddokdak/internal/usage"
)

const (
	// maxThreadNameLength는 디스코드 쓰레드 이름의 최대 길이입니다.
	maxThreadNameLength = 100

	// typingInterval은 타이핑 인디케이터 갱신 간격입니다.
	typingInterval = 5 * time.Second

	// debounceInterval은 여러 메시지를 하나로 묶어 처리하기 위한 대기 시간입니다.
	debounceInterval = 2 * time.Second

	// maxMessageLength는 디스코드 메시지 최대 길이입니다.
	maxMessageLength = 2000
)

// MessageHandler는 메시지 이벤트를 처리하는 핸들러입니다.
type MessageHandler struct {
	channelStore   store.ChannelStore
	sessionManager *session.SessionManager
	agyClient      *agy.Client
	config         *config.Config
	downloader     *downloader.Downloader
	usageTracker   *usage.Tracker
}

// NewMessageHandler는 새로운 메시지 핸들러를 생성합니다.
func NewMessageHandler(
	channelStore store.ChannelStore,
	sessionManager *session.SessionManager,
	agyClient *agy.Client,
	cfg *config.Config,
	dl *downloader.Downloader,
	ut *usage.Tracker,
) *MessageHandler {
	return &MessageHandler{
		channelStore:   channelStore,
		sessionManager: sessionManager,
		agyClient:      agyClient,
		config:         cfg,
		downloader:     dl,
		usageTracker:   ut,
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

	// 세션 프로세서 시작
	h.startSessionProcessor(s, sess)

	// 메시지 큐에 추가
	h.enqueueMessage(s, m, sess)
}

// handleThreadMessage는 쓰레드 내 메시지를 처리합니다.
func (h *MessageHandler) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	threadID := m.ChannelID

	// 세션 확인
	sess, exists := h.sessionManager.GetSession(threadID)
	if !exists {
		// 봇 재시작 등으로 세션이 메모리에 없다면 새로 생성하여 처리 이어나감
		sess = h.sessionManager.CreateSession(threadID)
		h.startSessionProcessor(s, sess)
	}

	// 마지막 활동 시간 갱신
	sess.UpdateLastActive()

	// 메시지 큐에 추가
	h.enqueueMessage(s, m, sess)
}

// enqueueMessage는 메시지와 첨부파일을 처리하여 세션 큐에 넣습니다.
func (h *MessageHandler) enqueueMessage(s *discordgo.Session, m *discordgo.MessageCreate, sess *session.Session) {
	content := m.Content

	// 첨부파일 처리
	for _, att := range m.Attachments {
		path, err := h.downloader.Download(att.URL, att.Filename)
		if err == nil {
			content += fmt.Sprintf("\n[첨부파일 참고: %s]", path)
			log.Printf("첨부파일 다운로드 완료: %s", path)
		} else {
			log.Printf("첨부파일 다운로드 실패: %v", err)
			content += fmt.Sprintf("\n[첨부파일 다운로드 실패: %s]", att.Filename)
		}
	}

	if strings.TrimSpace(content) != "" {
		sess.MsgChan <- content
	}
}

// startSessionProcessor는 세션의 큐를 구독하여 디바운싱 처리 후 AI 응답을 트리거합니다.
// AI 응답이 완료될 때까지 새 메시지는 MsgChan 버퍼에 쌓이며 순차적으로 처리됩니다.
func (h *MessageHandler) startSessionProcessor(s *discordgo.Session, sess *session.Session) {
	go func() {
		var buffer []string
		timer := time.NewTimer(time.Hour)
		timer.Stop()

		for {
			select {
			case msg := <-sess.MsgChan:
				buffer = append(buffer, msg)
				timer.Reset(debounceInterval)
			case <-timer.C:
				if len(buffer) > 0 {
					prompt := strings.Join(buffer, "\n\n")
					buffer = nil
					// 동기 실행: AI 응답이 끝날 때까지 블로킹됩니다.
					// 응답 중에 들어온 메시지는 MsgChan 버퍼(cap 100)에 쌓이고,
					// 응답 완료 후 다시 루프가 돌면서 꺼내 처리합니다.
					h.processAIResponse(s, sess.ThreadID, prompt, sess)

					// AI 응답 완료 후 대기 중인 메시지를 즉시 수거
					h.drainPendingMessages(sess, &buffer, timer)
				}
			}
		}
	}()
}

// drainPendingMessages는 AI 응답 완료 직후 MsgChan에 쌓인 메시지를 논블로킹으로 수거합니다.
// 수거된 메시지가 있으면 디바운싱 타이머를 다시 시작합니다.
func (h *MessageHandler) drainPendingMessages(sess *session.Session, buffer *[]string, timer *time.Timer) {
	for {
		select {
		case msg := <-sess.MsgChan:
			*buffer = append(*buffer, msg)
		default:
			// 채널이 비었으면 수거 완료
			if len(*buffer) > 0 {
				timer.Reset(debounceInterval)
			}
			return
		}
	}
}

// processAIResponse는 AI 응답을 생성하고 쓰레드에 전송합니다.
func (h *MessageHandler) processAIResponse(s *discordgo.Session, threadID string, prompt string, sess *session.Session) {
	// 타이핑 인디케이터 시작
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.showTyping(ctx, s, threadID)

	// 세션 모델 확인 (없으면 글로벌 모델 사용)
	modelName := sess.GetModel()
	if modelName == "" {
		modelName = h.config.GetGlobalModel()
	}

	// Antigravity CLI 호출
	conversationID := sess.GetConversationID()
	h.usageTracker.RecordCall(modelName)
	response, newConversationID, err := h.agyClient.Execute(ctx, prompt, modelName, conversationID, threadID)
	if err != nil {
		h.usageTracker.RecordError(modelName)
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
