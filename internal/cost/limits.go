package cost

import (
	"fmt"
	"strings"
)

// LimitChecker enforces per-run token and cost thresholds
// and detects rate limit errors.
type LimitChecker struct {
	MaxTokensPerRun int
	MaxCostPerRun   float64
}

// CheckRun returns whether a run has exceeded its configured thresholds.
// A zero threshold means the check is disabled for that dimension.
func (lc *LimitChecker) CheckRun(runTokens int, runCost float64) (exceeded bool, reason string) {
	if lc.MaxCostPerRun > 0 && runCost >= lc.MaxCostPerRun {
		return true, fmt.Sprintf("cost threshold exceeded ($%.2f >= $%.2f)", runCost, lc.MaxCostPerRun)
	}
	if lc.MaxTokensPerRun > 0 && runTokens >= lc.MaxTokensPerRun {
		return true, fmt.Sprintf("token threshold exceeded (%d >= %d)", runTokens, lc.MaxTokensPerRun)
	}
	return false, ""
}

// IsRateLimit returns true if the error text indicates an API rate limit.
func (lc *LimitChecker) IsRateLimit(errorText string) bool {
	lower := strings.ToLower(errorText)
	patterns := []string{
		"rate limit",
		"rate_limit",
		"429",
		"too many requests",
		"overloaded",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}
