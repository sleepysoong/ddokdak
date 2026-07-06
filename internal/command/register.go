// Package command는 디스코드 슬래시 커맨드의 등록 및 관리를 담당합니다.
package command

import (
	"github.com/bwmarrin/discordgo"
)

// commands는 봇에서 사용하는 슬래시 커맨드 목록입니다.
var commands = []*discordgo.ApplicationCommand{
	{
		Name:        "채널지정",
		Description: "이 채널을 AI 대화 채널로 지정합니다.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "채널",
				Description: "AI 대화를 활성화할 채널을 선택하세요.",
				Required:    true,
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildText,
				},
			},
		},
	},
	{
		Name:        "채널해제",
		Description: "지정된 AI 대화 채널을 해제합니다.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "채널",
				Description: "AI 대화를 해제할 채널을 선택하세요.",
				Required:    true,
				ChannelTypes: []discordgo.ChannelType{
					discordgo.ChannelTypeGuildText,
				},
			},
		},
	},
	{
		Name:        "모델변경",
		Description: "AI 모델을 변경합니다.",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "타입",
				Description: "글로벌 전체 변경인지, 현재 세션 변경인지 선택하세요.",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "세션 (현재 대화만)", Value: "session"},
					{Name: "글로벌 (기본값)", Value: "global"},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "모델명",
				Description: "변경할 모델명을 입력하세요 (예: Gemini 3.1 Pro (High))",
				Required:    true,
			},
		},
	},
}

// RegisterCommands는 디스코드 서버에 슬래시 커맨드를 등록합니다.
// guildID가 빈 문자열이면 글로벌 커맨드로 등록합니다.
func RegisterCommands(s *discordgo.Session, guildID string) ([]*discordgo.ApplicationCommand, error) {
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, cmd := range commands {
		registered, err := s.ApplicationCommandCreate(s.State.User.ID, guildID, cmd)
		if err != nil {
			return nil, err
		}
		registeredCommands[i] = registered
	}
	return registeredCommands, nil
}

// UnregisterCommands는 등록된 슬래시 커맨드를 제거합니다.
func UnregisterCommands(s *discordgo.Session, guildID string, registeredCommands []*discordgo.ApplicationCommand) error {
	for _, cmd := range registeredCommands {
		if err := s.ApplicationCommandDelete(s.State.User.ID, guildID, cmd.ID); err != nil {
			return err
		}
	}
	return nil
}
