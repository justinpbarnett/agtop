package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/selection"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

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
	gTap        DoubleTap
	sel         selection.Selection
}

// diffLinesProvider adapts DiffView.rawLines() to the selection.LinesProvider interface.
type diffLinesProvider struct{ d *DiffView }

func (p *diffLinesProvider) Lines() []string { return p.d.rawLines() }

func NewDiffView() DiffView {
	vp := viewport.New(0, 0)
	return DiffView{
		viewport: vp,
		emptyMsg: "No changes on branch",
		gTap:     NewDoubleTap(gTapIDDiffView),
	}
}

func (d *DiffView) resetState() {
	d.loading = false
	d.errMsg = ""
	d.emptyMsg = ""
	d.rawDiff = ""
	d.diffStat = ""
	d.fileOffsets = nil
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
	d.resetState()
	d.loading = true
	d.refreshContent()
}

func (d *DiffView) SetError(err string) {
	d.resetState()
	d.errMsg = err
	d.refreshContent()
}

func (d *DiffView) SetEmpty() {
	d.resetState()
	d.emptyMsg = "No changes on branch"
	d.refreshContent()
}

func (d *DiffView) SetNoBranch() {
	d.resetState()
	d.emptyMsg = "No branch — diff unavailable"
	d.refreshContent()
}

func (d *DiffView) SetWaiting() {
	d.resetState()
	d.emptyMsg = "Waiting for worktree..."
	d.refreshContent()
}

func (d *DiffView) SetSize(w, h int) {
	d.width = w
	d.height = h
	if w > 0 && h > 0 {
		d.viewport.Width = w
		vpH := h
		if d.sel.Active() {
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
	case GTimerExpiredMsg:
		d.gTap.HandleExpiry(msg)
		return d, nil
	case tea.KeyMsg:
		if d.sel.Active() {
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
			fired, cmd := d.gTap.Check()
			if fired {
				d.viewport.GotoTop()
				return d, nil
			}
			return d, cmd
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
	return d.sel.Active()
}

// Content returns the rendered content string for embedding in another panel.
func (d DiffView) Content() string {
	content := d.viewport.View()
	if d.sel.Active() {
		selStart, selEnd := d.sel.CopySelectionRange()
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
	if d.sel.Active() {
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
	provider := &diffLinesProvider{d: d}
	d.sel.EnterCopyMode(provider, d.viewport.YOffset, d.viewport.Height)
	if d.sel.Active() {
		d.refreshContent()
	}
}

func (d *DiffView) updateCopyMode(msg tea.KeyMsg) (DiffView, tea.Cmd) {
	provider := &diffLinesProvider{d: d}
	yankText, cmd := d.sel.UpdateCopyMode(msg, provider, &d.viewport, &d.gTap.Pending, GTimerExpiredMsg{ID: gTapIDDiffView})
	d.refreshContent()
	if yankText != "" {
		return *d, func() tea.Msg { return YankMsg{Text: yankText} }
	}
	return *d, cmd
}

// StartMouseSelection begins a mouse drag selection at the given panel-relative coordinates.
func (d *DiffView) StartMouseSelection(relX, relY int) {
	d.sel.StartMouse(relX, relY, d.viewport.YOffset)
	d.refreshContent()
}

// ExtendMouseSelection updates the cursor position during a mouse drag.
func (d *DiffView) ExtendMouseSelection(relX, relY int) {
	if !d.sel.MouseActive() {
		return
	}
	d.sel.ExtendMouse(relX, relY, d.viewport.YOffset)
	d.refreshContent()
}

// FinalizeMouseSelection ends the mouse drag and returns the selected text.
// Returns empty string for single-click (no drag).
func (d *DiffView) FinalizeMouseSelection(relX, relY int) string {
	if !d.sel.MouseActive() {
		return ""
	}
	sl, sc, el, ec, singleClick := d.sel.FinalizeMouse(relX, relY, d.viewport.YOffset)
	if singleClick {
		d.refreshContent()
		return ""
	}
	content := d.styledContent()
	text := extractCharSelection(content, sl, sc, el, ec)
	d.refreshContent()
	return text
}

// CancelMouseSelection clears mouse selection state without copying.
func (d *DiffView) CancelMouseSelection() {
	d.sel.CancelMouse()
	d.refreshContent()
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
	if d.sel.Active() {
		selStart, selEnd := d.sel.CopySelectionRange()
		content = applySelectionHighlight(content, selStart, selEnd)
	} else if d.sel.MouseActive() {
		sl, sc, el, ec := d.sel.NormalizedMouseSelection()
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
