package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

// DetailGTimerExpiredMsg is sent when the gg double-tap window expires in the detail panel.
type DetailGTimerExpiredMsg struct{}

type Detail struct {
	width       int
	height      int
	selectedRun *run.Run
	focused     bool
	viewport    viewport.Model
	gPending    bool
}

func NewDetail() Detail {
	return Detail{
		viewport: viewport.New(0, 0),
	}
}

func (d Detail) Update(msg tea.Msg) (Detail, tea.Cmd) {
	switch msg := msg.(type) {
	case DetailGTimerExpiredMsg:
		d.gPending = false
		return d, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "y":
			if d.selectedRun != nil {
				return d, func() tea.Msg { return YankMsg{Text: d.plainText()} }
			}
		case "j", "down":
			if d.focused && d.selectedRun != nil {
				d.viewport.SetYOffset(d.viewport.YOffset + 1)
				return d, nil
			}
		case "k", "up":
			if d.focused && d.selectedRun != nil {
				offset := d.viewport.YOffset - 1
				if offset < 0 {
					offset = 0
				}
				d.viewport.SetYOffset(offset)
				return d, nil
			}
		case "G":
			if d.focused && d.selectedRun != nil {
				d.viewport.GotoBottom()
				return d, nil
			}
		case "g":
			if d.focused && d.selectedRun != nil {
				if d.gPending {
					d.gPending = false
					d.viewport.GotoTop()
					return d, nil
				}
				d.gPending = true
				return d, tea.Tick(gTimeout, func(time.Time) tea.Msg {
					return DetailGTimerExpiredMsg{}
				})
			}
		}
	}
	return d, nil
}

func (d Detail) View() string {
	title := "[2] Details"

	var keybinds []border.Keybind
	if d.focused && d.selectedRun != nil {
		keybinds = []border.Keybind{
			{Key: "j/k", Label: " scroll"},
			{Key: "G", Label: " bottom"},
			{Key: "g", Label: "g top"},
			{Key: "y", Label: "ank"},
		}
	}

	var content string
	if d.selectedRun == nil {
		content = "No run selected"
	} else {
		d.viewport.SetContent(d.renderDetails())
		content = d.viewport.View()
	}

	return border.RenderPanel(title, content, keybinds, d.width, d.height, d.focused)
}

func (d *Detail) SetRun(r *run.Run) {
	d.selectedRun = r
	if r != nil {
		d.viewport.SetContent(d.renderDetails())
		d.viewport.GotoTop()
	}
}

func (d *Detail) SetSize(w, h int) {
	d.width = w
	d.height = h
	innerW := w - 2
	innerH := h - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	d.viewport.Width = innerW
	d.viewport.Height = innerH
	if d.selectedRun != nil {
		d.viewport.SetContent(d.renderDetails())
	}
}

func (d *Detail) SetFocused(focused bool) {
	d.focused = focused
}

