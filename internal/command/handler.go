package command

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/config"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
)

// Handler는 슬래시 커맨드를 처리하는 핸들러입니다.
type Handler struct {
	channelStore   store.ChannelStore
	config         *config.Config
	sessionManager *session.SessionManager
}

// NewHandler는 새로운 커맨드 핸들러를 생성합니다.
func NewHandler(channelStore store.ChannelStore, cfg *config.Config, sm *session.SessionManager) *Handler {
	return &Handler{
		channelStore:   channelStore,
		config:         cfg,
		sessionManager: sm,
	}
}

// HandleInteraction은 슬래시 커맨드 인터랙션을 처리합니다.
func (h *Handler) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	data := i.ApplicationCommandData()

	switch data.Name {
	case "채널지정":
		h.handleSetChannel(s, i)
	case "채널해제":
		h.handleUnsetChannel(s, i)
	case "모델변경":
		h.handleModelChange(s, i)
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
		h.respond(s, i, fmt.Sprintf("✅ 현재 세션의 AI 모델이 **%s**(으)로 변경되었습니다.", modelName))
		return
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
