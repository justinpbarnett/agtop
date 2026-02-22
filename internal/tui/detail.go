package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jpb/agtop/internal/run"
)

type Detail struct {
	activeTab   int
	tabNames    []string
	logViewer   LogViewer
	diffViewer  DiffViewer
	width       int
	height      int
	selectedRun *run.Run
}

func NewDetail() Detail {
	return Detail{
		tabNames:   []string{"Details", "Logs", "Diff"},
		logViewer:  NewLogViewer(),
		diffViewer: NewDiffViewer(),
	}
}

func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "l", "right":
			d.activeTab = (d.activeTab + 1) % len(d.tabNames)
			return d, nil
		case "h", "left":
			d.activeTab = (d.activeTab - 1 + len(d.tabNames)) % len(d.tabNames)
			return d, nil
		}
	}

	var cmd tea.Cmd
	switch d.activeTab {
	case 1:
		d.logViewer, cmd = d.logViewer.Update(msg)
	case 2:
		d.diffViewer, cmd = d.diffViewer.Update(msg)
	}
	return d, cmd
}

func (d Detail) View() string {
	tabs := d.renderTabs()

	contentHeight := d.height - 2
	if contentHeight < 0 {
		contentHeight = 0
	}

	var content string
	if d.selectedRun == nil {
		content = lipgloss.Place(d.width, contentHeight,
			lipgloss.Center, lipgloss.Center, "No run selected")
	} else {
		switch d.activeTab {
		case 0:
			content = d.renderDetails()
		case 1:
			content = d.logViewer.View()
		case 2:
			content = d.diffViewer.View()
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, tabs, content)
}

func (d *Detail) SetRun(r *run.Run) {
	d.selectedRun = r
}

func (d *Detail) SetSize(w, h int) {
	d.width = w
	d.height = h
	contentHeight := h - 2
	if contentHeight < 0 {
		contentHeight = 0
	}
	d.logViewer.SetSize(w, contentHeight)
	d.diffViewer.SetSize(w, contentHeight)
}

func (d Detail) renderTabs() string {
	var tabs []string
	for i, name := range d.tabNames {
		if i == d.activeTab {
			tabs = append(tabs, ActiveTabStyle.Render(name))
		} else {
			tabs = append(tabs, TabStyle.Render(name))
		}
	}
	return strings.Join(tabs, " │ ")
}

func (d Detail) renderDetails() string {
	r := d.selectedRun
	if r == nil {
		return ""
	}

	icon := RunStateStyle(r.State).Render(r.StatusIcon())

	var b strings.Builder
	fmt.Fprintf(&b, "  Run       #%s\n", r.ID)
	fmt.Fprintf(&b, "  Branch    %s\n", r.Branch)
	fmt.Fprintf(&b, "  Workflow  %s\n", r.Workflow)
	fmt.Fprintf(&b, "  State     %s %s\n", icon, r.State)
	if r.SkillTotal > 0 {
		fmt.Fprintf(&b, "  Progress  %d / %d skills\n", r.SkillIndex, r.SkillTotal)
	}
	fmt.Fprintf(&b, "  Tokens    %s\n", formatTokens(r.Tokens))
	fmt.Fprintf(&b, "  Cost      $%.2f\n", r.Cost)
	if r.Worktree != "" {
		fmt.Fprintf(&b, "  Worktree  %s\n", r.Worktree)
	}

	return b.String()
}
