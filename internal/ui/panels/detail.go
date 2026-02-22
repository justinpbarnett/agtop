package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

type Detail struct {
	width       int
	height      int
	selectedRun *run.Run
	focused     bool
}

func NewDetail() Detail {
	return Detail{}
}

func (d Detail) View() string {
	title := "2 Details"

	var keybinds []border.Keybind

	var content string
	if d.selectedRun == nil {
		content = "No run selected"
	} else {
		content = d.renderDetails()
	}

	return border.RenderPanel(title, content, keybinds, d.width, d.height, d.focused)
}

func (d *Detail) SetRun(r *run.Run) {
	d.selectedRun = r
}

func (d *Detail) SetSize(w, h int) {
	d.width = w
	d.height = h
}

func (d *Detail) SetFocused(focused bool) {
	d.focused = focused
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
