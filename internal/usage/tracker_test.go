package usage

import (
	"strings"
	"sync"
	"testing"
	"time"
)

// TestRecordCall은 모델 호출 기록이 올바르게 카운트되는지 검증합니다.
func TestRecordCall(t *testing.T) {
	tracker := NewTracker()

	tracker.RecordCall("Claude Opus 4.6 (Thinking)")
	tracker.RecordCall("Claude Opus 4.6 (Thinking)")
	tracker.RecordCall("Gemini 3.1 Pro (High)")

	usages := tracker.GetUsages()
	if len(usages) != 2 {
		t.Fatalf("기대한 모델 수 2, 실제: %d", len(usages))
	}

	// 호출 횟수 내림차순 정렬이므로 첫 번째가 Claude
	if usages[0].ModelName != "Claude Opus 4.6 (Thinking)" {
		t.Errorf("첫 번째 모델 기대: Claude Opus 4.6 (Thinking), 실제: %s", usages[0].ModelName)
	}
	if usages[0].CallCount != 2 {
		t.Errorf("Claude 호출 횟수 기대: 2, 실제: %d", usages[0].CallCount)
	}

	if usages[1].ModelName != "Gemini 3.1 Pro (High)" {
		t.Errorf("두 번째 모델 기대: Gemini 3.1 Pro (High), 실제: %s", usages[1].ModelName)
	}
	if usages[1].CallCount != 1 {
		t.Errorf("Gemini 호출 횟수 기대: 1, 실제: %d", usages[1].CallCount)
	}
}

// TestRecordError은 에러 기록이 올바르게 카운트되는지 검증합니다.
func TestRecordError(t *testing.T) {
	tracker := NewTracker()

	tracker.RecordCall("Claude Opus 4.6 (Thinking)")
	tracker.RecordError("Claude Opus 4.6 (Thinking)")
	tracker.RecordError("Claude Opus 4.6 (Thinking)")

	usages := tracker.GetUsages()
	if len(usages) != 1 {
		t.Fatalf("기대한 모델 수 1, 실제: %d", len(usages))
	}

	if usages[0].CallCount != 1 {
		t.Errorf("호출 횟수 기대: 1, 실제: %d", usages[0].CallCount)
	}
	if usages[0].ErrorCount != 2 {
		t.Errorf("에러 횟수 기대: 2, 실제: %d", usages[0].ErrorCount)
	}
}

// TestRecordErrorWithoutCall은 호출 없이 에러만 기록해도 정상 동작하는지 검증합니다.
func TestRecordErrorWithoutCall(t *testing.T) {
	tracker := NewTracker()

	tracker.RecordError("Gemini 3.5 Flash (High)")

	usages := tracker.GetUsages()
	if len(usages) != 1 {
		t.Fatalf("기대한 모델 수 1, 실제: %d", len(usages))
	}

	if usages[0].CallCount != 0 {
		t.Errorf("호출 횟수 기대: 0, 실제: %d", usages[0].CallCount)
	}
	if usages[0].ErrorCount != 1 {
		t.Errorf("에러 횟수 기대: 1, 실제: %d", usages[0].ErrorCount)
	}
}

// TestGetUsagesSortedByCallCount는 GetUsages가 호출 횟수 내림차순으로 정렬하는지 검증합니다.
func TestGetUsagesSortedByCallCount(t *testing.T) {
	tracker := NewTracker()

	// 각기 다른 횟수로 호출
	for i := 0; i < 5; i++ {
		tracker.RecordCall("모델A")
	}
	for i := 0; i < 10; i++ {
		tracker.RecordCall("모델B")
	}
	for i := 0; i < 3; i++ {
		tracker.RecordCall("모델C")
	}

	usages := tracker.GetUsages()
	if len(usages) != 3 {
		t.Fatalf("기대한 모델 수 3, 실제: %d", len(usages))
	}

	// 내림차순 검증: 모델B(10) > 모델A(5) > 모델C(3)
	expectedOrder := []struct {
		name  string
		count int64
	}{
		{"모델B", 10},
		{"모델A", 5},
		{"모델C", 3},
	}

	for i, expected := range expectedOrder {
		if usages[i].ModelName != expected.name {
			t.Errorf("인덱스 %d: 기대 모델 %s, 실제: %s", i, expected.name, usages[i].ModelName)
		}
		if usages[i].CallCount != expected.count {
			t.Errorf("인덱스 %d: 기대 횟수 %d, 실제: %d", i, expected.count, usages[i].CallCount)
		}
	}
}

