package command

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/agy"
	"github.com/sleepysoong/ddokdak/internal/config"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
	"github.com/sleepysoong/ddokdak/internal/usage"
)

// Handler는 슬래시 커맨드를 처리하는 핸들러입니다.
type Handler struct {
	channelStore   store.ChannelStore
	config         *config.Config
	sessionManager *session.SessionManager
	dashboard      *usage.Dashboard
}

// NewHandler는 새로운 커맨드 핸들러를 생성합니다.
func NewHandler(channelStore store.ChannelStore, cfg *config.Config, sm *session.SessionManager, dashboard *usage.Dashboard) *Handler {
	return &Handler{
		channelStore:   channelStore,
		config:         cfg,
		sessionManager: sm,
		dashboard:      dashboard,
	}
}

// HandleInteraction은 커맨드 및 컴포넌트 인터랙션을 처리합니다.
func (h *Handler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		data := i.ApplicationCommandData()
		switch data.Name {
		case "채널지정":
			h.handleSetChannel(s, i)
		case "채널해제":
			h.handleUnsetChannel(s, i)
		case "모델변경":
			h.handleModelChange(s, i)
		case "사용량":
			h.handleUsage(s, i)
		case "로그":
			h.handleLogCommand(s, i)
		case "new":
			h.handleNewSessionCommand(s, i)
		case "세션종료":
			h.handleEndSession(s, i)
		}
	case discordgo.InteractionMessageComponent:
		h.handleComponent(s, i)
	}
}

// handleSetChannel은 /채널지정 커맨드를 처리합니다.
func (h *Handler) handleSetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "채널을 선택해주세요.")
		return
	}

	channel := options[0].ChannelValue(s)
	if channel == nil {
		h.respondError(s, i, "유효하지 않은 채널입니다.")
		return
	}

	guildID := i.GuildID
	if guildID == "" {
		h.respondError(s, i, "이 커맨드는 서버에서만 사용할 수 있습니다.")
		return
	}

	if h.channelStore.IsRegistered(guildID, channel.ID) {
		h.respond(s, i, fmt.Sprintf("✅ <#%s> 채널은 이미 AI 대화 채널로 지정되어 있습니다.", channel.ID))
		return
	}

	if err := h.channelStore.AddChannel(guildID, channel.ID); err != nil {
		log.Printf("채널 저장 실패: %v", err)
		h.respondError(s, i, "채널 지정에 실패했습니다. 다시 시도해주세요.")
		return
	}

	h.respond(s, i, fmt.Sprintf("✅ <#%s> 채널이 AI 대화 채널로 지정되었습니다.\n이 채널에서 메시지를 보내면 자동으로 쓰레드가 생성되고 AI가 응답합니다.", channel.ID))
}

// handleUnsetChannel은 /채널해제 커맨드를 처리합니다.
func (h *Handler) handleUnsetChannel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		h.respondError(s, i, "채널을 선택해주세요.")
		return
	}

	channel := options[0].ChannelValue(s)
	if channel == nil {
		h.respondError(s, i, "유효하지 않은 채널입니다.")
		return
	}

	guildID := i.GuildID
	if guildID == "" {
		h.respondError(s, i, "이 커맨드는 서버에서만 사용할 수 있습니다.")
		return
	}

	if !h.channelStore.IsRegistered(guildID, channel.ID) {
		h.respond(s, i, fmt.Sprintf("⚠️ <#%s> 채널은 AI 대화 채널로 지정되어 있지 않습니다.", channel.ID))
		return
	}

	if err := h.channelStore.RemoveChannel(guildID, channel.ID); err != nil {
		log.Printf("채널 해제 실패: %v", err)
		h.respondError(s, i, "채널 해제에 실패했습니다. 다시 시도해주세요.")
		return
	}

	h.respond(s, i, fmt.Sprintf("✅ <#%s> 채널의 AI 대화 기능이 해제되었습니다.", channel.ID))
}

