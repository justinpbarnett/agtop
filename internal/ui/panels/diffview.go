package panels

import (
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
		d.viewport.Height = h
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
		switch msg.String() {
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

// Content returns the rendered content string for embedding in another panel.
func (d DiffView) Content() string {
	return d.viewport.View()
}

// Keybinds returns the keybinds to show when this view is active.
func (d DiffView) Keybinds() []border.Keybind {
	if !d.focused {
		return nil
	}
	binds := []border.Keybind{
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

func (d *DiffView) refreshContent() {
	var content string
	switch {
	case d.loading:
		content = styles.TextDimStyle.Render("Loading diff...")
	case d.errMsg != "":
		content = styles.DiffRemovedStyle.Render("Error: " + d.errMsg)
	case d.emptyMsg != "":
		content = styles.TextDimStyle.Render(d.emptyMsg)
	default:
		content = d.renderStyledDiff()
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
