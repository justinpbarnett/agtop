package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpb/agtop/internal/run"
)

type RunList struct {
	runs     []run.Run
	selected int
	width    int
	height   int
	lastKeyG bool
	lastKeyT time.Time
}

func NewRunList() RunList {
	return RunList{
		runs: []run.Run{
			{ID: "001", Branch: "feat/add-auth", Workflow: "sdlc", State: run.StateRunning, SkillIndex: 3, SkillTotal: 7, Tokens: 12400, Cost: 0.42},
			{ID: "002", Branch: "fix/nav-bug", Workflow: "quick-fix", State: run.StatePaused, SkillIndex: 1, SkillTotal: 3, Tokens: 3100, Cost: 0.08},
			{ID: "003", Branch: "feat/dashboard", Workflow: "plan-build", State: run.StateReviewing, SkillIndex: 3, SkillTotal: 3, Tokens: 45200, Cost: 1.23},
			{ID: "004", Branch: "fix/css-overflow", Workflow: "build", State: run.StateFailed, SkillIndex: 2, SkillTotal: 3, Tokens: 8700, Cost: 0.31},
		},
	}
}

func (r RunList) Update(msg tea.Msg) (RunList, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if r.selected < len(r.runs)-1 {
				r.selected++
			}
			r.lastKeyG = false
		case "k", "up":
			if r.selected > 0 {
				r.selected--
			}
			r.lastKeyG = false
		case "G":
			r.selected = len(r.runs) - 1
			r.lastKeyG = false
		case "g":
			if r.lastKeyG && time.Since(r.lastKeyT) < 500*time.Millisecond {
				r.selected = 0
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

func (r RunList) View() string {
	if len(r.runs) == 0 {
		return "No runs. Press n to start one."
	}

	var b strings.Builder
	for i, rn := range r.runs {
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
		if i < len(r.runs)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (r *RunList) SetSize(w, h int) {
	r.width = w
	r.height = h
}

func (r RunList) SelectedRun() *run.Run {
	if len(r.runs) == 0 {
		return nil
	}
	rn := r.runs[r.selected]
	return &rn
}

func formatTokens(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}
