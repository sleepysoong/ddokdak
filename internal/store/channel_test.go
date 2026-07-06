package store

import (
	"fmt"
	"sync"
	"testing"
)

func TestAddChannel(t *testing.T) {
	s := NewInMemoryChannelStore()

	if err := s.AddChannel("guild1", "ch1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if err := s.AddChannel("guild1", "ch2"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 동일 채널 중복 추가 시 에러
	if err := s.AddChannel("guild1", "ch1"); err == nil {
		t.Fatal("expected error for duplicate channel, got nil")
	}

	// 다른 길드에 같은 채널 ID 추가는 허용
	if err := s.AddChannel("guild2", "ch1"); err != nil {
		t.Fatalf("expected no error for same channel in different guild, got %v", err)
	}
}

func TestRemoveChannel(t *testing.T) {
	s := NewInMemoryChannelStore()

	_ = s.AddChannel("guild1", "ch1")
	_ = s.AddChannel("guild1", "ch2")

	if err := s.RemoveChannel("guild1", "ch1"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// 이미 삭제된 채널 재삭제 시 에러
	if err := s.RemoveChannel("guild1", "ch1"); err == nil {
		t.Fatal("expected error for removing non-existent channel, got nil")
	}

	// 존재하지 않는 길드에서 삭제 시 에러
	if err := s.RemoveChannel("guild999", "ch1"); err == nil {
		t.Fatal("expected error for removing channel from non-existent guild, got nil")
	}

	// 마지막 채널 삭제 후 길드 맵 정리 확인
	if err := s.RemoveChannel("guild1", "ch2"); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	channels := s.GetChannels("guild1")
	if len(channels) != 0 {
		t.Fatalf("expected empty channels after removing all, got %v", channels)
	}
}

func TestIsRegistered(t *testing.T) {
	s := NewInMemoryChannelStore()

	_ = s.AddChannel("guild1", "ch1")

	if !s.IsRegistered("guild1", "ch1") {
		t.Fatal("expected channel ch1 to be registered in guild1")
	}

	if s.IsRegistered("guild1", "ch2") {
		t.Fatal("expected channel ch2 to not be registered in guild1")
	}

	if s.IsRegistered("guild999", "ch1") {
		t.Fatal("expected channel ch1 to not be registered in non-existent guild")
	}

	// 삭제 후 등록 해제 확인
	_ = s.RemoveChannel("guild1", "ch1")
	if s.IsRegistered("guild1", "ch1") {
		t.Fatal("expected channel ch1 to not be registered after removal")
	}
}

func TestGetChannels(t *testing.T) {
	s := NewInMemoryChannelStore()

	// 빈 길드 조회
	channels := s.GetChannels("guild1")
	if len(channels) != 0 {
		t.Fatalf("expected empty slice, got %v", channels)
	}

	_ = s.AddChannel("guild1", "ch1")
	_ = s.AddChannel("guild1", "ch2")
	_ = s.AddChannel("guild1", "ch3")
	_ = s.AddChannel("guild2", "ch4")

	guild1Channels := s.GetChannels("guild1")
	if len(guild1Channels) != 3 {
		t.Fatalf("expected 3 channels for guild1, got %d", len(guild1Channels))
	}

	expected := map[string]bool{"ch1": true, "ch2": true, "ch3": true}
	for _, ch := range guild1Channels {
		if !expected[ch] {
			t.Fatalf("unexpected channel %s in guild1", ch)
		}
	}

	guild2Channels := s.GetChannels("guild2")
	if len(guild2Channels) != 1 {
		t.Fatalf("expected 1 channel for guild2, got %d", len(guild2Channels))
	}
	if guild2Channels[0] != "ch4" {
		t.Fatalf("expected ch4, got %s", guild2Channels[0])
	}
}

func TestConcurrency(t *testing.T) {
	s := NewInMemoryChannelStore()
	var wg sync.WaitGroup

	// 동시에 여러 고루틴에서 채널 추가
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			guildID := fmt.Sprintf("guild%d", i%5)
			channelID := fmt.Sprintf("ch%d", i)
			_ = s.AddChannel(guildID, channelID)
		}(i)
	}
	wg.Wait()

	// 전체 채널 수 검증
	totalChannels := 0
	for i := 0; i < 5; i++ {
		guildID := fmt.Sprintf("guild%d", i)
		totalChannels += len(s.GetChannels(guildID))
	}
	if totalChannels != 100 {
		t.Fatalf("expected 100 total channels, got %d", totalChannels)
	}

	// 동시에 읽기/쓰기 혼합 작업
	for i := 0; i < 100; i++ {
		wg.Add(3)
		go func(i int) {
			defer wg.Done()
			guildID := fmt.Sprintf("guild%d", i%5)
			channelID := fmt.Sprintf("ch%d", i)
			s.IsRegistered(guildID, channelID)
		}(i)
		go func(i int) {
			defer wg.Done()
			guildID := fmt.Sprintf("guild%d", i%5)
			s.GetChannels(guildID)
		}(i)
		go func(i int) {
			defer wg.Done()
			guildID := fmt.Sprintf("guild%d", i%5)
			channelID := fmt.Sprintf("ch%d", i)
			_ = s.RemoveChannel(guildID, channelID)
		}(i)
	}
	wg.Wait()

	// 모든 채널 삭제되었는지 확인
	totalChannels = 0
	for i := 0; i < 5; i++ {
		guildID := fmt.Sprintf("guild%d", i)
		totalChannels += len(s.GetChannels(guildID))
	}
	if totalChannels != 0 {
		t.Fatalf("expected 0 total channels after concurrent removal, got %d", totalChannels)
	}
}

// InMemoryChannelStore가 ChannelStore 인터페이스를 구현하는지 컴파일 타임 검증
var _ ChannelStore = (*InMemoryChannelStore)(nil)
