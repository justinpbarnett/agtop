package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPanelWidthsInFullLayout(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	// Render each panel individually and check widths
	runListView := a.runList.View()
	logViewView := a.logView.View()
	detailView := a.detail.View()

	checkAllLines := func(name string, view string, wantWidth, wantHeight int) {
		lines := strings.Split(view, "\n")
		if len(lines) != wantHeight {
			t.Errorf("%s: line count=%d, want=%d", name, len(lines), wantHeight)
		}
		for i, line := range lines {
			w := lipgloss.Width(line)
			if w != wantWidth {
				t.Errorf("%s line %d: width=%d, want=%d (off by %+d) content_bytes=%d",
					name, i, w, wantWidth, w-wantWidth, len(line))
			}
		}
	}

	checkAllLines("RunList", runListView, a.layout.RunListWidth, a.layout.RunListHeight)
	checkAllLines("LogView", logViewView, a.layout.LogViewWidth, a.layout.LogViewHeight)
	checkAllLines("Detail", detailView, a.layout.DetailWidth, a.layout.DetailHeight)

	// Check JoinHorizontal
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, runListView, logViewView)
	topLines := strings.Split(topRow, "\n")
	totalTopWidth := a.layout.RunListWidth + a.layout.LogViewWidth
	if totalTopWidth != 120 {
		t.Errorf("total top width: %d, want 120", totalTopWidth)
	}
	for i, line := range topLines {
		w := lipgloss.Width(line)
		if w != totalTopWidth {
			t.Errorf("joined top line %d: width=%d, want=%d (off by %+d)",
				i, w, totalTopWidth, w-totalTopWidth)
		}
	}

	// Full layout
	statusBarView := a.statusBar.View()
	fullLayout := lipgloss.JoinVertical(lipgloss.Left, topRow, detailView, statusBarView)
	fullLines := strings.Split(fullLayout, "\n")
	t.Logf("Full layout: %d lines (term height=%d)", len(fullLines), 40)
	for i, line := range fullLines {
		w := lipgloss.Width(line)
		if w > 120 {
			t.Errorf("full layout line %d: width=%d, exceeds terminal width 120 (off by %+d)",
				i, w, w-120)
		}
	}
}
