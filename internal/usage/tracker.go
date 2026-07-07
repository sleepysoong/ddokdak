// Package usage는 Antigravity CLI의 모델별 사용량을 추적하고
// 디스코드 메시지로 대시보드를 표시합니다.
package usage

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// kst는 한국 표준 시간대입니다.
var kst = time.FixedZone("KST", 9*60*60)

// ModelUsage는 모델별 사용 통계입니다.
type ModelUsage struct {
	ModelName    string
	CallCount    int64
	InputTokens  int64
	OutputTokens int64
	LastUsedAt   time.Time
	ErrorCount   int64
}

// Tracker는 모델별 사용량을 추적합니다.
type Tracker struct {
	mu        sync.RWMutex
	usages    map[string]*ModelUsage // 모델명 -> 사용량
	startedAt time.Time
}

// NewTracker는 새로운 Tracker를 생성합니다.
func NewTracker() *Tracker {
	return &Tracker{
		usages:    make(map[string]*ModelUsage),
		startedAt: time.Now(),
	}
}

// RecordCall은 모델 호출과 토큰 사용량을 기록합니다.
func (t *Tracker) RecordCall(modelName string, inputTokens, outputTokens int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	u, ok := t.usages[modelName]
	if !ok {
		u = &ModelUsage{ModelName: modelName}
		t.usages[modelName] = u
	}

	u.CallCount++
	u.InputTokens += inputTokens
	u.OutputTokens += outputTokens
	u.LastUsedAt = time.Now()
}

// RecordError은 모델 호출 에러를 기록합니다.
func (t *Tracker) RecordError(modelName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	u, ok := t.usages[modelName]
	if !ok {
		u = &ModelUsage{ModelName: modelName}
		t.usages[modelName] = u
	}

	u.ErrorCount++
	u.LastUsedAt = time.Now()
}

// GetUsages는 모든 모델의 사용량을 호출 횟수 내림차순으로 정렬하여 반환합니다.
func (t *Tracker) GetUsages() []ModelUsage {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]ModelUsage, 0, len(t.usages))
	for _, u := range t.usages {
		result = append(result, *u)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CallCount > result[j].CallCount
	})

	return result
}

// formatTimeSince는 경과 시간을 한국어로 표시합니다.
// 예: "2분 전", "1시간 전", "3일 전"
func formatTimeSince(t time.Time) string {
	if t.IsZero() {
		return "사용 기록 없음"
	}

	d := time.Since(t)

	switch {
	case d < time.Minute:
		return "방금 전"
	case d < time.Hour:
		minutes := int(d.Minutes())
		return fmt.Sprintf("%d분 전", minutes)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		return fmt.Sprintf("%d시간 전", hours)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d일 전", days)
	}
}

// FormatDashboard는 사용량 정보를 디스코드 메시지 포맷으로 변환합니다.
// 이모지와 코드 블록을 사용하여 보기 좋게 포맷합니다.
func (t *Tracker) FormatDashboard() string {
	var sb strings.Builder

	sb.WriteString("📊 **AI 모델 사용량 대시보드**\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	usages := t.GetUsages()

	if len(usages) == 0 {
		sb.WriteString("📭 아직 사용 기록이 없습니다.\n\n")
	} else {
		for _, u := range usages {
			sb.WriteString(fmt.Sprintf("🤖 **%s**\n", u.ModelName))
			sb.WriteString(fmt.Sprintf("├ 호출: %d회 | 오류: %d회\n", u.CallCount, u.ErrorCount))
			sb.WriteString(fmt.Sprintf("├ 입력 토큰: ~%d | 출력 토큰: ~%d\n", u.InputTokens, u.OutputTokens))
			sb.WriteString(fmt.Sprintf("└ 마지막 사용: %s\n\n", formatTimeSince(u.LastUsedAt)))
		}
	}

	t.mu.RLock()
	startedAt := t.startedAt
	t.mu.RUnlock()

	sb.WriteString(fmt.Sprintf("📅 추적 시작: %s\n", startedAt.In(kst).Format("2006-01-02 15:04")))
	sb.WriteString(fmt.Sprintf("🔄 마지막 업데이트: %s", time.Now().In(kst).Format("15:04:05")))

	return sb.String()
}

// Dashboard는 자동 업데이트되는 사용량 대시보드 메시지를 관리합니다.
type Dashboard struct {
	tracker          *Tracker
	stopChan         chan struct{}
	mu               sync.Mutex
	activeDashboards map[string]string // channelID -> messageID
}

// NewDashboard는 새로운 Dashboard를 생성합니다.
func NewDashboard(tracker *Tracker) *Dashboard {
	return &Dashboard{
		tracker:          tracker,
		stopChan:         make(chan struct{}),
		activeDashboards: make(map[string]string),
	}
}

// StartDashboard는 지정된 채널에 사용량 메시지를 전송하고 1분마다 자동 업데이트합니다.
// 이미 해당 채널에 대시보드가 있으면 기존 메시지를 삭제하고 새로 생성합니다.
func (d *Dashboard) StartDashboard(s *discordgo.Session, channelID string) error {
	d.mu.Lock()

	// 기존 대시보드 메시지가 있으면 삭제 시도
	if oldMsgID, exists := d.activeDashboards[channelID]; exists {
		_ = s.ChannelMessageDelete(channelID, oldMsgID)
		delete(d.activeDashboards, channelID)
	}

	d.mu.Unlock()

	// 새 대시보드 메시지 전송
	content := d.tracker.FormatDashboard()
	msg, err := s.ChannelMessageSend(channelID, content)
	if err != nil {
		return fmt.Errorf("usage: 대시보드 메시지 전송 실패: %w", err)
	}

	d.mu.Lock()
	d.activeDashboards[channelID] = msg.ID
	d.mu.Unlock()

	// 고루틴으로 1분마다 대시보드 자동 업데이트
	go d.runAutoUpdate(s, channelID)

	return nil
}

// runAutoUpdate는 1분마다 대시보드 메시지를 자동으로 업데이트합니다.
func (d *Dashboard) runAutoUpdate(s *discordgo.Session, channelID string) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.updateDashboard(s, channelID)
		}
	}
}

// StopAllDashboards는 모든 자동 업데이트를 중지합니다.
func (d *Dashboard) StopAllDashboards() {
	close(d.stopChan)
}

// updateDashboard는 특정 채널의 대시보드 메시지를 업데이트합니다.
func (d *Dashboard) updateDashboard(s *discordgo.Session, channelID string) {
	d.mu.Lock()
	msgID, exists := d.activeDashboards[channelID]
	d.mu.Unlock()

	if !exists {
		return
	}

	content := d.tracker.FormatDashboard()
	_, err := s.ChannelMessageEdit(channelID, msgID, content)
	if err != nil {
		log.Printf("usage: 대시보드 업데이트 실패 (채널: %s): %v", channelID, err)

		// 메시지가 삭제된 경우 활성 대시보드에서 제거
		d.mu.Lock()
		delete(d.activeDashboards, channelID)
		d.mu.Unlock()
	}
}