// TestGetUsagesEmpty는 기록이 없을 때 빈 슬라이스를 반환하는지 검증합니다.
func TestGetUsagesEmpty(t *testing.T) {
	tracker := NewTracker()

	usages := tracker.GetUsages()
	if len(usages) != 0 {
		t.Errorf("빈 트래커에서 기대한 결과 길이 0, 실제: %d", len(usages))
	}
}

// TestFormatDashboardEmpty는 사용 기록이 없을 때 대시보드 포맷을 검증합니다.
func TestFormatDashboardEmpty(t *testing.T) {
	tracker := NewTracker()

	output := tracker.FormatDashboard()

	if !strings.Contains(output, "📊 **AI 모델 사용량 대시보드**") {
		t.Error("대시보드 제목이 포함되어야 합니다")
	}
	if !strings.Contains(output, "📭 아직 사용 기록이 없습니다.") {
		t.Error("빈 상태 메시지가 포함되어야 합니다")
	}
	if !strings.Contains(output, "📅 추적 시작:") {
		t.Error("추적 시작 시간이 포함되어야 합니다")
	}
	if !strings.Contains(output, "🔄 마지막 업데이트:") {
		t.Error("마지막 업데이트 시간이 포함되어야 합니다")
	}
}

// TestFormatDashboardWithData는 사용 기록이 있을 때 대시보드 포맷을 검증합니다.
func TestFormatDashboardWithData(t *testing.T) {
	tracker := NewTracker()

	tracker.RecordCall("Claude Opus 4.6 (Thinking)")
	tracker.RecordCall("Claude Opus 4.6 (Thinking)")
	tracker.RecordError("Claude Opus 4.6 (Thinking)")
	tracker.RecordCall("Gemini 3.1 Pro (High)")

	output := tracker.FormatDashboard()

	// 모델 정보가 포함되어야 함
	if !strings.Contains(output, "🤖 **Claude Opus 4.6 (Thinking)**") {
		t.Error("Claude 모델명이 포함되어야 합니다")
	}
	if !strings.Contains(output, "🤖 **Gemini 3.1 Pro (High)**") {
		t.Error("Gemini 모델명이 포함되어야 합니다")
	}

	// 호출/에러 횟수 포맷 확인
	if !strings.Contains(output, "호출: 2회 | 오류: 1회") {
		t.Error("Claude 통계가 올바르게 포맷되어야 합니다")
	}
	if !strings.Contains(output, "호출: 1회 | 오류: 0회") {
		t.Error("Gemini 통계가 올바르게 포맷되어야 합니다")
	}

	// 마지막 사용 시간 표시 확인 (방금 기록했으므로 "방금 전")
	if !strings.Contains(output, "마지막 사용: 방금 전") {
		t.Error("마지막 사용 시간이 '방금 전'으로 표시되어야 합니다")
	}
}

// TestFormatTimeSince는 경과 시간 포맷팅을 검증합니다.
func TestFormatTimeSince(t *testing.T) {
	tests := []struct {
		name     string
		elapsed  time.Duration
		expected string
	}{
		{"제로 시간", 0, "사용 기록 없음"},
		{"30초 전", 30 * time.Second, "방금 전"},
		{"5분 전", 5 * time.Minute, "5분 전"},
		{"2시간 전", 2 * time.Hour, "2시간 전"},
		{"3일 전", 72 * time.Hour, "3일 전"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var input time.Time
			if tt.elapsed == 0 {
				input = time.Time{} // 제로 값
			} else {
				input = time.Now().Add(-tt.elapsed)
			}

			result := formatTimeSince(input)
			if result != tt.expected {
				t.Errorf("기대: %q, 실제: %q", tt.expected, result)
			}
		})
	}
}

