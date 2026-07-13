package format

import (
	"testing"
	"time"
)

func TestComma(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{-1234, "-1,234"},
		{999999999, "999,999,999"},
	}

	for _, tt := range tests {
		result := Comma(tt.input)
		if result != tt.expected {
			t.Errorf("Comma(%d) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCostUSD(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0, "0.00"},
		{0.005, "0.0050"},
		{1.5, "1.50"},
		{123.456, "123.46"},
	}

	for _, tt := range tests {
		result := CostUSD(tt.input)
		if result != tt.expected {
			t.Errorf("CostUSD(%f) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestCostLine(t *testing.T) {
	result := CostLine(1.0, 1500.0)
	if result != "$1.00 (약 1,500원)" {
		t.Errorf("CostLine(1.0, 1500.0) = %q; want %q", result, "$1.00 (약 1,500원)")
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected string
	}{
		{0, "0s"},
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{5 * time.Minute, "5m 0s"},
		{time.Hour + 2*time.Minute + 3*time.Second, "62m 3s"},
	}

	for _, tt := range tests {
		result := Duration(tt.input)
		if result != tt.expected {
			t.Errorf("Duration(%v) = %q; want %q", tt.input, result, tt.expected)
		}
	}
}

func TestExchangeRateCache(t *testing.T) {
	// 첫 호출은 API를 호출하거나 폴백 반환
	rate1 := ExchangeRate()
	if rate1 <= 0 {
		t.Errorf("ExchangeRate() returned non-positive: %f", rate1)
	}

	// 두 번째 호출은 캐시에서 반환되어야 함 (10분 TTL 내)
	rate2 := ExchangeRate()
	if rate1 != rate2 {
		t.Errorf("ExchangeRate() cache miss: first=%f, second=%f", rate1, rate2)
	}
}
