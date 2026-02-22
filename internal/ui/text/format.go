package text

import (
	"fmt"
	"time"
)

// RelativeTime formats a time as relative: "3m ago", "1h ago", or "Jan 02 15:04" if > 24h.
func RelativeTime(t time.Time) string {
	d := time.Since(t)
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return "<1m ago"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return t.Format("Jan 02 15:04")
	}
}

// FormatTokens formats token counts: 12400 -> "12.4k", 1200000 -> "1.2M"
func FormatTokens(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FormatCost formats cost with 2 decimal places: 1.234 -> "$1.23"
func FormatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

// FormatPercent formats percentages: 87 -> "87%", 8.3 -> "8.3%"
func FormatPercent(pct float64) string {
	if pct < 10 {
		return fmt.Sprintf("%.1f%%", pct)
	}
	return fmt.Sprintf("%.0f%%", pct)
}

// FormatElapsed formats a duration as "3m", "1h12m", "25m" (no seconds unless < 1m).
func FormatElapsed(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
