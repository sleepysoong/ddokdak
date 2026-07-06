package bot

import (
	"testing"

	"github.com/sleepysoong/ddokdak/internal/config"
)

func TestNew(t *testing.T) {
	// 유효하지 않은 토큰이라도 Bot 구조체는 생성되어야 합니다.
	cfg := &config.Config{
		DiscordToken:     "test-token",
		AgyModel:         "test-model",
		AgyFallbackModel: "test-fallback",
		AgyTimeout:       300000000000, // 5m
		LogLevel:         "info",
	}

	b, err := New(cfg)
	if err != nil {
		t.Fatalf("New() 에러: %v", err)
	}

	if b == nil {
		t.Fatal("New()가 nil을 반환했습니다")
	}

	if b.config != cfg {
		t.Error("config가 올바르게 설정되지 않았습니다")
	}

	if b.channelStore == nil {
		t.Error("channelStore가 nil입니다")
	}

	if b.sessionManager == nil {
		t.Error("sessionManager가 nil입니다")
	}

	if b.agyClient == nil {
		t.Error("agyClient가 nil입니다")
	}

	if b.commandHandler == nil {
		t.Error("commandHandler가 nil입니다")
	}

	if b.messageHandler == nil {
		t.Error("messageHandler가 nil입니다")
	}

	if b.session == nil {
		t.Error("디스코드 세션이 nil입니다")
	}
}
