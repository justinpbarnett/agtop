package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/run"
)

type RunList struct {
	store       *run.Store
	filtered    []run.Run
	selected    int
	offset      int
	width       int
	height      int
	lastKeyG    bool
	lastKeyT    time.Time
	filterActive bool
	filterText  string
	filterInput textinput.Model
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
	switch msg := msg.(type) {
	case RunStoreUpdatedMsg:
		r.applyFilter()
		r.clampSelection()
		return r, nil

	case tea.KeyMsg:
		if r.filterActive {
			return r.updateFilter(msg)
		}

		switch msg.String() {
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
	if len(r.filtered) == 0 {
		if r.filterActive || r.filterText != "" {
			return r.renderFilterBar() + "\nNo matching runs."
		}
		return "No runs. Press n to start one."
	}

	var b strings.Builder

	if r.filterActive {
		b.WriteString(r.renderFilterBar())
		b.WriteString("\n")
	}

	visibleRows := r.visibleRows()
	end := r.offset + visibleRows
	if end > len(r.filtered) {
		end = len(r.filtered)
	}

	if r.offset > 0 {
		b.WriteString(DimStyle.Render("  ▲"))
		b.WriteString("\n")
	}

	for i := r.offset; i < end; i++ {
		rn := r.filtered[i]
		icon := RunStateStyle(rn.State).Render(rn.StatusIcon())

		progress := ""
		if !rn.IsTerminal() && rn.State != run.StateReviewing {
			progress = fmt.Sprintf("[%d/%d]", rn.SkillIndex, rn.SkillTotal)
		}

		tokens := formatTokens(rn.Tokens)

		line := fmt.Sprintf("%s #%s  %-16s %-10s %-8s %-5s %8s  $%.2f",
			icon, rn.ID, rn.Branch, rn.Workflow, rn.State, progress, tokens, rn.Cost)

		if i == r.selected {
			line = SelectedStyle.Width(r.width).Render(line)
		} else if rn.IsTerminal() {
			line = DimStyle.Render(line)
		}

		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	if end < len(r.filtered) {
		b.WriteString("\n")
		b.WriteString(DimStyle.Render("  ▼"))
	}

	return b.String()
}

func (r *RunList) SetSize(w, h int) {
	r.width = w
	r.height = h
	r.filterInput.Width = w - 4
	r.clampSelection()
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
			strings.Contains(strings.ToLower(rn.CurrentSkill), query) {
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
	rows := r.height
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

func (r RunList) renderFilterBar() string {
	return "/ " + r.filterInput.View()
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
