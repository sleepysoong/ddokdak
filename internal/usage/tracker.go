// Package usage는 Antigravity CLI의 모델별 사용량을 추적하고
// 디스코드 메시지로 대시보드를 표시합니다.
package usage

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/sleepysoong/ddokdak/internal/session"
)

// kst는 한국 표준 시간대입니다.
var kst = time.FixedZone("KST", 9*60*60)

// ModelUsage는 모델별 사용 통계입니다.
type ModelUsage struct {
	ModelName    string
	CallCount    int64
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

// RecordCall은 모델 호출을 기록합니다.
func (t *Tracker) RecordCall(modelName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	u, ok := t.usages[modelName]
	if !ok {
		u = &ModelUsage{ModelName: modelName}
		t.usages[modelName] = u
	}

	u.CallCount++
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
	sessionManager   *session.SessionManager
}

// NewDashboard는 새로운 Dashboard를 생성합니다.
func NewDashboard(tracker *Tracker, sessionManager *session.SessionManager) *Dashboard {
	return &Dashboard{
		tracker:          tracker,
		stopChan:         make(chan struct{}),
		activeDashboards: make(map[string]string),
		sessionManager:   sessionManager,
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

	content := d.FormatGlobalDashboard()
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
	defer func() {
		if r := recover(); r != nil {
			log.Printf("🔥 [Panic Recovered] runAutoUpdate: %v", r)
		}
	}()

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

	content := d.FormatGlobalDashboard()
	_, err := s.ChannelMessageEdit(channelID, msgID, content)
	if err != nil {
		log.Printf("usage: 대시보드 업데이트 실패 (채널: %s): %v", channelID, err)

		// 메시지가 삭제된 경우 활성 대시보드에서 제거
		d.mu.Lock()
		delete(d.activeDashboards, channelID)
		d.mu.Unlock()
	}
}

// FormatGlobalDashboard는 전체 세션의 모델별 토큰 사용량과 예상 비용을 집계하여 메시지로 포맷합니다.
func (d *Dashboard) FormatGlobalDashboard() string {
	var sb strings.Builder

	sb.WriteString("📊 **AI 모델 전체 사용량 대시보드**\n")
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	usages, err := d.sessionManager.GetGlobalTokenUsages()
	if err != nil {
		log.Printf("Failed to get global token usages: %v", err)
	}

	rate := getExchangeRate()
	var totalCostUSD float64

	if len(usages) == 0 {
		sb.WriteString("📭 아직 사용 기록이 없습니다.\n\n")
	} else {
		for _, u := range usages {
			sb.WriteString(fmt.Sprintf("🤖 **%s**\n", u.ModelName))
			sb.WriteString(fmt.Sprintf("├ 호출: %d회\n", u.CallCount))
			sb.WriteString(fmt.Sprintf("├ 입력 토큰: %s\n", formatComma(u.InputTokens)))
			sb.WriteString(fmt.Sprintf("├ 출력 토큰: %s\n", formatComma(u.OutputTokens)))

			pricing, ok := matchPricing(u.ModelName)
			if ok {
				inputCost := (float64(u.InputTokens) / 1000000.0) * pricing.InputPriceUSD
				outputCost := (float64(u.OutputTokens) / 1000000.0) * pricing.OutputPriceUSD
				costUSD := inputCost + outputCost
				totalCostUSD += costUSD
				costKRW := costUSD * rate

				sb.WriteString(fmt.Sprintf("└ 예상 비용: $%s (약 %s원)\n\n", formatCostUSD(costUSD), formatComma(int64(costKRW+0.5))))
			} else {
				sb.WriteString("└ 예상 비용: 가격 정보 없음\n\n")
			}
		}
	}

	totalCostKRW := totalCostUSD * rate
	sb.WriteString(fmt.Sprintf("💵 **총 누적 비용**: $%s (약 %s원)\n", formatCostUSD(totalCostUSD), formatComma(int64(totalCostKRW+0.5))))
	sb.WriteString(fmt.Sprintf("💱 실시간 환율: 1 USD = %s KRW\n", fmt.Sprintf("%.2f", rate)))
	sb.WriteString(fmt.Sprintf("🔄 마지막 업데이트: %s", time.Now().In(kst).Format("15:04:05")))

	return sb.String()
}

// FormatSessionDashboard는 특정 세션의 모델별 토큰 사용량과 예상 비용을 메시지로 포맷합니다.
func (d *Dashboard) FormatSessionDashboard(usages []session.ModelTokenUsage, threadID string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("📊 **현재 세션 사용량 리포트** (쓰레드 ID: `%s`)\n", threadID))
	sb.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	rate := getExchangeRate()
	var totalCostUSD float64

	if len(usages) == 0 {
		sb.WriteString("📭 이 세션에서 아직 사용한 토큰 기록이 없습니다.\n\n")
	} else {
		for _, u := range usages {
			sb.WriteString(fmt.Sprintf("🤖 **%s**\n", u.ModelName))
			sb.WriteString(fmt.Sprintf("├ 호출: %d회\n", u.CallCount))
			sb.WriteString(fmt.Sprintf("├ 입력 토큰: %s\n", formatComma(u.InputTokens)))
			sb.WriteString(fmt.Sprintf("├ 출력 토큰: %s\n", formatComma(u.OutputTokens)))

			pricing, ok := matchPricing(u.ModelName)
			if ok {
				inputCost := (float64(u.InputTokens) / 1000000.0) * pricing.InputPriceUSD
				outputCost := (float64(u.OutputTokens) / 1000000.0) * pricing.OutputPriceUSD
				costUSD := inputCost + outputCost
				totalCostUSD += costUSD
				costKRW := costUSD * rate

				sb.WriteString(fmt.Sprintf("└ 예상 비용: $%s (약 %s원)\n\n", formatCostUSD(costUSD), formatComma(int64(costKRW+0.5))))
			} else {
				sb.WriteString("└ 예상 비용: 가격 정보 없음\n\n")
			}
		}
	}

	totalCostKRW := totalCostUSD * rate
	sb.WriteString(fmt.Sprintf("💵 **세션 누적 비용**: $%s (약 %s원)\n", formatCostUSD(totalCostUSD), formatComma(int64(totalCostKRW+0.5))))
	sb.WriteString(fmt.Sprintf("💱 실시간 환율: 1 USD = %s KRW\n", fmt.Sprintf("%.2f", rate)))

	return sb.String()
}

type ModelPricing struct {
	InputPriceUSD  float64 // per 1M tokens
	OutputPriceUSD float64 // per 1M tokens
}

var pricingMap = map[string]ModelPricing{
	"Gemini 3.5 Flash": {
		InputPriceUSD:  1.5,
		OutputPriceUSD: 9.0,
	},
	"Gemini 3.1 Pro": {
		InputPriceUSD:  2.0,
		OutputPriceUSD: 12.0,
	},
	"Claude Sonnet 4.6": {
		InputPriceUSD:  3.0,
		OutputPriceUSD: 15.0,
	},
	"Claude Opus 4.6 (Thinking)": {
		InputPriceUSD:  5.0,
		OutputPriceUSD: 25.0,
	},
}

func matchPricing(modelName string) (ModelPricing, bool) {
	if strings.Contains(modelName, "Gemini 3.5 Flash") {
		return pricingMap["Gemini 3.5 Flash"], true
	}
	if strings.Contains(modelName, "Gemini 3.1 Pro") {
		return pricingMap["Gemini 3.1 Pro"], true
	}
	if strings.Contains(modelName, "Claude Sonnet") {
		return pricingMap["Claude Sonnet 4.6"], true
	}
	if strings.Contains(modelName, "Claude Opus") {
		return pricingMap["Claude Opus 4.6 (Thinking)"], true
	}
	return ModelPricing{}, false
}

func getExchangeRate() float64 {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return 1500.0 // Fallback
	}
	defer resp.Body.Close()

	var result struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 1500.0 // Fallback
	}

	rate, ok := result.Rates["KRW"]
	if !ok || rate <= 0 {
		return 1500.0 // Fallback
	}
	return rate
}

func formatComma(n int64) string {
	in := fmt.Sprintf("%d", n)
	out := make([]byte, len(in)+(len(in)-1)/3)
	if len(in) == 0 {
		return ""
	}
	for i, j, k := len(in)-1, len(out)-1, 0; i >= 0; i, j = i-1, j-1 {
		out[j] = in[i]
		k++
		if k%3 == 0 && i > 0 {
			j--
			out[j] = ','
		}
	}
	return string(out)
}

func formatCostUSD(cost float64) string {
	if cost == 0 {
		return "0.00"
	}
	if cost < 0.01 {
		return fmt.Sprintf("%.4f", cost)
	}
	return fmt.Sprintf("%.2f", cost)
}