// TestConcurrentAccess는 동시에 여러 고루틴에서 접근해도 데이터 경합이 없는지 검증합니다.
func TestConcurrentAccess(t *testing.T) {
	tracker := NewTracker()

	const goroutines = 100
	const callsPerGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 3) // RecordCall, RecordError, GetUsages 각각

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				tracker.RecordCall("동시성 테스트 모델")
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				tracker.RecordError("동시성 테스트 모델")
			}
		}()
	}

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < callsPerGoroutine; j++ {
				_ = tracker.GetUsages()
			}
		}()
	}

	wg.Wait()

	usages := tracker.GetUsages()
	if len(usages) != 1 {
		t.Fatalf("기대한 모델 수 1, 실제: %d", len(usages))
	}

	expectedCalls := int64(goroutines * callsPerGoroutine)
	if usages[0].CallCount != expectedCalls {
		t.Errorf("호출 횟수 기대: %d, 실제: %d", expectedCalls, usages[0].CallCount)
	}

	expectedErrors := int64(goroutines * callsPerGoroutine)
	if usages[0].ErrorCount != expectedErrors {
		t.Errorf("에러 횟수 기대: %d, 실제: %d", expectedErrors, usages[0].ErrorCount)
	}
}

// TestConcurrentFormatDashboard는 대시보드 포맷팅이 동시 접근에서도 안전한지 검증합니다.
func TestConcurrentFormatDashboard(t *testing.T) {
	tracker := NewTracker()

	tracker.RecordCall("테스트 모델")

	var wg sync.WaitGroup
	wg.Add(50)

	for i := 0; i < 50; i++ {
		go func() {
			defer wg.Done()
			output := tracker.FormatDashboard()
			if !strings.Contains(output, "📊") {
				t.Error("대시보드 출력에 이모지가 포함되어야 합니다")
			}
		}()
	}

	wg.Wait()
}

// TestNewTrackerInitialization은 NewTracker의 초기 상태를 검증합니다.
func TestNewTrackerInitialization(t *testing.T) {
	before := time.Now()
	tracker := NewTracker()
	after := time.Now()

	if tracker.usages == nil {
		t.Error("usages 맵이 nil이 아니어야 합니다")
	}
	if len(tracker.usages) != 0 {
		t.Errorf("초기 usages 길이 기대: 0, 실제: %d", len(tracker.usages))
	}

	if tracker.startedAt.Before(before) || tracker.startedAt.After(after) {
		t.Error("startedAt이 생성 시점과 일치해야 합니다")
	}
}

// TestNewDashboardInitialization은 NewDashboard의 초기 상태를 검증합니다.
func TestNewDashboardInitialization(t *testing.T) {
	tracker := NewTracker()
	dashboard := NewDashboard(tracker)

	if dashboard.tracker != tracker {
		t.Error("대시보드의 tracker가 주입된 tracker와 동일해야 합니다")
	}
	if dashboard.activeDashboards == nil {
		t.Error("activeDashboards 맵이 nil이 아니어야 합니다")
	}
	if len(dashboard.activeDashboards) != 0 {
		t.Errorf("초기 activeDashboards 길이 기대: 0, 실제: %d", len(dashboard.activeDashboards))
	}
	if dashboard.stopChan == nil {
		t.Error("stopChan이 nil이 아니어야 합니다")
	}
}

// TestLastUsedAtUpdated는 RecordCall과 RecordError가 LastUsedAt을 갱신하는지 검증합니다.
func TestLastUsedAtUpdated(t *testing.T) {
	tracker := NewTracker()

	before := time.Now()
	tracker.RecordCall("테스트 모델")
	after := time.Now()

	usages := tracker.GetUsages()
	if usages[0].LastUsedAt.Before(before) || usages[0].LastUsedAt.After(after) {
		t.Error("RecordCall 후 LastUsedAt이 현재 시간이어야 합니다")
	}

	time.Sleep(10 * time.Millisecond) // 시간 차이를 만들기 위해

	beforeError := time.Now()
	tracker.RecordError("테스트 모델")
	afterError := time.Now()

	usages = tracker.GetUsages()
	if usages[0].LastUsedAt.Before(beforeError) || usages[0].LastUsedAt.After(afterError) {
		t.Error("RecordError 후 LastUsedAt이 갱신되어야 합니다")
	}
}
