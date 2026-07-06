// Package config는 ddokdak 디스코드 봇의 설정을 관리합니다.
// 환경변수에서 설정값을 로드하며, 필수값 검증과 기본값 처리를 제공합니다.
package config

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// 환경변수 키 상수
const (
	envDiscordToken    = "DISCORD_TOKEN"
	envAgyModel        = "AGY_MODEL"
	envAgyFallbackModel = "AGY_FALLBACK_MODEL"
	envAgyTimeout      = "AGY_TIMEOUT"
	envLogLevel        = "LOG_LEVEL"
)

// 기본값 상수
const (
	DefaultAgyModel        = "Claude Opus 4.6 (Thinking)"
	DefaultAgyFallbackModel = "Gemini 3.1 Pro (High)"
	DefaultAgyTimeout      = "5m"
	DefaultLogLevel        = "info"
)

// Config는 ddokdak 봇의 전체 설정을 담는 구조체입니다.
type Config struct {
	// DiscordToken은 디스코드 봇 인증에 사용되는 토큰입니다. (필수)
	DiscordToken string

	// AgyModel은 Antigravity CLI에서 사용할 기본 모델명입니다.
	AgyModel string

	// AgyFallbackModel은 기본 모델 사용 불가 시 대체할 모델명입니다.
	AgyFallbackModel string

	// AgyTimeout은 agy 명령 실행 시 적용되는 타임아웃입니다.
	AgyTimeout time.Duration

	// LogLevel은 로깅 수준을 나타냅니다. (debug, info, warn, error)
	LogLevel string

	mu sync.RWMutex
}

// SetGlobalModel은 글로벌 AI 모델을 변경합니다.
func (c *Config) SetGlobalModel(model string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.AgyModel = model
}

// GetGlobalModel은 현재 설정된 글로벌 AI 모델을 반환합니다.
func (c *Config) GetGlobalModel() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.AgyModel
}

// LoadConfig는 환경변수에서 설정값을 읽어 Config 구조체를 반환합니다.
// DISCORD_TOKEN이 설정되지 않은 경우 에러를 반환합니다.
func LoadConfig() (*Config, error) {
	discordToken := os.Getenv(envDiscordToken)
	if discordToken == "" {
		return nil, fmt.Errorf("필수 환경변수 %s가 설정되지 않았습니다", envDiscordToken)
	}

	agyModel := getEnvOrDefault(envAgyModel, DefaultAgyModel)
	agyFallbackModel := getEnvOrDefault(envAgyFallbackModel, DefaultAgyFallbackModel)
	agyTimeoutStr := getEnvOrDefault(envAgyTimeout, DefaultAgyTimeout)
	logLevel := getEnvOrDefault(envLogLevel, DefaultLogLevel)

	agyTimeout, err := time.ParseDuration(agyTimeoutStr)
	if err != nil {
		return nil, fmt.Errorf("환경변수 %s의 값 %q을(를) 파싱할 수 없습니다: %w", envAgyTimeout, agyTimeoutStr, err)
	}

	if !isValidLogLevel(logLevel) {
		return nil, fmt.Errorf("환경변수 %s의 값 %q은(는) 유효하지 않습니다 (허용: debug, info, warn, error)", envLogLevel, logLevel)
	}

	return &Config{
		DiscordToken:     discordToken,
		AgyModel:         agyModel,
		AgyFallbackModel: agyFallbackModel,
		AgyTimeout:       agyTimeout,
		LogLevel:         logLevel,
	}, nil
}

// getEnvOrDefault는 환경변수 값을 반환하되, 비어 있으면 기본값을 반환합니다.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// isValidLogLevel은 주어진 로그 레벨이 유효한지 검사합니다.
func isValidLogLevel(level string) bool {
	switch level {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}
