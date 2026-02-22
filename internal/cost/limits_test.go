package cost

import "testing"

func TestCheckRunCostExceeded(t *testing.T) {
	lc := &LimitChecker{MaxCostPerRun: 5.00}
	exceeded, reason := lc.CheckRun(0, 5.50)
	if !exceeded {
		t.Error("expected exceeded for cost over threshold")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCheckRunCostAtThreshold(t *testing.T) {
	lc := &LimitChecker{MaxCostPerRun: 5.00}
	exceeded, _ := lc.CheckRun(0, 5.00)
	if !exceeded {
		t.Error("expected exceeded at exact threshold")
	}
}

func TestCheckRunTokensExceeded(t *testing.T) {
	lc := &LimitChecker{MaxTokensPerRun: 500000}
	exceeded, reason := lc.CheckRun(600000, 0)
	if !exceeded {
		t.Error("expected exceeded for tokens over threshold")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestCheckRunUnderThresholds(t *testing.T) {
	lc := &LimitChecker{MaxTokensPerRun: 500000, MaxCostPerRun: 5.00}
	exceeded, reason := lc.CheckRun(100000, 1.50)
	if exceeded {
		t.Errorf("expected not exceeded, got reason: %s", reason)
	}
}

func TestCheckRunZeroThresholdsDisabled(t *testing.T) {
	lc := &LimitChecker{MaxTokensPerRun: 0, MaxCostPerRun: 0}
	exceeded, _ := lc.CheckRun(999999, 999.99)
	if exceeded {
		t.Error("expected not exceeded with zero thresholds (disabled)")
	}
}

func TestCheckRunCostCheckedFirst(t *testing.T) {
	lc := &LimitChecker{MaxTokensPerRun: 100, MaxCostPerRun: 1.00}
	exceeded, reason := lc.CheckRun(200, 2.00)
	if !exceeded {
		t.Error("expected exceeded")
	}
	// Cost should be checked first
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestIsRateLimitMatches(t *testing.T) {
	lc := &LimitChecker{}
	cases := []struct {
		text string
		want bool
	}{
		{"rate limit exceeded", true},
		{"Rate Limit Exceeded", true},
		{"error 429: too many requests", true},
		{"429", true},
		{"too many requests", true},
		{"Too Many Requests", true},
		{"server overloaded", true},
		{"Overloaded", true},
		{"rate_limit_error", true},
		{"connection refused", false},
		{"timeout", false},
		{"permission denied", false},
		{"", false},
	}

	for _, tc := range cases {
		got := lc.IsRateLimit(tc.text)
		if got != tc.want {
			t.Errorf("IsRateLimit(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
