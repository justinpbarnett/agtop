package panels

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

const sampleDiff = `diff --git a/src/auth.ts b/src/auth.ts
index 1234567..abcdefg 100644
--- a/src/auth.ts
+++ a/src/auth.ts
@@ -1,5 +1,8 @@
 import express from 'express';
+import jwt from 'jsonwebtoken';

 const app = express();
-const port = 3000;
+const port = process.env.PORT || 3000;
+
+app.use(express.json());
diff --git a/src/routes.ts b/src/routes.ts
index 2345678..bcdefgh 100644
--- a/src/routes.ts
+++ a/src/routes.ts
@@ -10,3 +10,7 @@
 app.get('/health', (req, res) => {
   res.json({ status: 'ok' });
 });
+
+app.post('/login', (req, res) => {
+  res.json({ token: 'test' });
+});
`

const sampleStat = ` src/auth.ts   | 5 +++--
 src/routes.ts | 4 ++++
 2 files changed, 7 insertions(+), 2 deletions(-)
`

func TestDiffViewRenderStyledDiff(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 30)
	dv.SetDiff(sampleDiff, sampleStat)

	content := dv.Content()

	// Added lines should be present
	if !strings.Contains(content, "import jwt") {
		t.Error("expected added line content to be present")
	}
	// Removed lines should be present
	if !strings.Contains(content, "const port = 3000") {
		t.Error("expected removed line content to be present")
	}
	// Hunk markers should be present
	if !strings.Contains(content, "@@") {
		t.Error("expected hunk markers to be present")
	}
	// Stat summary should be present
	if !strings.Contains(content, "2 files changed") {
		t.Error("expected stat summary to be present")
	}
}

func TestDiffViewParseFileOffsets(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 30)
	dv.SetDiff(sampleDiff, "")

	if len(dv.fileOffsets) != 2 {
		t.Fatalf("expected 2 file offsets, got %d", len(dv.fileOffsets))
	}

	// First file header should be at the first line of the diff
	lines := strings.Split(sampleDiff, "\n")
	for i, offset := range dv.fileOffsets {
		if offset < 0 || offset >= len(lines) {
			t.Errorf("file offset %d out of range: %d", i, offset)
			continue
		}
		if !strings.HasPrefix(lines[offset], "diff --git") {
			t.Errorf("file offset %d (line %d) does not point to a diff header: %q",
				i, offset, lines[offset])
		}
	}
}

func TestDiffViewFileNavigation(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 10) // Small viewport to enable scrolling
	dv.SetDiff(sampleDiff, "")

	if len(dv.fileOffsets) < 2 {
		t.Fatal("need at least 2 file offsets for navigation test")
	}

	// Start at file 0
	if dv.currentFile != 0 {
		t.Fatalf("expected currentFile=0, got %d", dv.currentFile)
	}

	// Navigate to next file
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if dv.currentFile != 1 {
		t.Errorf("expected currentFile=1 after ], got %d", dv.currentFile)
	}

	// Navigate past last file should stay at last
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{']'}})
	if dv.currentFile != 1 {
		t.Errorf("expected currentFile=1 at end, got %d", dv.currentFile)
	}

	// Navigate back
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if dv.currentFile != 0 {
		t.Errorf("expected currentFile=0 after [, got %d", dv.currentFile)
	}

	// Navigate before first file should stay at first
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'['}})
	if dv.currentFile != 0 {
		t.Errorf("expected currentFile=0 at start, got %d", dv.currentFile)
	}
}

func TestDiffViewEmptyDiff(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetDiff("", "")

	content := dv.Content()
	if !strings.Contains(content, "No changes") {
		t.Error("expected 'No changes' placeholder for empty diff")
	}
}

func TestDiffViewError(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetError("branch not found")

	content := dv.Content()
	if !strings.Contains(content, "branch not found") {
		t.Error("expected error message in diff view content")
	}
}

func TestDiffViewLoading(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetLoading()

	content := dv.Content()
	if !strings.Contains(content, "Loading") {
		t.Error("expected 'Loading' placeholder in diff view content")
	}
}

func TestDiffViewGGJumpsToTop(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 5) // Small viewport
	dv.SetDiff(sampleDiff, sampleStat)

	// Scroll down first
	dv.viewport.SetYOffset(10)

	// First g press
	var cmd tea.Cmd
	dv, cmd = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !dv.gPending {
		t.Fatal("expected gPending after first g")
	}
	if cmd == nil {
		t.Fatal("expected timer cmd after first g")
	}

	// Second g press
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if dv.gPending {
		t.Error("expected gPending=false after gg")
	}
	if dv.viewport.YOffset != 0 {
		t.Errorf("expected viewport at top (offset 0), got %d", dv.viewport.YOffset)
	}
}

func TestDiffViewGTimerExpiry(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)

	// First g press
	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !dv.gPending {
		t.Fatal("expected gPending after first g")
	}

	// Timer expires
	dv, _ = dv.Update(DiffGTimerExpiredMsg{})
	if dv.gPending {
		t.Error("expected gPending cleared after timer expiry")
	}
}

func TestDiffViewGCapitalJumpsToBottom(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 5)
	dv.SetDiff(sampleDiff, sampleStat)

	dv, _ = dv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	// Viewport should be at the bottom
	if dv.viewport.YOffset == 0 && len(strings.Split(sampleDiff, "\n")) > 5 {
		t.Error("expected viewport to move towards bottom after G")
	}
}

