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

func TestWrapTextEmpty(t *testing.T) {
	got := WrapText("", 20)
	if len(got) != 1 || got[0] != "" {
		t.Errorf("WrapText empty: got %v", got)
	}
}

func TestWrapTextFitsOnOneLine(t *testing.T) {
	got := WrapText("hello world", 20)
	if len(got) != 1 || got[0] != "hello world" {
		t.Errorf("WrapText fits: got %v", got)
	}
}

func TestWrapTextExactWidth(t *testing.T) {
	got := WrapText("hello", 5)
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("WrapText exact: got %v", got)
	}
}

func TestWrapTextBasicWrap(t *testing.T) {
	got := WrapText("hello world", 7)
	// "hello" (5) fits, "world" (5) needs new line
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Errorf("WrapText basic wrap: got %v", got)
	}
}

func TestWrapTextMultipleLines(t *testing.T) {
	got := WrapText("one two three four five", 9)
	// "one two" (7), "three" (5), "four five" (9)
	if len(got) != 3 {
		t.Errorf("WrapText multiple lines: got %d lines: %v", len(got), got)
	}
}

func TestWrapTextRespectsExistingNewlines(t *testing.T) {
	got := WrapText("line one\nline two", 20)
	if len(got) != 2 || got[0] != "line one" || got[1] != "line two" {
		t.Errorf("WrapText newlines: got %v", got)
	}
}

func TestWrapTextZeroWidth(t *testing.T) {
	got := WrapText("hello", 0)
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("WrapText zero width: got %v", got)
	}
}

func TestWrapTextLongWordTruncated(t *testing.T) {
	// A single word longer than width is truncated with …
	got := WrapText("averylongwordthatexceedswidth", 10)
	if len(got) != 1 {
		t.Errorf("WrapText long word: expected 1 line, got %d: %v", len(got), got)
	}
	w := ansi.StringWidth(got[0])
	if w > 10 {
		t.Errorf("WrapText long word: visual width=%d, want <=10", w)
	}
}
