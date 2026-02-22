package panels

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

const (
	tabDetails = 0
	tabDiff    = 1
	numTabs    = 2
)

type Detail struct {
	width       int
	height      int
	selectedRun *run.Run
	focused     bool
	activeTab   int
	diffView    DiffView
}

func NewDetail() Detail {
	return Detail{
		diffView: NewDiffView(),
	}
}

func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case DiffGTimerExpiredMsg:
		if d.activeTab == tabDiff {
			var cmd tea.Cmd
			d.diffView, cmd = d.diffView.Update(msg)
			return d, cmd
		}
		return d, nil
	case tea.KeyMsg:
		if !d.focused {
			return d, nil
		}
		switch msg.String() {
		case "h", "left":
			if d.activeTab > 0 {
				d.activeTab--
				d.updateDiffFocus()
			}
			return d, nil
		case "l", "right":
			if d.activeTab < numTabs-1 {
				d.activeTab++
				d.updateDiffFocus()
			}
			return d, nil
		}
		if d.activeTab == tabDiff {
			var cmd tea.Cmd
			d.diffView, cmd = d.diffView.Update(msg)
			return d, cmd
		}
	}
	return d, nil
}

func (d Detail) View() string {
	tabNames := []string{"Details", "Diff"}
	var titleParts []string
	for i, name := range tabNames {
		if i == d.activeTab {
			titleParts = append(titleParts, styles.TitleStyle.Render(name))
		} else {
			titleParts = append(titleParts, styles.TextDimStyle.Render(name))
		}
	}
	title := strings.Join(titleParts, styles.TextDimStyle.Render(" │ "))

	var keybinds []border.Keybind
	if d.focused {
		keybinds = []border.Keybind{
			{Key: "h", Label: "/l tab"},
		}
		if d.activeTab == tabDiff {
			keybinds = append(keybinds, d.diffView.Keybinds()...)
		}
	}

	var content string
	if d.selectedRun == nil {
		content = "No run selected"
	} else {
		switch d.activeTab {
		case tabDetails:
			content = d.renderDetails()
		case tabDiff:
			content = d.diffView.Content()
		}
	}

	return border.RenderPanel(title, content, keybinds, d.width, d.height, d.focused)
}

func (d *Detail) SetRun(r *run.Run) {
	d.selectedRun = r
}

func (d *Detail) SetDiff(diff, stat string) {
	d.diffView.SetDiff(diff, stat)
}

func (d *Detail) SetDiffLoading() {
	d.diffView.SetLoading()
}

func (d *Detail) SetDiffError(err string) {
	d.diffView.SetError(err)
}

func (d *Detail) SetDiffEmpty() {
	d.diffView.SetEmpty()
}

func (d *Detail) SetDiffNoBranch() {
	d.diffView.SetNoBranch()
}

func (d *Detail) SetDiffWaiting() {
	d.diffView.SetWaiting()
}

func (d *Detail) SetSize(w, h int) {
	d.width = w
	d.height = h
	// Inner dimensions for the diff viewport (accounting for panel borders)
	innerW := w - 2
	innerH := h - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	d.diffView.SetSize(innerW, innerH)
}

func (d *Detail) SetFocused(focused bool) {
	d.focused = focused
	d.updateDiffFocus()
}

func (d *Detail) updateDiffFocus() {
	d.diffView.SetFocused(d.focused && d.activeTab == tabDiff)
}

func (d Detail) renderDetails() string {
	r := d.selectedRun
	if r == nil {
		return ""
	}

	keyStyle := styles.TextSecondaryStyle
	valStyle := styles.TextPrimaryStyle
	stateColor := lipgloss.NewStyle().Foreground(styles.RunStateColor(r.State))
	costColor := lipgloss.NewStyle().Foreground(styles.CostColor(r.Cost))

	statusText := string(r.State)
	if !r.IsTerminal() && !r.StartedAt.IsZero() {
		statusText += fmt.Sprintf(" (%s)", text.FormatElapsed(r.ElapsedTime()))
	}

	skillName := r.CurrentSkill
	if skillName == "" {
		skillName = r.Workflow
	}

	var b strings.Builder
	// Two-column key-value layout
	leftCol := func(key, val string) string {
		return keyStyle.Render(key+": ") + valStyle.Render(val)
	}
	rightCol := func(key, val string) string {
		return keyStyle.Render(key+": ") + valStyle.Render(val)
	}
	styledRight := func(key string, val string, style lipgloss.Style) string {
		return keyStyle.Render(key+": ") + style.Render(val)
	}

	// Row 1: Skill + Branch
	fmt.Fprintf(&b, "  %s    %s\n",
		leftCol("Skill", skillName),
		rightCol("Branch", r.Branch))

	// Row 2: Model + Status
	model := r.Model
	if model == "" {
		model = "—"
	}
	fmt.Fprintf(&b, "  %s    %s\n",
		leftCol("Model", model),
		styledRight("Status", statusText, stateColor))

	// Row 3: Tokens + Cost
	tokStr := fmt.Sprintf("%s in / %s out", text.FormatTokens(r.TokensIn), text.FormatTokens(r.TokensOut))
	fmt.Fprintf(&b, "  %s    %s\n",
		leftCol("Tokens", tokStr),
		styledRight("Cost", text.FormatCost(r.Cost), costColor))

	// Row 4: Worktree (if present)
	if r.Worktree != "" {
		fmt.Fprintf(&b, "  %s\n", leftCol("Worktree", r.Worktree))
	}

	// Row 5: Dev Server (if running)
	if r.DevServerURL != "" {
		devStyle := lipgloss.NewStyle().Foreground(styles.StatusRunning)
		fmt.Fprintf(&b, "  %s\n", styledRight("DevServer", r.DevServerURL, devStyle))
	}

	// Row 6: Command (if present)
	if r.Command != "" {
		fmt.Fprintf(&b, "  %s\n", leftCol("Command", r.Command))
	}

	// Row 7: Error (if present)
	if r.Error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(styles.StatusError)
		fmt.Fprintf(&b, "  %s\n", styledRight("Error", r.Error, errorStyle))
	}

	// Per-skill cost breakdown
	if len(r.SkillCosts) > 0 {
		b.WriteString("\n")
		headerStyle := styles.TextSecondaryStyle
		b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(fmt.Sprintf("%-12s %8s %8s", "Skill", "Tokens", "Cost"))))

		for _, sc := range r.SkillCosts {
			name := sc.SkillName
			if name == "" {
				name = "—"
			}
			scCostStyle := lipgloss.NewStyle().Foreground(styles.CostColor(sc.CostUSD))
			b.WriteString(fmt.Sprintf("  %-12s %8s %s\n",
				valStyle.Render(text.Truncate(name, 12)),
				valStyle.Render(text.FormatTokens(sc.TotalTokens)),
				scCostStyle.Render(text.FormatCost(sc.CostUSD)),
			))
		}

		b.WriteString(fmt.Sprintf("  %s\n", headerStyle.Render(strings.Repeat("─", 32))))
		b.WriteString(fmt.Sprintf("  %-12s %8s %s\n",
			valStyle.Render("Total"),
			valStyle.Render(text.FormatTokens(r.Tokens)),
			costColor.Render(text.FormatCost(r.Cost)),
		))
	}

	return b.String()
}
