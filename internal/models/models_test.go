package models

import (
	"testing"
)

func TestGetSelectOptions(t *testing.T) {
	options := GetSelectOptions()
	if len(options) != len(SupportedModels) {
		t.Errorf("GetSelectOptions() length = %d; want %d", len(options), len(SupportedModels))
	}

	for i, opt := range options {
		if opt.Label != SupportedModels[i].Label {
			t.Errorf("Option[%d] label = %q; want %q", i, opt.Label, SupportedModels[i].Label)
		}
		if opt.Value != SupportedModels[i].Name {
			t.Errorf("Option[%d] value = %q; want %q", i, opt.Value, SupportedModels[i].Name)
		}
	}
}

func TestGetPricing(t *testing.T) {
	tests := []struct {
		name          string
		modelName     string
		wantExist     bool
		wantName      string
		wantInputCost float64
	}{
		{
			name:          "Exact match Claude Opus",
			modelName:     "Claude Opus 4.6 (Thinking)",
			wantExist:     true,
			wantName:      "Claude Opus 4.6 (Thinking)",
			wantInputCost: 5.0,
		},
		{
			name:          "Partial match Gemini 3.5 Flash",
			modelName:     "Gemini 3.5 Flash (High)",
			wantExist:     true,
			wantName:      "Gemini 3.5 Flash (High)",
			wantInputCost: 1.5,
		},
		{
			name:          "Keyword match custom name",
			modelName:     "custom-Gemini 3.1 Pro-model",
			wantExist:     true,
			wantName:      "Gemini 3.1 Pro (High)",
			wantInputCost: 2.0,
		},
		{
			name:          "Legacy Claude Sonnet",
			modelName:     "Claude Sonnet 4.6",
			wantExist:     true,
			wantName:      "Claude Sonnet 4.6",
			wantInputCost: 3.0,
		},
		{
			name:      "Unknown model",
			modelName: "UltraGPT-9.0",
			wantExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing, ok := GetPricing(tt.modelName)
			if ok != tt.wantExist {
				t.Errorf("GetPricing(%q) ok = %v; want %v", tt.modelName, ok, tt.wantExist)
			}
			if ok {
				if pricing.Name != tt.wantName {
					t.Errorf("GetPricing(%q) Name = %q; want %q", tt.modelName, pricing.Name, tt.wantName)
				}
				if pricing.InputPriceUSD != tt.wantInputCost {
					t.Errorf("GetPricing(%q) InputPriceUSD = %v; want %v", tt.modelName, pricing.InputPriceUSD, tt.wantInputCost)
				}
			}
		})
	}
}
