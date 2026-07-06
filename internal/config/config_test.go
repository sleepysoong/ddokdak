package config

import (
	"os"
	"testing"
	"time"
)

// setEnvVars는 테스트용 환경변수를 일괄 설정합니다.
func setEnvVars(t *testing.T, vars map[string]string) {
	t.Helper()
	for k, v := range vars {
		t.Setenv(k, v)
	}
}

func TestLoadConfig_AllEnvVarsSet(t *testing.T) {
	setEnvVars(t, map[string]string{
		"DISCORD_TOKEN":      "test-token-123",
		"AGY_MODEL":          "CustomModel",
		"AGY_FALLBACK_MODEL": "FallbackModel",
		"AGY_TIMEOUT":        "10m",
		"LOG_LEVEL":          "debug",
	})

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 에러 발생: %v", err)
	}

	if cfg.DiscordToken != "test-token-123" {
		t.Errorf("DiscordToken = %q, want %q", cfg.DiscordToken, "test-token-123")
	}
	if cfg.AgyModel != "CustomModel" {
		t.Errorf("AgyModel = %q, want %q", cfg.AgyModel, "CustomModel")
	}
	if cfg.AgyFallbackModel != "FallbackModel" {
		t.Errorf("AgyFallbackModel = %q, want %q", cfg.AgyFallbackModel, "FallbackModel")
	}
	if cfg.AgyTimeout != 10*time.Minute {
		t.Errorf("AgyTimeout = %v, want %v", cfg.AgyTimeout, 10*time.Minute)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
}

func TestLoadConfig_DefaultValues(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "test-token")
	// 나머지 환경변수는 설정하지 않아 기본값이 적용되어야 함

	// 기존에 설정되어 있을 수 있는 환경변수를 명시적으로 제거
	os.Unsetenv("AGY_MODEL")
	os.Unsetenv("AGY_FALLBACK_MODEL")
	os.Unsetenv("AGY_TIMEOUT")
	os.Unsetenv("LOG_LEVEL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() 에러 발생: %v", err)
	}

	if cfg.AgyModel != DefaultAgyModel {
		t.Errorf("AgyModel = %q, want default %q", cfg.AgyModel, DefaultAgyModel)
	}
	if cfg.AgyFallbackModel != DefaultAgyFallbackModel {
		t.Errorf("AgyFallbackModel = %q, want default %q", cfg.AgyFallbackModel, DefaultAgyFallbackModel)
	}
	if cfg.AgyTimeout != 5*time.Minute {
		t.Errorf("AgyTimeout = %v, want default %v", cfg.AgyTimeout, 5*time.Minute)
	}
	if cfg.LogLevel != DefaultLogLevel {
		t.Errorf("LogLevel = %q, want default %q", cfg.LogLevel, DefaultLogLevel)
	}
}

func TestLoadConfig_MissingDiscordToken(t *testing.T) {
	os.Unsetenv("DISCORD_TOKEN")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("DISCORD_TOKEN 미설정 시 에러가 발생해야 합니다")
	}
	if cfg != nil {
		t.Errorf("에러 시 Config는 nil이어야 합니다, got %+v", cfg)
	}
}

func TestLoadConfig_InvalidTimeout(t *testing.T) {
	setEnvVars(t, map[string]string{
		"DISCORD_TOKEN": "test-token",
		"AGY_TIMEOUT":   "invalid-duration",
	})

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("잘못된 AGY_TIMEOUT 값에 대해 에러가 발생해야 합니다")
	}
	if cfg != nil {
		t.Errorf("에러 시 Config는 nil이어야 합니다, got %+v", cfg)
	}
}

func TestLoadConfig_InvalidLogLevel(t *testing.T) {
	setEnvVars(t, map[string]string{
		"DISCORD_TOKEN": "test-token",
		"LOG_LEVEL":     "verbose",
	})

	// 다른 환경변수 제거하여 기본값 사용
	os.Unsetenv("AGY_MODEL")
	os.Unsetenv("AGY_FALLBACK_MODEL")
	os.Unsetenv("AGY_TIMEOUT")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("잘못된 LOG_LEVEL 값에 대해 에러가 발생해야 합니다")
	}
	if cfg != nil {
		t.Errorf("에러 시 Config는 nil이어야 합니다, got %+v", cfg)
	}
}

func TestLoadConfig_EmptyTokenTreatedAsMissing(t *testing.T) {
	t.Setenv("DISCORD_TOKEN", "")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatal("빈 DISCORD_TOKEN에 대해 에러가 발생해야 합니다")
	}
	if cfg != nil {
		t.Errorf("에러 시 Config는 nil이어야 합니다, got %+v", cfg)
	}
}

func TestLoadConfig_VariousTimeoutFormats(t *testing.T) {
	tests := []struct {
		name     string
		timeout  string
		expected time.Duration
	}{
		{"seconds", "30s", 30 * time.Second},
		{"minutes", "5m", 5 * time.Minute},
		{"hours", "1h", time.Hour},
		{"combined", "1h30m", time.Hour + 30*time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnvVars(t, map[string]string{
				"DISCORD_TOKEN": "test-token",
				"AGY_TIMEOUT":   tt.timeout,
			})
			os.Unsetenv("AGY_MODEL")
			os.Unsetenv("AGY_FALLBACK_MODEL")
			os.Unsetenv("LOG_LEVEL")

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() 에러 발생: %v", err)
			}
			if cfg.AgyTimeout != tt.expected {
				t.Errorf("AgyTimeout = %v, want %v", cfg.AgyTimeout, tt.expected)
			}
		})
	}
}

func TestGetEnvOrDefault(t *testing.T) {
	t.Setenv("TEST_KEY", "custom-value")

	got := getEnvOrDefault("TEST_KEY", "default")
	if got != "custom-value" {
		t.Errorf("getEnvOrDefault() = %q, want %q", got, "custom-value")
	}

	os.Unsetenv("TEST_KEY_MISSING")
	got = getEnvOrDefault("TEST_KEY_MISSING", "fallback")
	if got != "fallback" {
		t.Errorf("getEnvOrDefault() = %q, want %q", got, "fallback")
	}
}

func TestIsValidLogLevel(t *testing.T) {
	validLevels := []string{"debug", "info", "warn", "error"}
	for _, level := range validLevels {
		if !isValidLogLevel(level) {
			t.Errorf("isValidLogLevel(%q) = false, want true", level)
		}
	}

	invalidLevels := []string{"verbose", "trace", "INFO", "Debug", ""}
	for _, level := range invalidLevels {
		if isValidLogLevel(level) {
			t.Errorf("isValidLogLevel(%q) = true, want false", level)
		}
	}
}
