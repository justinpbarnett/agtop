package panels

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

const teatestDiff = `diff --git a/main.go b/main.go
index abc1234..def5678 100644
--- a/main.go
+++ b/main.go
@@ -10,6 +10,8 @@ func main() {
     fmt.Println("hello")
+    fmt.Println("world")
+    fmt.Println("!")
     os.Exit(0)
-    // old comment
 }
diff --git a/util.go b/util.go
index 1111111..2222222 100644
--- a/util.go
+++ b/util.go
@@ -1,3 +1,4 @@
 package main
+import "strings"
 func helper() {}`

const teatestDiffStat = ` main.go | 3 ++-
 util.go | 1 +
 2 files changed, 3 insertions(+), 1 deletion(-)`

func TestDiffViewWithDiffSnapshot(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 25)
	dv.SetFocused(true)
	dv.SetDiff(teatestDiff, teatestDiffStat)

	tm := teatest.NewTestModel(t, wrapDiffView(&dv), teatest.WithInitialTermSize(80, 25))
	waitForContains(t, tm, "main.go")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(dv.Content()))
}

func TestDiffViewLoadingSnapshot(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 25)
	dv.SetFocused(true)
	dv.SetLoading()

	tm := teatest.NewTestModel(t, wrapDiffView(&dv), teatest.WithInitialTermSize(80, 25))
	waitForContains(t, tm, "Loading")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(dv.Content()))
}

func TestDiffViewEmptySnapshot(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 25)
	dv.SetFocused(true)
	dv.SetEmpty()

	tm := teatest.NewTestModel(t, wrapDiffView(&dv), teatest.WithInitialTermSize(80, 25))
	waitForContains(t, tm, "No changes")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(dv.Content()))
}

func TestDiffViewFileNavigationFlow(t *testing.T) {
	dv := NewDiffView()
	dv.SetSize(80, 25)
	dv.SetFocused(true)
	dv.SetDiff(teatestDiff, teatestDiffStat)

	tm := teatest.NewTestModel(t, wrapDiffView(&dv), teatest.WithInitialTermSize(80, 25))
	waitForContains(t, tm, "main.go")

	if len(dv.fileOffsets) < 2 {
		t.Fatalf("expected at least 2 file offsets, got %d", len(dv.fileOffsets))
	}

	// Navigate to next file
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	time.Sleep(100 * time.Millisecond)

	if dv.currentFile != 1 {
		t.Errorf("expected currentFile 1 after ], got %d", dv.currentFile)
	}

	// Navigate back
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	time.Sleep(100 * time.Millisecond)

	if dv.currentFile != 0 {
		t.Errorf("expected currentFile 0 after [, got %d", dv.currentFile)
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}
