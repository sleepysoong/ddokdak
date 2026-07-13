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
		Description: "글로벌 기본 AI 모델 또는 현재 세션의 AI 모델을 변경합니다.",
	},
	{
		Name:        "사용량",
		Description: "AI 모델별 사용량 대시보드를 표시합니다. 1분마다 자동으로 업데이트됩니다.",
	},
	{
		Name:        "로그",
		Description: "현재 쓰레드의 AI 로그 파일을 다운로드합니다.",
	},
	{
		Name:        "new",
		Description: "기존 AI 대화 세션 목록을 불러와 이어서 대화할 수 있습니다.",
	},
	{
		Name:        "세션종료",
		Description: "현재 쓰레드의 AI 세션을 종료하고 아카이브 및 잠금 처리합니다.",
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