func TestDiffViewSetEmpty(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetDiff(sampleDiff, sampleStat) // Set real content first
	dv.SetEmpty()                       // Then clear it

	content := dv.Content()
	if !strings.Contains(content, "No changes") {
		t.Error("expected placeholder after SetEmpty")
	}
	if len(dv.fileOffsets) != 0 {
		t.Error("expected fileOffsets cleared after SetEmpty")
	}
}

func TestDiffViewKeybindsWhenFocused(t *testing.T) {
	dv := NewDiffView()
	dv.SetFocused(true)
	dv.SetSize(80, 20)
	dv.SetDiff(sampleDiff, "")

	keybinds := dv.Keybinds()
	if len(keybinds) == 0 {
		t.Error("expected keybinds when focused")
	}

	// Should have file nav keybinds since there are 2 files
	hasFileNav := false
	for _, kb := range keybinds {
		if kb.Key == "]" {
			hasFileNav = true
		}
	}
	if !hasFileNav {
		t.Error("expected file navigation keybind for multi-file diff")
	}
}

func TestDiffViewKeybindsWhenUnfocused(t *testing.T) {
	dv := NewDiffView()
	dv.SetFocused(false)
	keybinds := dv.Keybinds()
	if len(keybinds) != 0 {
		t.Error("expected no keybinds when unfocused")
	}
}

func TestDiffViewFileOffsetsWithStat(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 30)
	dv.SetDiff(sampleDiff, sampleStat)

	if len(dv.fileOffsets) != 2 {
		t.Fatalf("expected 2 file offsets, got %d", len(dv.fileOffsets))
	}

	// Stat summary adds lines at the top of the viewport content.
	// File offsets must account for those extra lines.
	statLines := strings.Split(strings.TrimRight(sampleStat, "\n"), "\n")
	statLineCount := len(statLines) + 1 // +1 for trailing blank line

	rawLines := strings.Split(sampleDiff, "\n")
	for i, offset := range dv.fileOffsets {
		rawIdx := offset - statLineCount
		if rawIdx < 0 || rawIdx >= len(rawLines) {
			t.Errorf("file offset %d adjusted index out of range: %d", i, rawIdx)
			continue
		}
		if !strings.HasPrefix(rawLines[rawIdx], "diff --git") {
			t.Errorf("file offset %d (raw line %d) does not point to a diff header: %q",
				i, rawIdx, rawLines[rawIdx])
		}
	}
}

func TestDiffViewNoBranch(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetNoBranch()

	content := dv.Content()
	if !strings.Contains(content, "No branch") {
		t.Error("expected 'No branch' placeholder")
	}
}

func TestDiffViewWaiting(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 20)
	dv.SetWaiting()

	content := dv.Content()
	if !strings.Contains(content, "Waiting for worktree") {
		t.Error("expected 'Waiting for worktree' placeholder")
	}
}

// --- LogView tab system tests ---

func TestLogViewTabSwitching(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	// Default tab should be Log
	if lv.ActiveTab() != tabLog {
		t.Fatalf("expected initial tab to be tabLog, got %d", lv.ActiveTab())
	}

	// Press l to switch to Diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lv.ActiveTab() != tabDiff {
		t.Errorf("expected tabDiff after l, got %d", lv.ActiveTab())
	}

	// Press h to switch back to Log tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if lv.ActiveTab() != tabLog {
		t.Errorf("expected tabLog after h, got %d", lv.ActiveTab())
	}
}

func TestLogViewActiveTab(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	if lv.ActiveTab() != 0 {
		t.Errorf("expected ActiveTab()=0, got %d", lv.ActiveTab())
	}

	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lv.ActiveTab() != 1 {
		t.Errorf("expected ActiveTab()=1, got %d", lv.ActiveTab())
	}
}

func TestLogViewDiffIntegration(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)
	lv.SetDiff(sampleDiff, sampleStat)

	// Switch to Diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lv.ActiveTab() != tabDiff {
		t.Fatal("expected to be on Diff tab")
	}

	view := lv.View()
	// Should show diff content
	if !strings.Contains(view, "import jwt") {
		t.Error("expected diff content visible in Diff tab view")
	}
}

func TestLogViewShowsTabIndicator(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	view := lv.View()
	if !strings.Contains(view, "Log") {
		t.Error("expected 'Log' tab label in view")
	}
	if !strings.Contains(view, "Diff") {
		t.Error("expected 'Diff' tab label in view")
	}
}

func TestLogViewDiffLoading(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)
	lv.SetDiffLoading()

	// Switch to Diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	view := lv.View()
	if !strings.Contains(view, "Loading") {
		t.Error("expected loading state visible in Diff tab")
	}
}

func TestLogViewDiffError(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)
	lv.SetDiffError("branch deleted")

	// Switch to Diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	view := lv.View()
	if !strings.Contains(view, "branch deleted") {
		t.Error("expected error message visible in Diff tab")
	}
}

func TestLogViewConsumesKeysFalseOnDiffTab(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	// Switch to diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lv.ActiveTab() != tabDiff {
		t.Fatal("expected to be on diff tab")
	}

	if lv.ConsumesKeys() {
		t.Error("expected ConsumesKeys()=false on diff tab")
	}
}

func TestLogViewSetRunResetsTab(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	// Switch to diff tab
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if lv.ActiveTab() != tabDiff {
		t.Fatal("expected to be on diff tab")
	}

	// SetRun should reset to log tab
	lv.SetRun("run-1", "build", "feat/x", nil, true)
	if lv.ActiveTab() != tabLog {
		t.Errorf("expected tab reset to tabLog after SetRun, got %d", lv.ActiveTab())
	}
}