// handleModelChange는 /모델변경 커맨드를 처리합니다. (드롭다운 인터랙티브 컴포넌트 제공)
func (h *Handler) handleModelChange(s *discordgo.Session, i *discordgo.InteractionCreate) {
	_, hasSession := h.sessionManager.GetSession(i.ChannelID)

	modelOptions := []discordgo.SelectMenuOption{
		{Label: "Claude Opus 4.6 (Thinking)", Value: "Claude Opus 4.6 (Thinking)"},
		{Label: "Gemini 3.1 Pro (High)", Value: "Gemini 3.1 Pro (High)"},
		{Label: "Gemini 3.5 Flash (High)", Value: "Gemini 3.5 Flash (High)"},
		{Label: "Gemini 3.5 Flash (Medium)", Value: "Gemini 3.5 Flash (Medium)"},
	}

	sessionPlaceholder := "🤖 이 세션의 AI 모델 변경..."
	if !hasSession {
		sessionPlaceholder = "❌ 이 채널은 세션 쓰레드가 아닙니다."
	}

	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "select_global_model",
					Placeholder: "🌐 글로벌 기본 AI 모델 변경...",
					Options:     modelOptions,
				},
			},
		},
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "select_session_model",
					Placeholder: sessionPlaceholder,
					Options:     modelOptions,
					Disabled:    !hasSession,
				},
			},
		},
	}

	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "⚙️ **변경할 AI 모델과 대상을 선택하세요.**\n- **글로벌 기본 모델**: 새 쓰레드 생성 시 기본으로 사용되는 모델입니다.\n- **세션 모델**: 현재 쓰레드 내에서만 커스텀으로 사용되는 모델입니다.",
			Components: components,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		log.Printf("모델변경 인터랙션 응답 실패: %v", err)
	}
}

// handleUsage는 /사용량 커맨드를 처리합니다.
func (h *Handler) handleUsage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// 현재 채널(쓰레드)에 활성 세션이 있는지 확인
	_, exists := h.sessionManager.GetSession(i.ChannelID)
	if exists {
		usages, err := h.sessionManager.GetSessionTokenUsages(i.ChannelID)
		if err != nil {
			h.respondError(s, i, fmt.Sprintf("세션 사용량 조회 실패: %v", err))
			return
		}

		embed := h.dashboard.FormatSessionDashboardEmbed(usages, i.ChannelID)
		h.respondEmbed(s, i, embed)
		return
	}

	// 쓰레드 밖인 경우 - 기존처럼 1분 주기 전체 대시보드 출력
	h.respond(s, i, "📊 전체 사용량 대시보드를 생성합니다...")

	if err := h.dashboard.StartDashboard(s, i.ChannelID); err != nil {
		log.Printf("사용량 대시보드 시작 실패: %v", err)
	}
}

// handleLogCommand는 /로그 커맨드를 처리합니다.
func (h *Handler) handleLogCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channel, err := s.State.Channel(i.ChannelID)
	if err != nil {
		channel, err = s.Channel(i.ChannelID)
	}

	if err != nil || !channel.IsThread() {
		h.respondError(s, i, "이 명령어는 AI 대화 쓰레드 내에서만 사용할 수 있습니다.")
		return
	}

	logFilePath := filepath.Join(".", "logs", i.ChannelID+".log")
	file, err := os.Open(logFilePath)
	if err != nil {
		h.respondError(s, i, "현재 쓰레드의 로그 파일을 찾을 수 없거나 열 수 없습니다.")
		return
	}
	defer file.Close()

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "📄 현재 대화 세션의 로그 파일입니다.",
			Files: []*discordgo.File{
				{
					Name:        i.ChannelID + ".log",
					ContentType: "text/plain",
					Reader:      file,
				},
			},
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("/로그 커맨드 응답 실패: %v", err)
	}
}

