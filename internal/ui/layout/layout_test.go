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
	// Verify dimensions sum correctly
	if l.RunListHeight+l.DetailHeight+1 != 24 {
		t.Errorf("height mismatch: top(%d) + bottom(%d) + status(1) = %d, want 24",
			l.RunListHeight, l.DetailHeight, l.RunListHeight+l.DetailHeight+1)
	}
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

	// Verify all dimensions sum correctly
	if l.RunListHeight+l.DetailHeight+1 != 40 {
		t.Errorf("height: top(%d) + bottom(%d) + 1 = %d, want 40",
			l.RunListHeight, l.DetailHeight, l.RunListHeight+l.DetailHeight+1)
	}
	if l.RunListWidth+l.LogViewWidth != 120 {
		t.Errorf("width: left(%d) + right(%d) = %d, want 120",
			l.RunListWidth, l.LogViewWidth, l.RunListWidth+l.LogViewWidth)
	}
	if l.DetailWidth != 120 {
		t.Errorf("detail width: got %d, want 120", l.DetailWidth)
	}
	if l.StatusBarWidth != 120 {
		t.Errorf("status bar width: got %d, want 120", l.StatusBarWidth)
	}

	// Top row should be ~65% of usable height (39)
	usable := 39.0
	expectedTopHeight := int(usable * 0.65)
	if l.RunListHeight != expectedTopHeight {
		t.Errorf("top row height: got %d, want %d", l.RunListHeight, expectedTopHeight)
	}
	if l.LogViewHeight != l.RunListHeight {
		t.Errorf("log view height should equal run list height")
	}
}

func TestDetailFullWidth(t *testing.T) {
	l := Calculate(100, 30)
	if l.DetailWidth != 100 {
		t.Errorf("detail width: got %d, want 100", l.DetailWidth)
	}
}
