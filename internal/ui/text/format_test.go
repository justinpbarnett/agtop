package text

import (
	"testing"
	"time"
)

func TestRelativeTimeSeconds(t *testing.T) {
	got := RelativeTime(time.Now().Add(-30 * time.Second))
	if got != "<1m ago" {
		t.Errorf("RelativeTime seconds: got %q, want %q", got, "<1m ago")
	}
}

func TestRelativeTimeMinutes(t *testing.T) {
	got := RelativeTime(time.Now().Add(-5 * time.Minute))
	if got != "5m ago" {
		t.Errorf("RelativeTime minutes: got %q, want %q", got, "5m ago")
	}
}

func TestRelativeTimeHours(t *testing.T) {
	got := RelativeTime(time.Now().Add(-3 * time.Hour))
	if got != "3h ago" {
		t.Errorf("RelativeTime hours: got %q, want %q", got, "3h ago")
	}
}

func TestRelativeTimeOld(t *testing.T) {
	old := time.Now().Add(-48 * time.Hour)
	got := RelativeTime(old)
	expected := old.Format("Jan 02 15:04")
	if got != expected {
		t.Errorf("RelativeTime old: got %q, want %q", got, expected)
	}
}

func TestFormatTokensZero(t *testing.T) {
	if got := FormatTokens(0); got != "0" {
		t.Errorf("FormatTokens 0: got %q", got)
	}
}

func TestFormatTokensSmall(t *testing.T) {
	if got := FormatTokens(500); got != "500" {
		t.Errorf("FormatTokens 500: got %q", got)
	}
}

func TestFormatTokensKilo(t *testing.T) {
	if got := FormatTokens(1000); got != "1.0k" {
		t.Errorf("FormatTokens 1000: got %q", got)
	}
}

func TestFormatTokensKiloLarge(t *testing.T) {
	if got := FormatTokens(12400); got != "12.4k" {
		t.Errorf("FormatTokens 12400: got %q, want %q", got, "12.4k")
	}
}

func TestFormatTokensMega(t *testing.T) {
	if got := FormatTokens(1200000); got != "1.2M" {
		t.Errorf("FormatTokens 1200000: got %q, want %q", got, "1.2M")
	}
}

func TestFormatCostZero(t *testing.T) {
	if got := FormatCost(0); got != "$0.00" {
		t.Errorf("FormatCost 0: got %q", got)
	}
}

func TestFormatCostSmall(t *testing.T) {
	if got := FormatCost(0.12); got != "$0.12" {
		t.Errorf("FormatCost 0.12: got %q", got)
	}
}

func TestFormatCostRound(t *testing.T) {
	if got := FormatCost(1.234); got != "$1.23" {
		t.Errorf("FormatCost 1.234: got %q", got)
	}
}

func TestFormatCostLarge(t *testing.T) {
	if got := FormatCost(99.999); got != "$100.00" {
		t.Errorf("FormatCost 99.999: got %q", got)
	}
}

func TestFormatPercentSmall(t *testing.T) {
	if got := FormatPercent(8.3); got != "8.3%" {
		t.Errorf("FormatPercent 8.3: got %q", got)
	}
}

func TestFormatPercentLarge(t *testing.T) {
	if got := FormatPercent(87); got != "87%" {
		t.Errorf("FormatPercent 87: got %q", got)
	}
}

func TestFormatElapsedSeconds(t *testing.T) {
	if got := FormatElapsed(30 * time.Second); got != "30s" {
		t.Errorf("FormatElapsed 30s: got %q", got)
	}
}

func TestFormatElapsedMinutes(t *testing.T) {
	if got := FormatElapsed(3 * time.Minute); got != "3m" {
		t.Errorf("FormatElapsed 3m: got %q", got)
	}
}

func TestFormatElapsedHoursMinutes(t *testing.T) {
	if got := FormatElapsed(72 * time.Minute); got != "1h12m" {
		t.Errorf("FormatElapsed 1h12m: got %q, want %q", got, "1h12m")
	}
}