// handleNewSessionCommand는 /new 커맨드를 처리합니다.
func (h *Handler) handleNewSessionCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	sessions, err := agy.GetRecentSessions(25)
	if err != nil {
		log.Printf("최근 세션 조회 실패: %v", err)
		h.respondError(s, i, "세션 목록을 불러오는 중 오류가 발생했습니다.")
		return
	}

	if len(sessions) == 0 {
		h.respond(s, i, "불러올 기존 대화 세션이 없습니다.")
		return
	}

	var options []discordgo.SelectMenuOption
	for _, sess := range sessions {
		options = append(options, discordgo.SelectMenuOption{
			Label:       sess.Title,
			Value:       sess.ID,
			Description: sess.ID[:8] + "...", // short UUID
		})
	}

	menu := []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				discordgo.SelectMenu{
					CustomID:    "select_session",
					Placeholder: "이어서 대화할 세션을 선택하세요",
					Options:     options,
				},
			},
		},
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    "기존 세션 중에서 고르고 시작할 수 있습니다. 선택하면 새 쓰레드가 생성되거나 현재 쓰레드의 세션이 교체됩니다.",
			Components: menu,
			Flags:      discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("/new 커맨드 응답 실패: %v", err)
	}
}

// handleComponent는 셀렉트 메뉴 등의 컴포넌트 인터랙션을 처리합니다.
func (h *Handler) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if data.CustomID == "select_session" {
		if len(data.Values) == 0 {
			return
		}
		selectedConvID := data.Values[0]

		channel, err := s.State.Channel(i.ChannelID)
		if err != nil {
			channel, err = s.Channel(i.ChannelID)
		}

		if err == nil && channel.IsThread() {
			// 현재 채널이 쓰레드라면 세션을 덮어씌움
			sess, exists := h.sessionManager.GetSession(i.ChannelID)
			if !exists {
				sess = h.sessionManager.CreateSession(i.ChannelID)
			}
			sess.SetConversationID(selectedConvID)
			h.sessionManager.Save()
			h.respond(s, i, "✅ 현재 쓰레드의 대화 세션이 선택한 세션으로 교체되었습니다. 계속 대화하세요!")
		} else {
			// 일반 채널이라면 새 쓰레드 생성
			if !h.channelStore.IsRegistered(i.GuildID, i.ChannelID) {
				h.respondError(s, i, "이 채널은 AI 대화 채널로 지정되어 있지 않습니다.")
				return
			}

			// 세션 제목 찾기
			threadName := "불러온 대화 세션"
			if sessions, err := agy.GetRecentSessions(25); err == nil {
				for _, sessInfo := range sessions {
					if sessInfo.ID == selectedConvID {
						threadName = sessInfo.Title
						break
					}
				}
			}

			// 인터랙션 메시지에 쓰레드를 만들 수는 없으므로, 새 안내 메시지를 보내고 거기서 쓰레드를 만듦
			msg, err := s.ChannelMessageSend(i.ChannelID, "🔗 불러온 세션으로 대화를 시작합니다...")
			if err != nil {
				h.respondError(s, i, "쓰레드 생성용 메시지 전송 실패.")
				return
			}

			thread, err := s.MessageThreadStartComplex(i.ChannelID, msg.ID, &discordgo.ThreadStart{
				Name:                threadName,
				AutoArchiveDuration: 1440,
			})
			if err != nil {
				h.respondError(s, i, "쓰레드 생성 실패.")
				return
			}

			sess := h.sessionManager.CreateSession(thread.ID)
			sess.SetConversationID(selectedConvID)
			h.sessionManager.Save()

			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseUpdateMessage,
				Data: &discordgo.InteractionResponseData{
					Content:    fmt.Sprintf("✅ 세션이 불러와졌습니다! <#%s> 에서 대화를 이어나가세요.", thread.ID),
					Components: []discordgo.MessageComponent{}, // 셀렉트 메뉴 숨김
				},
			})
		}
	} else if data.CustomID == "select_model" {
		if len(data.Values) == 0 {
			return
		}
		modelName := data.Values[0]
		sess, exists := h.sessionManager.GetSession(i.ChannelID)
		if !exists {
			h.respondError(s, i, "이 쓰레드에 활성화된 세션이 없습니다.")
			return
		}
		sess.SetModel(modelName)
		h.sessionManager.Save()
		h.respond(s, i, fmt.Sprintf("🤖 이 세션의 AI 모델이 **%s**(으)로 변경되었습니다.", modelName))
	} else if data.CustomID == "select_session_model" {
		if len(data.Values) == 0 {
			return
		}
		modelName := data.Values[0]
		sess, exists := h.sessionManager.GetSession(i.ChannelID)
		if !exists {
			h.respondError(s, i, "이 쓰레드에 활성화된 세션이 없습니다.")
			return
		}
		sess.SetModel(modelName)
		h.sessionManager.Save()

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("✅ 현재 세션의 AI 모델이 **%s**(으)로 변경되었습니다.", modelName),
				Components: []discordgo.MessageComponent{}, // 컴포넌트 완전 제거
			},
		})
	} else if data.CustomID == "select_global_model" {
		if len(data.Values) == 0 {
			return
		}
		modelName := data.Values[0]
		h.config.SetGlobalModel(modelName)

		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    fmt.Sprintf("✅ 글로벌 기본 AI 모델이 **%s**(으)로 변경되었습니다.", modelName),
				Components: []discordgo.MessageComponent{}, // 컴포넌트 완전 제거
			},
		})
	} else if data.CustomID == "btn_usage" {
		usages, err := h.sessionManager.GetSessionTokenUsages(i.ChannelID)
		if err != nil {
			h.respondError(s, i, fmt.Sprintf("세션 사용량 조회 실패: %v", err))
			return
		}
		embed := h.dashboard.FormatSessionDashboardEmbed(usages, i.ChannelID)
		h.respondEmbed(s, i, embed)
	} else if data.CustomID == "btn_end_session" {
		h.handleEndSession(s, i)
	}
}

