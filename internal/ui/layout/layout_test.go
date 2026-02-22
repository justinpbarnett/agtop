package layout

import "testing"

func TestTooSmallWidth(t *testing.T) {
	l := Calculate(79, 24)
	if !l.TooSmall {
		t.Error("expected TooSmall for width 79")
	}
}

func TestTooSmallHeight(t *testing.T) {
	l := Calculate(80, 23)
	if !l.TooSmall {
		t.Error("expected TooSmall for height 23")
	}
}

func TestMinimumViable(t *testing.T) {
	l := Calculate(80, 24)
	if l.TooSmall {
		t.Error("80x24 should not be too small")
	}
	// Left column heights sum to usable height
	if l.RunListHeight+l.DetailHeight+1 != 24 {
		t.Errorf("left col height mismatch: runList(%d) + detail(%d) + status(1) = %d, want 24",
			l.RunListHeight, l.DetailHeight, l.RunListHeight+l.DetailHeight+1)
	}
	// Left + right widths sum to terminal width
	if l.RunListWidth+l.LogViewWidth != 80 {
		t.Errorf("width mismatch: left(%d) + right(%d) = %d, want 80",
			l.RunListWidth, l.LogViewWidth, l.RunListWidth+l.LogViewWidth)
	}
}

func TestStandard120x40(t *testing.T) {
	l := Calculate(120, 40)
	if l.TooSmall {
		t.Error("120x40 should not be too small")
	}

	usable := 39 // 40 - 1 status bar

	// Left column heights sum to usable height
	if l.RunListHeight+l.DetailHeight != usable {
		t.Errorf("left col height: runList(%d) + detail(%d) = %d, want %d",
			l.RunListHeight, l.DetailHeight, l.RunListHeight+l.DetailHeight, usable)
	}
	// LogView gets full usable height
	if l.LogViewHeight != usable {
		t.Errorf("logView height: got %d, want %d", l.LogViewHeight, usable)
	}
	// Left + right widths sum to terminal width
	if l.RunListWidth+l.LogViewWidth != 120 {
		t.Errorf("width: left(%d) + right(%d) = %d, want 120",
			l.RunListWidth, l.LogViewWidth, l.RunListWidth+l.LogViewWidth)
	}
	// Detail and RunList share the same left column width
	if l.DetailWidth != l.RunListWidth {
		t.Errorf("detail width (%d) should equal runList width (%d)", l.DetailWidth, l.RunListWidth)
	}
	if l.StatusBarWidth != 120 {
		t.Errorf("status bar width: got %d, want 120", l.StatusBarWidth)
	}
}

func TestDetailSameWidthAsRunList(t *testing.T) {
	l := Calculate(100, 30)
	if l.DetailWidth != l.RunListWidth {
		t.Errorf("detail width (%d) should equal runList width (%d)", l.DetailWidth, l.RunListWidth)
	}
}

func TestLogViewFullHeight(t *testing.T) {
	l := Calculate(100, 30)
	usable := 29 // 30 - 1 status bar
	if l.LogViewHeight != usable {
		t.Errorf("logView height: got %d, want %d", l.LogViewHeight, usable)
	}
}
