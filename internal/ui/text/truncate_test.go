package text

import (
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestTruncateEmpty(t *testing.T) {
	if got := Truncate("", 10); got != "" {
		t.Errorf("Truncate empty: got %q", got)
	}
}

func TestTruncateWithinLimit(t *testing.T) {
	if got := Truncate("hello", 10); got != "hello" {
		t.Errorf("Truncate within limit: got %q", got)
	}
}

func TestTruncateExactLimit(t *testing.T) {
	if got := Truncate("hello", 5); got != "hello" {
		t.Errorf("Truncate exact limit: got %q", got)
	}
}

func TestTruncateOverLimit(t *testing.T) {
	got := Truncate("hello world", 8)
	if got != "hello w…" {
		t.Errorf("Truncate over limit: got %q, want %q", got, "hello w…")
	}
}

func TestTruncateZeroWidth(t *testing.T) {
	if got := Truncate("hello", 0); got != "" {
		t.Errorf("Truncate zero width: got %q", got)
	}
}

func TestTruncateWidthOne(t *testing.T) {
	got := Truncate("hello", 1)
	if got != "…" {
		t.Errorf("Truncate width 1: got %q, want %q", got, "…")
	}
}

func TestTruncateMultibyte(t *testing.T) {
	// CJK characters are width 2
	got := Truncate("日本語テスト", 7)
	// 3 CJK chars (width 6) + ellipsis (width 1) = 7
	if got != "日本語…" {
		t.Errorf("Truncate multibyte: got %q, want %q", got, "日本語…")
	}
}

func TestTruncateANSI(t *testing.T) {
	// Styled text: ANSI codes should be ignored for width calculation
	styled := "\033[38;2;125;207;255m●\033[0m hello world"
	got := Truncate(styled, 8)
	// Visual: "● hello " = 8 chars, so it should show "● hell…" (7 + ellipsis)
	w := ansi.StringWidth(got)
	if w != 8 {
		t.Errorf("Truncate ANSI: visual width=%d, want 8, got=%q", w, got)
	}
}

func TestTruncateANSIWithinLimit(t *testing.T) {
	// Short styled text that fits within limit
	styled := "\033[38;2;125;207;255m●\033[0m"
	got := Truncate(styled, 10)
	// Visual width is 1 ("●"), fits within 10
	if got != styled {
		t.Errorf("Truncate ANSI within limit: got %q, want %q", got, styled)
	}
}

func TestTruncatePreservesANSISequences(t *testing.T) {
	// Ensure truncation doesn't break ANSI escape sequences
	styled := "\033[38;2;125;207;255mhello world\033[0m"
	got := Truncate(styled, 8)
	w := ansi.StringWidth(got)
	if w != 8 {
		t.Errorf("Truncate preserves ANSI: visual width=%d, want 8", w)
	}
}

func TestPadRightShorter(t *testing.T) {
	got := PadRight("hi", 5)
	if got != "hi   " {
		t.Errorf("PadRight shorter: got %q, want %q", got, "hi   ")
	}
}

func TestPadRightExact(t *testing.T) {
	got := PadRight("hello", 5)
	if got != "hello" {
		t.Errorf("PadRight exact: got %q, want %q", got, "hello")
	}
}

func TestPadRightLonger(t *testing.T) {
	got := PadRight("hello world", 5)
	if got != "hello world" {
		t.Errorf("PadRight longer: got %q, want %q", got, "hello world")
	}
}

func TestPadRightANSI(t *testing.T) {
	// Styled text should be padded based on visual width, not byte length
	styled := "\033[38;2;125;207;255m●\033[0m"
	got := PadRight(styled, 5)
	w := ansi.StringWidth(got)
	if w != 5 {
		t.Errorf("PadRight ANSI: visual width=%d, want 5", w)
	}
}
