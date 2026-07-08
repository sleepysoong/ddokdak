package handler

import (
	"testing"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "짧은 문자열은 그대로 반환",
			input:    "짧은 텍스트",
			maxLen:   20,
			expected: "짧은 텍스트",
		},
		{
			name:     "최대 길이와 같은 문자열",
			input:    "정확히 5자입니다",
			maxLen:   8,
			expected: "정확히 5자입니다",
		},
		{
			name:     "긴 문자열 자르기",
			input:    "이것은 매우 긴 텍스트 메시지입니다",
			maxLen:   10,
			expected: "이것은 매우 ....",
		},
		{
			name:     "빈 문자열",
			input:    "",
			maxLen:   10,
			expected: "",
		},
		{
			name:     "영문 문자열",
			input:    "Hello, World! This is a test message.",
			maxLen:   15,
			expected: "Hello, World...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			runeCount := len([]rune(result))
			if runeCount > tt.maxLen {
				t.Errorf("truncateString(%q, %d) = %q (길이: %d), 최대 길이 초과",
					tt.input, tt.maxLen, result, runeCount)
			}
		})
	}
}

func TestSplitMessage(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		maxLen         int
		expectedParts  int
		checkFirstPart string
	}{
		{
			name:           "짧은 메시지는 분할하지 않음",
			content:        "짧은 메시지",
			maxLen:         100,
			expectedParts:  1,
			checkFirstPart: "짧은 메시지",
		},
		{
			name:          "긴 메시지 분할",
			content:       "가나다라마바사아자차카타파하가나다라마바사아자차카타파하가나다라마바사아자차카타파하",
			maxLen:        30,
			expectedParts: 2,
		},
		{
			name:           "빈 메시지",
			content:        "",
			maxLen:         100,
			expectedParts:  1,
			checkFirstPart: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := splitMessage(tt.content, tt.maxLen)
			if len(parts) < 1 {
				t.Errorf("splitMessage() 반환된 부분이 없습니다")
				return
			}
			if tt.checkFirstPart != "" && parts[0] != tt.checkFirstPart {
				t.Errorf("splitMessage() 첫 번째 부분 = %q, 기대값 %q",
					parts[0], tt.checkFirstPart)
			}
			for i, part := range parts {
				if len(part) > tt.maxLen {
					t.Errorf("splitMessage() 부분 %d 길이(%d)가 최대 길이(%d)를 초과",
						i, len(part), tt.maxLen)
				}
			}
		})
	}
}
