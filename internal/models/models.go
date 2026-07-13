package models

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// ModelInfo는 AI 모델의 메타데이터 및 가격 정보를 정의합니다.
type ModelInfo struct {
	Name           string
	Label          string
	InputPriceUSD  float64
	OutputPriceUSD float64
}

// SupportedModels는 시스템에서 공식적으로 지원하는 AI 모델 목록입니다.
var SupportedModels = []ModelInfo{
	{
		Name:           "Claude Opus 4.6 (Thinking)",
		Label:          "Claude Opus 4.6 (Thinking)",
		InputPriceUSD:  5.0,
		OutputPriceUSD: 25.0,
	},
	{
		Name:           "Gemini 3.1 Pro (High)",
		Label:          "Gemini 3.1 Pro (High)",
		InputPriceUSD:  2.0,
		OutputPriceUSD: 12.0,
	},
	{
		Name:           "Gemini 3.5 Flash (High)",
		Label:          "Gemini 3.5 Flash (High)",
		InputPriceUSD:  1.5,
		OutputPriceUSD: 9.0,
	},
	{
		Name:           "Gemini 3.5 Flash (Medium)",
		Label:          "Gemini 3.5 Flash (Medium)",
		InputPriceUSD:  1.5,
		OutputPriceUSD: 9.0,
	},
}

// GetSelectOptions는 디스코드 SelectMenu에 사용될 옵션 슬라이스를 생성합니다.
func GetSelectOptions() []discordgo.SelectMenuOption {
	options := make([]discordgo.SelectMenuOption, len(SupportedModels))
	for i, m := range SupportedModels {
		options[i] = discordgo.SelectMenuOption{
			Label: m.Label,
			Value: m.Name,
		}
	}
	return options
}

// GetPricing은 모델명에 해당하는 가격 정보를 반환합니다.
func GetPricing(modelName string) (ModelInfo, bool) {
	// 1. 완전 일치 매칭
	for _, m := range SupportedModels {
		if m.Name == modelName {
			return m, true
		}
	}

	// 2. 키워드 기반 유연한 부분 매칭
	if strings.Contains(modelName, "Gemini 3.5 Flash") {
		for _, m := range SupportedModels {
			if strings.Contains(m.Name, "Gemini 3.5 Flash") {
				return m, true
			}
		}
	}
	if strings.Contains(modelName, "Gemini 3.1 Pro") {
		for _, m := range SupportedModels {
			if strings.Contains(m.Name, "Gemini 3.1 Pro") {
				return m, true
			}
		}
	}
	if strings.Contains(modelName, "Claude Opus") {
		for _, m := range SupportedModels {
			if strings.Contains(m.Name, "Claude Opus") {
				return m, true
			}
		}
	}

	// 레거시/대체 모델 대응
	if strings.Contains(modelName, "Claude Sonnet") {
		return ModelInfo{
			Name:           "Claude Sonnet 4.6",
			InputPriceUSD:  3.0,
			OutputPriceUSD: 15.0,
		}, true
	}

	return ModelInfo{}, false
}

// CostResult는 토큰 비용 계산 결과를 담는 구조체입니다.
type CostResult struct {
	InputCostUSD  float64
	OutputCostUSD float64
	TotalCostUSD  float64
}

// CalculateCost는 입력/출력 토큰 수를 기반으로 USD 비용을 계산합니다.
func (m ModelInfo) CalculateCost(inputTokens, outputTokens int64) CostResult {
	inputCost := (float64(inputTokens) / 1_000_000) * m.InputPriceUSD
	outputCost := (float64(outputTokens) / 1_000_000) * m.OutputPriceUSD
	return CostResult{
		InputCostUSD:  inputCost,
		OutputCostUSD: outputCost,
		TotalCostUSD:  inputCost + outputCost,
	}
}

