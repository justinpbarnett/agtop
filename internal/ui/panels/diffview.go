package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

const diffGTimeout = 300 * time.Millisecond

type DiffGTimerExpiredMsg struct{}

type DiffView struct {
	viewport    viewport.Model
	width       int
	height      int
	rawDiff     string
	diffStat    string
	fileOffsets []int
	currentFile int
	loading     bool
	errMsg      string
	emptyMsg    string
	focused     bool
	gPending    bool

	// Copy mode state
	copyMode   bool
	copyAnchor int
	copyCursor int

	// Mouse selection state (character-level)
	mouseSelecting   bool
	mouseAnchorLine  int
	mouseAnchorCol   int
	mouseCurrentLine int
	mouseCurrentCol  int
}

func NewDiffView() DiffView {
	vp := viewport.New(0, 0)
	return DiffView{viewport: vp, emptyMsg: "No changes on branch"}
}

func (d *DiffView) SetDiff(diff, stat string) {
	d.rawDiff = diff
	d.diffStat = stat
	d.loading = false
	d.errMsg = ""
	if strings.TrimSpace(diff) == "" {
		d.emptyMsg = "No changes on branch"
	} else {
		d.emptyMsg = ""
	}
	d.currentFile = 0
	d.parseFileOffsets()
	d.refreshContent()
}

func (d *DiffView) SetLoading() {
	d.loading = true
	d.errMsg = ""
	d.emptyMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
	d.refreshContent()
}

func (d *DiffView) SetError(err string) {
	d.errMsg = err
	d.loading = false
	d.emptyMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
	d.refreshContent()
}

func (d *DiffView) SetEmpty() {
	d.emptyMsg = "No changes on branch"
	d.loading = false
	d.errMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
	d.refreshContent()
}

func (d *DiffView) SetNoBranch() {
	d.emptyMsg = "No branch — diff unavailable"
	d.loading = false
	d.errMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
	d.refreshContent()
}

func (d *DiffView) SetWaiting() {
	d.emptyMsg = "Waiting for worktree..."
	d.loading = false
	d.errMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
	d.refreshContent()
}

func (d *DiffView) SetSize(w, h int) {
	d.width = w
	d.height = h
	if w > 0 && h > 0 {
		d.viewport.Width = w
		vpH := h
		if d.copyMode {
			vpH-- // Reserve row for copy mode status
		}
		if vpH < 0 {
			vpH = 0
		}
		d.viewport.Height = vpH
	}
	d.refreshContent()
}

func (d *DiffView) SetFocused(focused bool) {
	d.focused = focused
}

