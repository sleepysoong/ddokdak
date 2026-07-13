// Package format는 숫자 포맷팅, 환율 조회 등 공통 유틸리티를 제공합니다.
package format

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ──────────────────────────────────────────────
// 숫자 포맷팅
// ──────────────────────────────────────────────

// Comma는 정수를 세 자리마다 쉼표로 구분된 문자열로 변환합니다.
func Comma(n int64) string {
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

// CostUSD는 USD 금액을 적절한 정밀도로 포맷팅합니다.
func CostUSD(cost float64) string {
	if cost == 0 {
		return "0.00"
	}
	if cost < 0.01 {
		return fmt.Sprintf("%.4f", cost)
	}
	return fmt.Sprintf("%.2f", cost)
}

// CostLine은 USD 비용을 환율을 적용하여 "$(USD) (약 (KRW)원)" 형식의 문자열을 반환합니다.
func CostLine(costUSD, rate float64) string {
	costKRW := costUSD * rate
	return fmt.Sprintf("$%s (약 %s원)", CostUSD(costUSD), Comma(int64(costKRW+0.5)))
}

// Duration은 time.Duration을 "Xm Ys" 또는 "Xs" 형식의 한국어 문자열로 변환합니다.
func Duration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

// ──────────────────────────────────────────────
// 환율 조회 (TTL 캐시 포함)
// ──────────────────────────────────────────────

const (
	exchangeRateTTL      = 10 * time.Minute
	exchangeRateFallback = 1500.0
)

var (
	cachedRate     float64
	cachedRateTime time.Time
	rateMu         sync.Mutex
)

// ExchangeRate는 USD/KRW 환율을 반환합니다.
// 10분 TTL 캐시를 사용하여 불필요한 외부 API 호출을 최소화합니다.
func ExchangeRate() float64 {
	rateMu.Lock()
	defer rateMu.Unlock()

	if cachedRate > 0 && time.Since(cachedRateTime) < exchangeRateTTL {
		return cachedRate
	}

	rate := fetchExchangeRate()
	cachedRate = rate
	cachedRateTime = time.Now()
	return rate
}

func fetchExchangeRate() float64 {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return exchangeRateFallback
	}
	defer resp.Body.Close()

	var result struct {
		Rates map[string]float64 `json:"rates"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return exchangeRateFallback
	}

	rate, ok := result.Rates["KRW"]
	if !ok || rate <= 0 {
		return exchangeRateFallback
	}
	return rate
}
