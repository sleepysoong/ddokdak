// Package handler는 디스코드 메시지 및 쓰레드 이벤트를 처리합니다.
package handler

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sort"
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
	if m.Author.ID == s.State.User.ID {
		return
	}

	if m.Author.Bot {
		return
	}

	if m.GuildID != "" {
		isThread, parentID := getThreadDetails(m.ChannelID, s)
		if isThread {
			isParentRegistered := parentID != "" && h.channelStore.IsRegistered(m.GuildID, parentID)

			if isParentRegistered {
				h.handleThreadMessage(s, m)
				return
			}
			return
		}
	}

	if m.GuildID == "" || !h.channelStore.IsRegistered(m.GuildID, m.ChannelID) {
		return
	}

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

	sess := h.sessionManager.CreateSession(thread.ID)
	log.Printf("새 세션 생성: ThreadID=%s, SessionID=%s", thread.ID, sess.ID)

	// 인터랙티브 컴포넌트를 가진 환영 메시지 구성
	welcomeMsg := &discordgo.MessageSend{
		Content: "🤖 **AI 대화 세션이 시작되었습니다!**\n이 쓰레드 내의 모든 대화는 하나의 세션으로 묶여 맥락이 유지됩니다.",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						CustomID:    "select_model",
						Placeholder: "🤖 이 세션의 AI 모델 변경...",
						Options: []discordgo.SelectMenuOption{
							{Label: "Claude Opus 4.6 (Thinking)", Value: "Claude Opus 4.6 (Thinking)"},
							{Label: "Gemini 3.1 Pro (High)", Value: "Gemini 3.1 Pro (High)"},
							{Label: "Gemini 3.5 Flash (High)", Value: "Gemini 3.5 Flash (High)"},
							{Label: "Gemini 3.5 Flash (Medium)", Value: "Gemini 3.5 Flash (Medium)"},
						},
					},
				},
			},
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "📊 현재 세션 사용량",
						Style:    discordgo.SecondaryButton,
						CustomID: "btn_usage",
					},
					discordgo.Button{
						Label:    "🔒 세션 종료 및 아카이브",
						Style:    discordgo.DangerButton,
						CustomID: "btn_end_session",
					},
				},
			},
		},
	}
	if _, err := s.ChannelMessageSendComplex(thread.ID, welcomeMsg); err != nil {
		log.Printf("환영 메시지 전송 실패: %v", err)
	}

	h.enqueueMessage(s, m, sess)
}

// handleThreadMessage는 쓰레드 내 메시지를 처리합니다.
func (h *MessageHandler) handleThreadMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	threadID := m.ChannelID

	sess, exists := h.sessionManager.GetSession(threadID)
	if !exists {
		// 봇 재시작 등으로 세션이 메모리에 없다면 새로 생성하여 처리 이어나감
		sess = h.sessionManager.CreateSession(threadID)
	}

	sess.UpdateLastActive()

	h.enqueueMessage(s, m, sess)
}

// enqueueMessage는 메시지와 첨부파일을 처리하여 세션 큐에 넣습니다.
func (h *MessageHandler) enqueueMessage(s *discordgo.Session, m *discordgo.MessageCreate, sess *session.Session) {
	content := m.Content

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
		sess.EnsureProcessorStarted(func(sessionObj *session.Session) {
			h.startSessionProcessor(s, sessionObj)
		})

		sess.Enqueue(m.ID, content)
	}
}

// startSessionProcessor는 세션의 큐를 구독하여 디바운싱 처리 후 AI 응답을 트리거합니다.
func (h *MessageHandler) startSessionProcessor(s *discordgo.Session, sess *session.Session) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🔥 [Panic Recovered] startSessionProcessor: %v", r)
			}
		}()

		timer := time.NewTimer(time.Hour)
		timer.Stop()

		for {
			select {
			case <-sess.NotifyChan():
				count := sess.GetPendingCount()
				if count > 0 {
					timer.Reset(debounceInterval)
				} else {
					timer.Stop()
				}
			case <-timer.C:
				msgs := sess.GetAndClearPending()
				if len(msgs) > 0 {
					var contents []string
					for _, m := range msgs {
						contents = append(contents, m.Content)
					}
					prompt := strings.Join(contents, "\n\n")

					h.processAIResponse(s, sess.ThreadID, prompt, sess)

					// 응답 후 대기 중인 새 메시지가 있으면 타이머 재가동
					if sess.GetPendingCount() > 0 {
						timer.Reset(debounceInterval)
					}
				}
			}
		}
	}()
}