// handleEndSession은 /세션종료 커맨드 혹은 종료 버튼 클릭을 처리합니다.
func (h *Handler) handleEndSession(s *discordgo.Session, i *discordgo.InteractionCreate) {
	channel, err := s.State.Channel(i.ChannelID)
	if err != nil {
		channel, err = s.Channel(i.ChannelID)
	}

	if err != nil || !channel.IsThread() {
		h.respondError(s, i, "이 명령어는 쓰레드 채널 안에서만 실행할 수 있습니다.")
		return
	}

	// 1. 세션 제거
	h.sessionManager.RemoveSession(i.ChannelID)

	// 2. 디스코드 응답 및 컴포넌트 제거
	if i.Type == discordgo.InteractionMessageComponent {
		_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "🔒 AI 세션이 종료되었습니다. 이 쓰레드는 잠시 후 잠금 및 아카이브 처리됩니다.",
				Components: []discordgo.MessageComponent{}, // 컴포넌트 완전 제거
			},
		})
	} else {
		h.respond(s, i, "🔒 AI 세션이 종료되었습니다. 이 쓰레드는 잠시 후 잠금 및 아카이브 처리됩니다.")
	}

	// 3. 쓰레드 잠금 및 아카이브 (비동기로 진행하여 레이트리밋이나 인터랙션 지연 방지)
	go func() {
		time.Sleep(2 * time.Second)
		archived := true
		locked := true
		_, editErr := s.ChannelEdit(i.ChannelID, &discordgo.ChannelEdit{
			Archived: &archived,
			Locked:   &locked,
		})
		if editErr != nil {
			log.Printf("Failed to archive/lock thread %s: %v", i.ChannelID, editErr)
		}
	}()
}

// respond는 인터랙션에 응답을 보냅니다.
func (h *Handler) respond(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("인터랙션 응답 실패: %v", err)
	}
}

// respondEmbed는 인터랙션에 임베드 형식의 응답을 보냅니다.
func (h *Handler) respondEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		log.Printf("인터랙션 임베드 응답 실패: %v", err)
	}
}

// respondError는 에러 응답을 보냅니다.
func (h *Handler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	h.respond(s, i, fmt.Sprintf("❌ %s", message))
}
