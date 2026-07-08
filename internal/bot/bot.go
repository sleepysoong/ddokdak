// Package bot은 디스코드 봇의 초기화, 실행, 종료를 관리합니다.
package bot

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/agy"
	"github.com/sleepysoong/ddokdak/internal/command"
	"github.com/sleepysoong/ddokdak/internal/config"
	"github.com/sleepysoong/ddokdak/internal/downloader"
	"github.com/sleepysoong/ddokdak/internal/handler"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
	"github.com/sleepysoong/ddokdak/internal/usage"
)

// Bot은 디스코드 봇의 핵심 구조체입니다.
type Bot struct {
	session            *discordgo.Session
	config             *config.Config
	channelStore       store.ChannelStore
	sessionManager     *session.SessionManager
	agyClient          *agy.Client
	commandHandler     *command.Handler
	messageHandler     *handler.MessageHandler
	dashboard          *usage.Dashboard
	registeredCommands []*discordgo.ApplicationCommand
}

// New는 새로운 Bot 인스턴스를 생성합니다.
func New(cfg *config.Config) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, fmt.Errorf("디스코드 세션 생성 실패: %w", err)
	}

	dg.Identify.Intents = discordgo.IntentsGuildMessages |
		discordgo.IntentsMessageContent |
		discordgo.IntentsGuilds

	logDir := filepath.Join(".", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("로그 디렉토리 생성 실패: %w", err)
	}

	channelStore := store.NewInMemoryChannelStore()
	sessionManager := session.NewSessionManager(filepath.Join(".", "data"))
	agyClient := agy.NewClient(cfg.AgyTimeout.String(), logDir)

	dlDir := filepath.Join(".", "downloads")
	dl, err := downloader.New(dlDir)
	if err != nil {
		return nil, fmt.Errorf("다운로더 초기화 실패: %w", err)
	}

	usageTracker := usage.NewTracker()
	dashboard := usage.NewDashboard(usageTracker)

	commandHandler := command.NewHandler(channelStore, cfg, sessionManager, dashboard)
	messageHandler := handler.NewMessageHandler(channelStore, sessionManager, agyClient, cfg, dl, usageTracker)

	return &Bot{
		session:        dg,
		config:         cfg,
		channelStore:   channelStore,
		sessionManager: sessionManager,
		agyClient:      agyClient,
		commandHandler: commandHandler,
		messageHandler: messageHandler,
		dashboard:      dashboard,
	}, nil
}

// Start는 봇을 시작합니다.
func (b *Bot) Start() error {
	b.session.AddHandler(b.commandHandler.HandleInteraction)
	b.session.AddHandler(b.messageHandler.HandleMessage)
	b.session.AddHandler(b.messageHandler.HandleMessageDelete)

	b.session.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("봇이 %s#%s로 로그인했습니다.", r.User.Username, r.User.Discriminator)
	})

	if err := b.session.Open(); err != nil {
		return fmt.Errorf("디스코드 연결 실패: %w", err)
	}

	registeredCommands, err := command.RegisterCommands(b.session, "")
	if err != nil {
		return fmt.Errorf("슬래시 커맨드 등록 실패: %w", err)
	}
	b.registeredCommands = registeredCommands
	log.Printf("%d개의 슬래시 커맨드가 등록되었습니다.", len(registeredCommands))

	return nil
}

// Wait는 종료 시그널을 기다립니다.
func (b *Bot) Wait() {
	log.Println("봇이 실행 중입니다. 종료하려면 Ctrl+C를 누르세요.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
	log.Println("종료 시그널을 받았습니다.")
}

// Stop은 봇을 종료합니다.
func (b *Bot) Stop() error {
	log.Println("봇을 종료합니다...")

	// 대시보드 자동 업데이트 중지
	b.dashboard.StopAllDashboards()

	// 슬래시 커맨드 해제
	if err := command.UnregisterCommands(b.session, "", b.registeredCommands); err != nil {
		log.Printf("슬래시 커맨드 해제 실패: %v", err)
	}

	// 디스코드 연결 종료
	if err := b.session.Close(); err != nil {
		return fmt.Errorf("디스코드 연결 종료 실패: %w", err)
	}

	log.Println("봇이 정상적으로 종료되었습니다.")
	return nil
}