func (d DiffView) Update(msg tea.Msg) (DiffView, tea.Cmd) {
	switch msg := msg.(type) {
	case DiffGTimerExpiredMsg:
		d.gPending = false
		return d, nil
	case tea.KeyMsg:
		if d.copyMode {
			return d.updateCopyMode(msg)
		}

		switch msg.String() {
		case "y":
			d.enterCopyMode()
			return d, nil
		case "G":
			d.viewport.GotoBottom()
			return d, nil
		case "g":
			if d.gPending {
				d.gPending = false
				d.viewport.GotoTop()
				return d, nil
			}
			d.gPending = true
			return d, tea.Tick(diffGTimeout, func(time.Time) tea.Msg {
				return DiffGTimerExpiredMsg{}
			})
		case "j", "down":
			d.viewport.SetYOffset(d.viewport.YOffset + 1)
			return d, nil
		case "k", "up":
			offset := d.viewport.YOffset - 1
			if offset < 0 {
				offset = 0
			}
			d.viewport.SetYOffset(offset)
			return d, nil
		case "]":
			d.nextFile()
			return d, nil
		case "[":
			d.prevFile()
			return d, nil
		}
	}

	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

// ConsumesKeys reports whether the diff view is in copy mode.
func (d DiffView) ConsumesKeys() bool {
	return d.copyMode
}

// Content returns the rendered content string for embedding in another panel.
func (d DiffView) Content() string {
	content := d.viewport.View()
	if d.copyMode {
		selStart, selEnd := d.copySelectionRange()
		count := selEnd - selStart + 1
		status := styles.TextSecondaryStyle.Render(
			fmt.Sprintf("  VISUAL: %d line(s) selected", count),
		) + styles.TextDimStyle.Render(" (y yank, Esc cancel)")
		content += "\n" + status
	}
	return content
}

// Keybinds returns the keybinds to show when this view is active.
func (d DiffView) Keybinds() []border.Keybind {
	if !d.focused {
		return nil
	}
	if d.copyMode {
		return []border.Keybind{
			{Key: "y", Label: "ank"},
			{Key: "j", Label: "/k select"},
			{Key: "Esc", Label: " cancel"},
		}
	}
	binds := []border.Keybind{
		{Key: "y", Label: "ank/copy"},
		{Key: "j", Label: "/k scroll"},
		{Key: "G", Label: "/gg jump"},
	}
	if len(d.fileOffsets) > 1 {
		binds = append(binds, border.Keybind{Key: "]", Label: "/[ file"})
	}
	return binds
}

func (d *DiffView) nextFile() {
	if len(d.fileOffsets) == 0 {
		return
	}
	if d.currentFile < len(d.fileOffsets)-1 {
		d.currentFile++
		d.viewport.SetYOffset(d.fileOffsets[d.currentFile])
	}
}

func (d *DiffView) prevFile() {
	if len(d.fileOffsets) == 0 {
		return
	}
	if d.currentFile > 0 {
		d.currentFile--
		d.viewport.SetYOffset(d.fileOffsets[d.currentFile])
	}
}

func (d *DiffView) enterCopyMode() {
	lines := d.rawLines()
	if len(lines) == 0 {
		return
	}
	centerLine := d.viewport.YOffset + d.viewport.Height/2
	if centerLine >= len(lines) {
		centerLine = len(lines) - 1
	}
	if centerLine < 0 {
		centerLine = 0
	}
	d.copyMode = true
	d.mouseSelecting = false
	d.copyAnchor = centerLine
	d.copyCursor = centerLine
	d.refreshContent()
}

func (d *DiffView) updateCopyMode(msg tea.KeyMsg) (DiffView, tea.Cmd) {
	lineCount := len(d.rawLines())

	switch msg.String() {
	case "esc":
		d.copyMode = false
		d.refreshContent()
		return *d, nil
	case "y":
		text := d.yankSelection()
		d.copyMode = false
		d.refreshContent()
		if text != "" {
			return *d, func() tea.Msg { return YankMsg{Text: text} }
		}
		return *d, nil
	case "j", "down":
		if d.copyCursor < lineCount-1 {
			d.copyCursor++
			if d.copyCursor >= d.viewport.YOffset+d.viewport.Height {
				d.viewport.SetYOffset(d.copyCursor - d.viewport.Height + 1)
			}
			d.refreshContent()
		}
		return *d, nil
	case "k", "up":
		if d.copyCursor > 0 {
			d.copyCursor--
			if d.copyCursor < d.viewport.YOffset {
				d.viewport.SetYOffset(d.copyCursor)
			}
			d.refreshContent()
		}
		return *d, nil
	case "G":
		d.copyCursor = lineCount - 1
		d.viewport.GotoBottom()
		d.refreshContent()
		return *d, nil
	case "g":
		if d.gPending {
			d.gPending = false
			d.copyCursor = 0
			d.viewport.GotoTop()
			d.refreshContent()
			return *d, nil
		}
		d.gPending = true
		return *d, tea.Tick(diffGTimeout, func(time.Time) tea.Msg {
			return DiffGTimerExpiredMsg{}
		})
	}
	return *d, nil
}

func (d *DiffView) yankSelection() string {
	lines := d.rawLines()
	if len(lines) == 0 {
		return ""
	}
	start, end := d.copySelectionRange()
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	return strings.Join(lines[start:end+1], "\n")
}

func (d *DiffView) copySelectionRange() (int, int) {
	start := d.copyAnchor
	end := d.copyCursor
	if start > end {
		start, end = end, start
	}
	return start, end
}

// StartMouseSelection begins a mouse drag selection at the given panel-relative coordinates.
func (d *DiffView) StartMouseSelection(relX, relY int) {
	d.copyMode = false
	bufLine := d.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	d.mouseSelecting = true
	d.mouseAnchorLine = bufLine
	d.mouseAnchorCol = col
	d.mouseCurrentLine = bufLine
	d.mouseCurrentCol = col
	d.refreshContent()
}

// ExtendMouseSelection updates the cursor position during a mouse drag.
func (d *DiffView) ExtendMouseSelection(relX, relY int) {
	if !d.mouseSelecting {
		return
	}
	bufLine := d.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	d.mouseCurrentLine = bufLine
	d.mouseCurrentCol = col
	d.refreshContent()
}

// FinalizeMouseSelection ends the mouse drag and returns the selected text.
// Returns empty string for single-click (no drag).
func (d *DiffView) FinalizeMouseSelection(relX, relY int) string {
	if !d.mouseSelecting {
		return ""
	}
	d.mouseSelecting = false
	bufLine := d.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	d.mouseCurrentLine = bufLine
	d.mouseCurrentCol = col

	// Single click (same position) — no copy
	if d.mouseAnchorLine == d.mouseCurrentLine && d.mouseAnchorCol == d.mouseCurrentCol {
		d.refreshContent()
		return ""
	}

	content := d.styledContent()
	sl, sc, el, ec := d.normalizedMouseSelection()
	text := extractCharSelection(content, sl, sc, el, ec)
	d.refreshContent()
	return text
}

// CancelMouseSelection clears mouse selection state without copying.
func (d *DiffView) CancelMouseSelection() {
	d.mouseSelecting = false
	d.refreshContent()
}

// normalizedMouseSelection returns the mouse selection with start before end.
func (d *DiffView) normalizedMouseSelection() (startLine, startCol, endLine, endCol int) {
	startLine, startCol = d.mouseAnchorLine, d.mouseAnchorCol
	endLine, endCol = d.mouseCurrentLine, d.mouseCurrentCol
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		startLine, startCol, endLine, endCol = endLine, endCol, startLine, startCol
	}
	return
}

