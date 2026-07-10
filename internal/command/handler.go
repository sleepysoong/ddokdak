package command

import (
	"fmt"
	"log"

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
		case "new":
			h.handleNewSessionCommand(s, i)
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

// handleModelChange는 /모델변경 커맨드를 처리합니다.
func (h *Handler) handleModelChange(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := i.ApplicationCommandData().Options
	if len(options) < 2 {
		h.respondError(s, i, "타입과 모델명을 모두 입력해주세요.")
		return
	}

	changeType := options[0].StringValue()
	modelName := options[1].StringValue()

	if changeType == "global" {
		h.config.SetGlobalModel(modelName)
		h.respond(s, i, fmt.Sprintf("✅ 글로벌 기본 AI 모델이 **%s**(으)로 변경되었습니다.", modelName))
		return
	}

	if changeType == "session" {
		// 현재 채널이 쓰레드이고 세션이 존재하는지 확인
		sess, exists := h.sessionManager.GetSession(i.ChannelID)
		if !exists {
			h.respondError(s, i, "현재 채널은 활성화된 AI 대화 세션(쓰레드)이 아닙니다.")
			return
		}

		sess.SetModel(modelName)
		h.sessionManager.Save()
		h.respond(s, i, fmt.Sprintf("✅ 현재 세션의 AI 모델이 **%s**(으)로 변경되었습니다.", modelName))
		return
	}
}

// handleUsage는 /사용량 커맨드를 처리합니다.
func (h *Handler) handleUsage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.respond(s, i, "📊 사용량 대시보드를 생성합니다...")

	if err := h.dashboard.StartDashboard(s, i.ChannelID); err != nil {
		log.Printf("사용량 대시보드 시작 실패: %v", err)
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

			// 인터랙션 메시지에 쓰레드를 만들 수는 없으므로, 새 안내 메시지를 보내고 거기서 쓰레드를 만듦
			msg, err := s.ChannelMessageSend(i.ChannelID, "🔗 불러온 세션으로 대화를 시작합니다...")
			if err != nil {
				h.respondError(s, i, "쓰레드 생성용 메시지 전송 실패.")
				return
			}

			thread, err := s.MessageThreadStartComplex(i.ChannelID, msg.ID, &discordgo.ThreadStart{
				Name:                "불러온 대화 세션",
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
	}
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

// respondError는 에러 응답을 보냅니다.
func (h *Handler) respondError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	h.respond(s, i, fmt.Sprintf("❌ %s", message))
}
