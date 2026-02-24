package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

// Column widths for run list layout.
const (
	colIconW   = 2
	colIDW     = 5
	colStateW  = 11
	colTimeW   = 7
	colTokensW = 8
	colCostW   = 7
)

var runSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type RunList struct {
	store        *run.Store
	filtered     []run.Run
	selected     int
	offset       int
	width        int
	height       int
	lastKeyG     bool
	lastKeyT     time.Time
	filterActive bool
	filterText   string
	filterInput  textinput.Model
	focused      bool
	tickStep     int
}

func NewRunList(store *run.Store) RunList {
	ti := textinput.New()
	ti.Placeholder = "Filter..."
	ti.CharLimit = 64

	rl := RunList{
		store:       store,
		filterInput: ti,
	}
	rl.applyFilter()
	return rl
}

func (r RunList) Update(msg tea.Msg) (RunList, tea.Cmd) {
	switch msg.(type) {
	case RunStoreUpdatedMsg:
		r.applyFilter()
		r.clampSelection()
		return r, nil
	case AnimTickMsg:
		r.tickStep++
		return r, nil
	}

	msg2, ok := msg.(tea.KeyMsg)
	if !ok {
		return r, nil
	}

	if r.filterActive {
		return r.updateFilter(msg2)
	}

	switch msg2.String() {
	case "/":
		r.filterActive = true
		r.filterInput.Focus()
		return r, nil
	case "j", "down":
		if r.selected < len(r.filtered)-1 {
			r.selected++
			r.scrollToSelection()
		}
		r.lastKeyG = false
	case "k", "up":
		if r.selected > 0 {
			r.selected--
			r.scrollToSelection()
		}
		r.lastKeyG = false
	case "y":
		if sel := r.SelectedRun(); sel != nil {
			return r, func() tea.Msg { return YankMsg{Text: sel.ID} }
		}
	case "G":
		r.selected = max(len(r.filtered)-1, 0)
		r.scrollToSelection()
		r.lastKeyG = false
	case "g":
		if r.lastKeyG && time.Since(r.lastKeyT) < 500*time.Millisecond {
			r.selected = 0
			r.scrollToSelection()
			r.lastKeyG = false
		} else {
			r.lastKeyG = true
			r.lastKeyT = time.Now()
		}
	default:
		r.lastKeyG = false
	}
	return r, nil
}

func (r *RunList) updateFilter(msg tea.KeyMsg) (RunList, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter, tea.KeyEsc:
		if msg.Type == tea.KeyEsc {
			r.filterText = ""
			r.filterInput.SetValue("")
		}
		r.filterActive = false
		r.filterInput.Blur()
		r.applyFilter()
		r.clampSelection()
		return *r, nil
	}

	var cmd tea.Cmd
	r.filterInput, cmd = r.filterInput.Update(msg)
	r.filterText = r.filterInput.Value()
	r.applyFilter()
	r.clampSelection()
	return *r, cmd
}

func (r RunList) View() string {
	innerWidth := r.width - 2 // border sides
	innerHeight := r.height - 2 // border top/bottom
	if innerWidth < 0 {
		innerWidth = 0
	}
	if innerHeight < 0 {
		innerHeight = 0
	}

	activeCount := 0
	for _, rn := range r.filtered {
		if !rn.IsTerminal() {
			activeCount++
		}
	}
	title := fmt.Sprintf("[1] Runs (%d active)", activeCount)

	var keybinds []border.Keybind
	if r.focused {
		keybinds = []border.Keybind{
			{Key: "↵", Label: " runs"},
			{Key: "n", Label: "ew"},
			{Key: "y", Label: "ank ID"},
			{Key: "/", Label: "filter"},
		}
	}

	content := r.renderContent(innerWidth, innerHeight)
	return border.RenderPanel(title, content, keybinds, r.width, r.height, r.focused)
}