func (d *DiffView) rawLines() []string {
	var lines []string
	if d.diffStat != "" {
		statLines := strings.Split(strings.TrimRight(d.diffStat, "\n"), "\n")
		lines = append(lines, statLines...)
		lines = append(lines, "") // blank line after stat
	}
	if d.rawDiff != "" {
		lines = append(lines, strings.Split(d.rawDiff, "\n")...)
	}
	return lines
}

func (d *DiffView) parseFileOffsets() {
	d.fileOffsets = nil
	if d.rawDiff == "" {
		return
	}

	// Count how many lines the stat summary adds at the top of the
	// rendered viewport content so file offsets align with the viewport.
	statLineCount := 0
	if d.diffStat != "" {
		statLineCount = len(strings.Split(strings.TrimRight(d.diffStat, "\n"), "\n"))
		statLineCount++ // trailing blank line after stat block
	}

	lines := strings.Split(d.rawDiff, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			d.fileOffsets = append(d.fileOffsets, i+statLineCount)
		}
	}
}

// styledContent returns the rendered content without selection highlighting.
func (d *DiffView) styledContent() string {
	switch {
	case d.loading:
		return styles.TextDimStyle.Render("Loading diff...")
	case d.errMsg != "":
		return styles.DiffRemovedStyle.Render("Error: " + d.errMsg)
	case d.emptyMsg != "":
		return styles.TextDimStyle.Render(d.emptyMsg)
	default:
		return d.renderStyledDiff()
	}
}

func (d *DiffView) refreshContent() {
	content := d.styledContent()
	if d.copyMode {
		selStart, selEnd := d.copySelectionRange()
		content = applySelectionHighlight(content, selStart, selEnd)
	} else if d.mouseSelecting {
		sl, sc, el, ec := d.normalizedMouseSelection()
		content = applyCharSelectionHighlight(content, sl, sc, el, ec)
	}
	d.viewport.SetContent(content)
}

func (d *DiffView) renderStyledDiff() string {
	if d.rawDiff == "" {
		return ""
	}

	var b strings.Builder

	// Stat summary at the top
	if d.diffStat != "" {
		statLines := strings.Split(strings.TrimRight(d.diffStat, "\n"), "\n")
		for _, line := range statLines {
			b.WriteString(styles.TextSecondaryStyle.Render(line))
			b.WriteByte('\n')
		}
		b.WriteByte('\n')
	}

	lines := strings.Split(d.rawDiff, "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "diff --git"):
			b.WriteString(styles.DiffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "index "):
			b.WriteString(styles.DiffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "--- "):
			b.WriteString(styles.DiffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "+++ "):
			b.WriteString(styles.DiffHeaderStyle.Render(line))
		case strings.HasPrefix(line, "@@"):
			b.WriteString(styles.DiffHunkStyle.Render(line))
		case strings.HasPrefix(line, "+"):
			b.WriteString(styles.DiffAddedStyle.Render(line))
		case strings.HasPrefix(line, "-"):
			b.WriteString(styles.DiffRemovedStyle.Render(line))
		default:
			b.WriteString(line)
		}
		b.WriteByte('\n')
	}

	return strings.TrimRight(b.String(), "\n")
}
