package llm

import "strings"

func tierForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "haiku"), strings.Contains(m, "flash"), strings.Contains(m, "mini"), strings.Contains(m, "nano"):
		return TierFast
	case strings.Contains(m, "opus"), strings.Contains(m, "pro"), strings.Contains(m, "4o"):
		return TierPowerful
	default:
		return TierBalanced
	}
}

func inputCostPer1K(model string) float64 {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "haiku"):
		return 0.0008
	case strings.Contains(m, "sonnet"):
		return 0.003
	case strings.Contains(m, "opus"):
		return 0.015
	case strings.Contains(m, "flash"):
		return 0.00035
	case strings.Contains(m, "gemini") && strings.Contains(m, "pro"):
		return 0.00125
	case strings.Contains(m, "mini"):
		return 0.00015
	default:
		return 0.005
	}
}

func outputCostPer1K(model string) float64 {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "haiku"):
		return 0.004
	case strings.Contains(m, "sonnet"):
		return 0.015
	case strings.Contains(m, "opus"):
		return 0.075
	case strings.Contains(m, "flash"):
		return 0.00105
	case strings.Contains(m, "gemini") && strings.Contains(m, "pro"):
		return 0.005
	case strings.Contains(m, "mini"):
		return 0.0006
	default:
		return 0.015
	}
}