// processAIResponse는 AI 응답을 생성하고 쓰레드에 전송합니다.
func (h *MessageHandler) processAIResponse(s *discordgo.Session, threadID string, prompt string, sess *session.Session) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go h.showTyping(ctx, s, threadID)

	// 세션 모델 확인 (없으면 글로벌 모델 사용)
	modelName := sess.GetModel()
	if modelName == "" {
		modelName = h.config.GetGlobalModel()
	}

	startTime := time.Now()

	// 1. 디스코드 생각 중(Thinking) 메시지 전송 (사용자 요청 커스텀 양식)
	initialContent := fmt.Sprintf("● **`%s`**로 응답을 생성하는 중입니다. (`0s`)", modelName)
	thinkingMsg, err := s.ChannelMessageSend(threadID, initialContent)
	var thinkingMsgID string
	if err == nil {
		thinkingMsgID = thinkingMsg.ID
	} else {
		log.Printf("생각 중 메시지 전송 실패: %v", err)
	}

	// 2. 백그라운드 실시간 진행 상황 업데이트 고루틴 가동 (1초 마다 업데이트)
	updateCtx, updateCancel := context.WithCancel(context.Background())
	defer updateCancel()

	if thinkingMsgID != "" {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			var lastToolsUsed []string
			var lastContent string
			for {
				select {
				case <-updateCtx.Done():
					return
				case <-ticker.C:
					elapsed := time.Since(startTime)

					convID := sess.GetConversationID()
					if convID == "" {
						logFile := filepath.Join(h.agyClient.GetLogDir(), threadID+".log")
						convID = agy.ExtractConversationID(logFile)
						if convID != "" {
							sess.SetConversationID(convID)
							h.sessionManager.Save()
						}
					}

					if convID == "" {
						// 아직 대화 ID가 확인되지 않은 경우에도 대기 시간은 지속 업데이트
						content := fmt.Sprintf("● **`%s`**로 응답을 생성하는 중입니다. (`%s`)", modelName, formatDuration(elapsed))
						if content != lastContent {
							_, _ = s.ChannelMessageEdit(threadID, thinkingMsgID, content)
							lastContent = content
						}
						continue
					}

					executions, err := agy.ParseToolExecutions(convID)
					if err == nil {
						var currentTools []string
						for _, exec := range executions {
							currentTools = append(currentTools, formatToolCallInline(exec))
						}
						lastToolsUsed = currentTools
					}

					var pct int
					telemetry, err := agy.ParseTelemetry(convID)
					if err == nil {
						pct = telemetry.Pct
					}

					var sb strings.Builder
					sb.WriteString(fmt.Sprintf("● **`%s`**로 응답을 생성하는 중입니다. (`%s`)\n", modelName, formatDuration(elapsed)))
					if len(lastToolsUsed) > 0 {
						sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
						for _, t := range lastToolsUsed {
							sb.WriteString(fmt.Sprintf("○ %s\n", t))
						}
					}
					if pct > 0 {
						sb.WriteString(fmt.Sprintf("\n*(현재 컨텍스트 사용량: %d%%)*", pct))
					}

					content := sb.String()
					if content != lastContent {
						_, _ = s.ChannelMessageEdit(threadID, thinkingMsgID, content)
						lastContent = content
					}
				}
			}
		}()
	}

	conversationID := sess.GetConversationID()
	response, newConversationID, actualModel, err := h.agyClient.Execute(ctx, prompt, modelName, conversationID, threadID)
	elapsed := time.Since(startTime)

	// 실시간 업데이트 루프 종료
	updateCancel()

	h.usageTracker.RecordCall(actualModel)
	if err != nil {
		h.usageTracker.RecordError(actualModel)
		log.Printf("AI 응답 생성 실패: %v", err)
		errorMsg := fmt.Sprintf("❌ AI 응답 생성 중 오류가 발생했습니다: %v", err)
		if thinkingMsgID != "" {
			_, _ = s.ChannelMessageEdit(threadID, thinkingMsgID, errorMsg)
		} else {
			_, _ = s.ChannelMessageSend(threadID, errorMsg)
		}
		return
	}

	if newConversationID != "" && conversationID == "" {
		sess.SetConversationID(newConversationID)
		log.Printf("대화 ID 설정: ThreadID=%s, ConversationID=%s", threadID, newConversationID)
		h.sessionManager.Save()
	}

	if actualModel != "" && actualModel != sess.GetModel() {
		sess.SetModel(actualModel)
		h.sessionManager.Save()
	}

	// 실제 사용한 모델 확정
	usedModel := modelName
	if actualModel != "" {
		usedModel = actualModel
	}

	// 최종 대화 ID 확정 (동영상/웹검색 및 실시간 텔레메트리 파싱용)
	finalConvID := sess.GetConversationID()
	if finalConvID == "" {
		finalConvID = newConversationID
	}

	// 도구 호출 및 결과 파싱
	var toolsUsed []string
	if finalConvID != "" {
		executions, err := agy.ParseToolExecutions(finalConvID)
		if err == nil {
			for _, exec := range executions {
				toolsUsed = append(toolsUsed, formatToolCallInline(exec))
			}
		} else {
			log.Printf("도구 실행 내역 파싱 실패: %v", err)
		}
	}

	// 실시간 텔레메트리(컨텍스트 비율 및 사용 토큰 수) 파싱
	var pct int
	if finalConvID != "" {
		telemetry, err := agy.ParseTelemetry(finalConvID)
		if err == nil {
			pct = telemetry.Pct
			// sqlite에 세션별 모델 토큰 사용량 기록
			h.sessionManager.RecordTokenUsage(threadID, usedModel, telemetry.InputTokens, telemetry.OutputTokens)
		} else {
			log.Printf("텔레메트리 파싱 실패: %v", err)
		}
	}

	// 답변 생성 정보 통합 요약 메시지 구성 (사용자 요청 템플릿과 100% 일치)
	var finalMessage strings.Builder
	if len(toolsUsed) > 0 {
		for _, t := range toolsUsed {
			finalMessage.WriteString(fmt.Sprintf("○ %s\n", t))
		}
		finalMessage.WriteString("\n")
	}

	finalMessage.WriteString(response)
	finalMessage.WriteString("\n\n")

	// 모델명 뒤에 (pct%) | 시간 추가
	durationStr := formatDuration(elapsed)
	if pct > 0 {
		finalMessage.WriteString(fmt.Sprintf("● **`%s`** `(%d%%)` | `%s`", usedModel, pct, durationStr))
	} else {
		finalMessage.WriteString(fmt.Sprintf("● **`%s`** | `%s`", usedModel, durationStr))
	}

	h.sendResponseWithEdit(s, threadID, finalMessage.String(), thinkingMsgID)
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}



func stripQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func formatToolCallInline(exec agy.ToolExecution) string {
	// 우선순위가 높은 공통 인자 Key 목록
	priorityKeys := []string{
		"CommandLine", "command", "cmd",
		"AbsolutePath", "TargetFile", "Target", "path", "file", "filename",
		"Query", "query", "q",
		"Url", "url", "uri",
		"name", "Recipient",
	}

	var argVal interface{}
	// 1. 우선순위 목록에서 먼저 매칭되는 Key를 찾음
	for _, key := range priorityKeys {
		if val, exists := exec.Args[key]; exists {
			argVal = val
			break
		}
	}

	// 2. 만약 매칭되는 우선순위 Key가 없으면, 모든 인자를 key=value 형태로 결합 (시스템 인자 제외)
	if argVal == nil && len(exec.Args) > 0 {
		var parts []string
		for k, v := range exec.Args {
			if k == "toolAction" || k == "toolSummary" || k == "IsSkillFile" || k == "IsMock" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		if len(parts) > 0 {
			sort.Strings(parts)
			argVal = strings.Join(parts, ", ")
		}
	}

	if argVal != nil {
		valStr := stripQuotes(fmt.Sprintf("%v", argVal))
		if len(valStr) > 60 {
			valStr = valStr[:57] + "..."
		}
		return fmt.Sprintf("`%s(%s)`", exec.ToolName, valStr)
	}
	return fmt.Sprintf("`%s`", exec.ToolName)
}

// showTyping은 타이핑 인디케이터를 표시합니다.
func (h *MessageHandler) showTyping(ctx context.Context, s *discordgo.Session, channelID string) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🔥 [Panic Recovered] showTyping: %v", r)
		}
	}()

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

// sendResponseWithEdit는 응답을 채널에 전송하며, 첫 번째 메시지는 기존 메시지를 수정하여 덮어씁니다.
func (h *MessageHandler) sendResponseWithEdit(s *discordgo.Session, channelID string, response string, editMsgID string) {
	if response == "" {
		response = "⚠️ AI로부터 빈 응답을 받았습니다."
	}

	messages := splitMessage(response, maxMessageLength)
	if len(messages) > 0 {
		// 첫 번째 메시지는 생각 중 메시지를 덮어씁니다.
		if editMsgID != "" {
			_, err := s.ChannelMessageEdit(channelID, editMsgID, messages[0])
			if err != nil {
				log.Printf("메시지 수정 실패, 새 메시지 전송 시도: %v", err)
				_, _ = s.ChannelMessageSend(channelID, messages[0])
			}
		} else {
			_, _ = s.ChannelMessageSend(channelID, messages[0])
		}

		// 분할된 후속 메시지들을 전송합니다.
		for i := 1; i < len(messages); i++ {
			if _, err := s.ChannelMessageSend(channelID, messages[i]); err != nil {
				log.Printf("메시지 전송 실패: %v", err)
				return
			}
		}
	}
}

// getThreadDetails는 채널이 쓰레드인지 확인하고, 부모 채널 ID를 반환합니다.
func getThreadDetails(channelID string, s *discordgo.Session) (bool, string) {
	channel, err := s.State.Channel(channelID)
	if err != nil {
		// 캐시에 없으면 API로 조회
		channel, err = s.Channel(channelID)
		if err != nil {
			return false, ""
		}
	}
	return channel.IsThread(), channel.ParentID
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

// HandleMessageDelete는 메시지가 삭제되었을 때 처리하는 핸들러입니다.
func (h *MessageHandler) HandleMessageDelete(s *discordgo.Session, m *discordgo.MessageDelete) {
	if m.Message != nil && m.Message.Author != nil && m.Message.Author.ID == s.State.User.ID {
		return
	}

	sess, exists := h.sessionManager.GetSession(m.ChannelID)
	if exists {
		sess.RemoveMessage(m.ID)
	}
}