func (d Detail) plainText() string {
	r := d.selectedRun
	if r == nil {
		return ""
	}

	var b strings.Builder
	row := func(key, val string) {
		fmt.Fprintf(&b, "%-9s: %s\n", key, val)
	}

	if r.TaskID != "" {
		row("Task", r.TaskID)
	}
	if r.Prompt != "" {
		row("Prompt", r.Prompt)
	}
	for i, fp := range r.FollowUpPrompts {
		row(fmt.Sprintf("Update %d", i+1), fp)
	}

	statusText := string(r.State)
	if !r.StartedAt.IsZero() {
		statusText += fmt.Sprintf(" (%s)", text.FormatElapsedVerbose(r.ElapsedTime()))
	}
	row("Status", statusText)

	if r.Workflow != "" {
		row("Workflow", r.Workflow)
	}

	skillName := r.CurrentSkill
	if skillName == "" {
		skillName = r.Workflow
	}
	stepText := skillName
	if r.SkillTotal > 0 {
		stepText = fmt.Sprintf("%s (%d/%d)", skillName, r.SkillIndex, r.SkillTotal)
	}
	row("Step", stepText)
	row("Branch", r.Branch)

	model := r.Model
	if model == "" {
		model = "—"
	}
	row("Model", model)

	row("Tokens", fmt.Sprintf("%s (%s in / %s out)", text.FormatTokens(r.Tokens), text.FormatTokens(r.TokensIn), text.FormatTokens(r.TokensOut)))
	row("Cost", text.FormatCost(r.Cost))

	if r.Worktree != "" {
		row("Worktree", r.Worktree)
	}
	if r.DevServerURL != "" {
		row("DevServer", r.DevServerURL)
	}
	if r.Command != "" {
		row("Command", r.Command)
	}
	if r.MergeStatus != "" {
		row("Merge", r.MergeStatus)
	}
	if r.PRURL != "" {
		row("PR", r.PRURL)
	}
	if r.Error != "" {
		row("Error", r.Error)
	}

	if len(r.SkillCosts) > 0 {
		fmt.Fprintf(&b, "\n%-12s %8s %8s\n", "Skill", "Tokens", "Cost")
		for _, sc := range r.SkillCosts {
			name := sc.SkillName
			if name == "" {
				name = "—"
			}
			fmt.Fprintf(&b, "%-12s %8s %s\n", text.Truncate(name, 12), text.FormatTokens(sc.TotalTokens), text.FormatCost(sc.CostUSD))
		}
		fmt.Fprintf(&b, "%s\n", strings.Repeat("─", 32))
		fmt.Fprintf(&b, "%-12s %8s %s\n", "Total", text.FormatTokens(r.Tokens), text.FormatCost(r.Cost))
	}

	return strings.TrimRight(b.String(), "\n")
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
	if !r.StartedAt.IsZero() {
		statusText += fmt.Sprintf(" (%s)", text.FormatElapsedVerbose(r.ElapsedTime()))
	}

	skillName := r.CurrentSkill
	if skillName == "" {
		skillName = r.Workflow
	}

	var b strings.Builder
	row := func(key, val string) string {
		return keyStyle.Render(fmt.Sprintf("%-9s: ", key)) + valStyle.Render(val)
	}
	styledRow := func(key string, val string, style lipgloss.Style) string {
		return keyStyle.Render(fmt.Sprintf("%-9s: ", key)) + style.Render(val)
	}
	// wrappedRow renders a field whose value may span multiple lines.
	// Lines are wrapped to fit the panel width. When maxLines > 0, output is
	// capped at maxLines (plus a "+N lines" indicator when truncated).
	// When maxLines <= 0, all wrapped lines are rendered without truncation.
	// prefixWidth = 2 (indent) + 11 ("%-9s: ") = 13
	const prefixWidth = 13
	wrappedRow := func(key, val string, style lipgloss.Style, maxLines int) string {
		innerW := d.width - 2
		valW := innerW - prefixWidth
		if valW < 20 {
			return fmt.Sprintf("  %s\n", styledRow(key, text.Truncate(val, valW), style))
		}
		wrapped := text.WrapText(val, valW)
		totalLines := len(wrapped)
		truncated := maxLines > 0 && totalLines > maxLines
		if truncated {
			wrapped = wrapped[:maxLines]
		}
		var sb strings.Builder
		for i, line := range wrapped {
			if i == 0 {
				sb.WriteString(fmt.Sprintf("  %s\n", keyStyle.Render(fmt.Sprintf("%-9s: ", key))+style.Render(line)))
			} else {
				sb.WriteString(fmt.Sprintf("  %s%s\n", strings.Repeat(" ", 11), style.Render(line)))
			}
		}
		if truncated {
			remaining := totalLines - maxLines
			indicator := fmt.Sprintf("%s+%d lines", strings.Repeat(" ", 11), remaining)
			sb.WriteString("  " + styles.TextSecondaryStyle.Render(indicator) + "\n")
		}
		return sb.String()
	}

	if r.TaskID != "" {
		fmt.Fprintf(&b, "  %s\n", row("Task", r.TaskID))
	}

	if r.Prompt != "" {
		b.WriteString(wrappedRow("Prompt", r.Prompt, valStyle, 0))
	}

	for i, fp := range r.FollowUpPrompts {
		label := fmt.Sprintf("Update %d", i+1)
		b.WriteString(wrappedRow(label, fp, valStyle, 3))
	}

	fmt.Fprintf(&b, "  %s\n", styledRow("Status", statusText, stateColor))

	if r.Workflow != "" {
		fmt.Fprintf(&b, "  %s\n", row("Workflow", r.Workflow))
	}

	stepText := skillName
	if r.SkillTotal > 0 {
		stepText = fmt.Sprintf("%s (%d/%d)", skillName, r.SkillIndex, r.SkillTotal)
	}
	fmt.Fprintf(&b, "  %s\n", row("Step", stepText))

	fmt.Fprintf(&b, "  %s\n", row("Branch", r.Branch))

	model := r.Model
	if model == "" {
		model = "—"
	}
	fmt.Fprintf(&b, "  %s\n", row("Model", model))

	tokStr := fmt.Sprintf("%s (%s in / %s out)", text.FormatTokens(r.Tokens), text.FormatTokens(r.TokensIn), text.FormatTokens(r.TokensOut))
	fmt.Fprintf(&b, "  %s\n", row("Tokens", tokStr))
	fmt.Fprintf(&b, "  %s\n", styledRow("Cost", text.FormatCost(r.Cost), costColor))

	if r.Worktree != "" {
		fmt.Fprintf(&b, "  %s\n", row("Worktree", r.Worktree))
	}

	if r.DevServerURL != "" {
		devStyle := lipgloss.NewStyle().Foreground(styles.StatusRunning)
		fmt.Fprintf(&b, "  %s\n", styledRow("DevServer", r.DevServerURL, devStyle))
	}

	if r.Command != "" {
		fmt.Fprintf(&b, "  %s\n", row("Command", r.Command))
	}

	if r.MergeStatus != "" {
		mergeStyle := lipgloss.NewStyle().Foreground(styles.StatusRunning)
		if r.MergeStatus == "merged" {
			mergeStyle = lipgloss.NewStyle().Foreground(styles.StatusSuccess)
		} else if r.MergeStatus == "failed" {
			mergeStyle = lipgloss.NewStyle().Foreground(styles.StatusError)
		}
		fmt.Fprintf(&b, "  %s\n", styledRow("Merge", r.MergeStatus, mergeStyle))
	}

	if r.PRURL != "" {
		fmt.Fprintf(&b, "  %s\n", row("PR", r.PRURL))
	}

	if r.Error != "" {
		errorStyle := lipgloss.NewStyle().Foreground(styles.StatusError)
		fmt.Fprintf(&b, "  %s\n", styledRow("Error", r.Error, errorStyle))
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
