package command

import (
	"testing"

	"github.com/sleepysoong/ddokdak/internal/config"
	"github.com/sleepysoong/ddokdak/internal/session"
	"github.com/sleepysoong/ddokdak/internal/store"
	"github.com/sleepysoong/ddokdak/internal/usage"
)

func TestNewHandler(t *testing.T) {
	channelStore := store.NewInMemoryChannelStore()
	cfg := &config.Config{}
	sm := session.NewSessionManager(t.TempDir())
	dashboard := usage.NewDashboard(usage.NewTracker())
	handler := NewHandler(channelStore, cfg, sm, dashboard)

	if handler == nil {
		t.Fatal("NewHandler()가 nil을 반환했습니다")
	}

	if handler.channelStore == nil {
		t.Fatal("channelStore가 nil입니다")
	}
}

func TestHandlerHasChannelStore(t *testing.T) {
	channelStore := store.NewInMemoryChannelStore()
	cfg := &config.Config{}
	sm := session.NewSessionManager(t.TempDir())
	dashboard := usage.NewDashboard(usage.NewTracker())
	handler := NewHandler(channelStore, cfg, sm, dashboard)

	// channelStore가 올바르게 설정되었는지 확인
	if handler.channelStore != channelStore {
		t.Error("channelStore가 올바르게 설정되지 않았습니다")
	}
}