func (r RunList) renderContent(width, height int) string {
	if len(r.filtered) == 0 {
		if r.filterActive || r.filterText != "" {
			return r.renderFilterBar(width) + "\nNo matching runs."
		}
		return "No runs. Press n to start one."
	}

	var b strings.Builder

	availableRows := height
	if r.filterActive {
		b.WriteString(r.renderFilterBar(width))
		b.WriteString("\n")
		availableRows--
	}

	// Column header
	header := fmt.Sprintf("%*s %*s  %-*s %*s %*s %*s",
		colIconW, "",
		colIDW, "ID",
		colStateW, "STATE",
		colTimeW, "TIME",
		colTokensW, "TOKENS",
		colCostW, "COST",
	)
	b.WriteString(styles.TextSecondaryStyle.Render(text.Truncate(header, width)))
	b.WriteString("\n")
	availableRows--

	if r.offset > 0 {
		b.WriteString(styles.TextDimStyle.Render("  ▲"))
		b.WriteString("\n")
		availableRows--
	}

	end := r.offset + availableRows
	if end > len(r.filtered) {
		end = len(r.filtered)
	}
	// Reserve a row for bottom scroll indicator if needed
	if end < len(r.filtered) && availableRows > 1 {
		end = r.offset + availableRows - 1
		if end > len(r.filtered) {
			end = len(r.filtered)
		}
	}

	for i := r.offset; i < end; i++ {
		rn := r.filtered[i]

		elapsed := text.FormatElapsed(rn.ElapsedTime())
		tokens := text.FormatTokens(rn.Tokens)
		cost := text.FormatCost(rn.Cost)

		statusIcon := rn.StatusIcon()
		if rn.State == run.StateRunning || rn.State == run.StateRouting {
			statusIcon = runSpinnerFrames[r.tickStep%len(runSpinnerFrames)]
		}

		var line string
		if i == r.selected {
			// Plain text for selected row so background covers the entire line
			plainLine := fmt.Sprintf("%s %*s  %-*s %*s %*s %*s",
				text.PadRight(statusIcon, colIconW),
				colIDW, rn.ID,
				colStateW, text.Truncate(string(rn.State), colStateW),
				colTimeW, elapsed,
				colTokensW, tokens,
				colCostW, cost,
			)
			plainLine = text.Truncate(plainLine, width)
			line = styles.SelectedRowStyle.Width(width).Render(plainLine)
		} else {
			icon := lipgloss.NewStyle().Foreground(styles.RunStateColor(rn.State)).Render(
				text.PadRight(statusIcon, colIconW),
			)
			costStyle := lipgloss.NewStyle().Foreground(styles.CostColor(rn.Cost))
			paddedCost := fmt.Sprintf("%*s", colCostW, cost)
			line = fmt.Sprintf("%s %*s  %-*s %*s %*s %s",
				icon,
				colIDW, rn.ID,
				colStateW, text.Truncate(string(rn.State), colStateW),
				colTimeW, elapsed,
				colTokensW, tokens,
				costStyle.Render(paddedCost),
			)
			line = text.Truncate(line, width)
			if rn.IsTerminal() {
				line = styles.TextDimStyle.Render(line)
			}
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if end < len(r.filtered) {
		b.WriteString("\n")
		b.WriteString(styles.TextDimStyle.Render("  ▼"))
	}

	return b.String()
}

func (r *RunList) SetSize(w, h int) {
	r.width = w
	r.height = h
	r.filterInput.Width = w - 6
	r.clampSelection()
}

func (r *RunList) SetFocused(focused bool) {
	r.focused = focused
}

func (r RunList) SelectedRun() *run.Run {
	if len(r.filtered) == 0 || r.selected >= len(r.filtered) {
		return nil
	}
	rn := r.filtered[r.selected]
	return &rn
}

func (r *RunList) applyFilter() {
	all := r.store.List()
	if r.filterText == "" {
		r.filtered = all
		return
	}
	query := strings.ToLower(r.filterText)
	filtered := make([]run.Run, 0, len(all))
	for _, rn := range all {
		if strings.Contains(strings.ToLower(rn.ID), query) ||
			strings.Contains(strings.ToLower(rn.Branch), query) ||
			strings.Contains(strings.ToLower(rn.Workflow), query) ||
			strings.Contains(strings.ToLower(string(rn.State)), query) ||
			strings.Contains(strings.ToLower(rn.CurrentSkill), query) ||
			strings.Contains(strings.ToLower(rn.TaskID), query) {
			filtered = append(filtered, rn)
		}
	}
	r.filtered = filtered
}

func (r *RunList) clampSelection() {
	if len(r.filtered) == 0 {
		r.selected = 0
		r.offset = 0
		return
	}
	if r.selected >= len(r.filtered) {
		r.selected = len(r.filtered) - 1
	}
	if r.selected < 0 {
		r.selected = 0
	}
	r.scrollToSelection()
}

func (r *RunList) scrollToSelection() {
	visible := r.visibleRows()
	if visible <= 0 {
		return
	}
	if r.selected < r.offset {
		r.offset = r.selected
	}
	if r.selected >= r.offset+visible {
		r.offset = r.selected - visible + 1
	}
	maxOffset := len(r.filtered) - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if r.offset > maxOffset {
		r.offset = maxOffset
	}
	if r.offset < 0 {
		r.offset = 0
	}
}

func (r RunList) visibleRows() int {
	rows := r.height - 2 // border top/bottom
	rows--               // column header
	if r.filterActive {
		rows--
	}
	if r.offset > 0 {
		rows--
	}
	if r.offset+rows < len(r.filtered) {
		rows--
	}
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (r RunList) renderFilterBar(width int) string {
	return "/ " + r.filterInput.View()
}

// FilterActive reports whether the filter input is currently active.
func (r RunList) FilterActive() bool {
	return r.filterActive
}

// SelectByID navigates the list to the run with the given ID and returns
// true if found. The selection and scroll offset are updated accordingly.
func (r *RunList) SelectByID(id string) bool {
	for i, rn := range r.filtered {
		if rn.ID == id {
			r.selected = i
			r.scrollToSelection()
			return true
		}
	}
	return false
}
