package panels

import (
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/detect"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

type OnboardingModal struct {
	step     int // 0 = runtime selection, 1 = confirm
	runtimes []string
	selected int
	detected *detect.Result
	width    int
	height   int
}

func NewOnboardingModal() *OnboardingModal {
	var runtimes []string
	if _, err := exec.LookPath("claude"); err == nil {
		runtimes = append(runtimes, "claude")
	}
	if _, err := exec.LookPath("opencode"); err == nil {
		runtimes = append(runtimes, "opencode")
	}

	var detected *detect.Result
	if r, err := detect.Detect("."); err == nil {
		detected = r
	}

	m := &OnboardingModal{
		runtimes: runtimes,
		detected: detected,
		width:    54,
		height:   12,
	}

	if len(runtimes) == 1 {
		m.step = 1
	}

	return m
}

func (m OnboardingModal) SelectedRuntime() string {
	if len(m.runtimes) == 0 {
		return ""
	}
	return m.runtimes[m.selected]
}

func (m OnboardingModal) Update(msg tea.Msg) (OnboardingModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.step {
		case 0:
			return m.updateRuntimeSelect(msg)
		case 1:
			return m.updateConfirm(msg)
		}
	}
	return m, nil
}

func (m OnboardingModal) updateRuntimeSelect(msg tea.KeyMsg) (OnboardingModal, tea.Cmd) {
	if len(m.runtimes) == 0 {
		if msg.String() == "esc" || msg.String() == "q" {
			return m, func() tea.Msg { return CloseModalMsg{} }
		}
		return m, nil
	}

	switch msg.String() {
	case "j", "down":
		m.selected = (m.selected + 1) % len(m.runtimes)
	case "k", "up":
		m.selected = (m.selected - 1 + len(m.runtimes)) % len(m.runtimes)
	case "enter":
		m.step = 1
	case "esc", "n", "N":
		return m, func() tea.Msg { return CloseModalMsg{} }
	}
	return m, nil
}

func (m OnboardingModal) updateConfirm(msg tea.KeyMsg) (OnboardingModal, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		rt := m.SelectedRuntime()
		return m, func() tea.Msg { return InitAcceptedMsg{Runtime: rt} }
	case "esc", "backspace":
		if len(m.runtimes) > 1 {
			m.step = 0
			return m, nil
		}
		return m, func() tea.Msg { return CloseModalMsg{} }
	}
	return m, nil
}

func (m OnboardingModal) View() string {
	switch m.step {
	case 0:
		return m.viewRuntimeSelect()
	case 1:
		return m.viewConfirm()
	}
	return ""
}

func (m OnboardingModal) viewRuntimeSelect() string {
	if len(m.runtimes) == 0 {
		body := styles.TextPrimaryStyle.Render("No AI runtime found.") + "\n" +
			"\n" +
			styles.TextSecondaryStyle.Render("Install claude or opencode first.")
		kb := []border.Keybind{{Key: "Esc", Label: " close"}}
		return border.RenderPanel("Setup agtop", body, kb, m.width, m.height, true)
	}

	var b strings.Builder
	b.WriteString(styles.TextPrimaryStyle.Render("Select your AI coding runtime:"))
	b.WriteString("\n\n")

	for i, rt := range m.runtimes {
		if i == m.selected {
			b.WriteString(styles.SelectedOptionStyle.Render("> " + rt))
		} else {
			b.WriteString(styles.TextDimStyle.Render("  " + rt))
		}
		b.WriteString("\n")
	}

	kb := []border.Keybind{
		{Key: "j/k", Label: " select"},
		{Key: "Enter", Label: " confirm"},
		{Key: "Esc", Label: " skip"},
	}
	return border.RenderPanel("Setup agtop", b.String(), kb, m.width, m.height, true)
}

func (m OnboardingModal) viewConfirm() string {
	var b strings.Builder
	rt := m.SelectedRuntime()

	b.WriteString(styles.TextPrimaryStyle.Render("Runtime: "))
	b.WriteString(styles.SelectedOptionStyle.Render(rt))
	b.WriteString("\n")

	if m.detected != nil {
		if m.detected.ProjectName != "" {
			b.WriteString(styles.TextSecondaryStyle.Render(fmt.Sprintf("Project: %s", m.detected.ProjectName)))
			b.WriteString("\n")
		}
		if m.detected.Language != "" {
			b.WriteString(styles.TextSecondaryStyle.Render(fmt.Sprintf("Language: %s", m.detected.Language)))
			b.WriteString("\n")
		}
		if m.detected.TestCommand != "" {
			b.WriteString(styles.TextSecondaryStyle.Render(fmt.Sprintf("Tests: %s", m.detected.TestCommand)))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(styles.TextPrimaryStyle.Render("Will create:"))
	b.WriteString("\n")
	b.WriteString(styles.TextSecondaryStyle.Render("  .agtop/hooks/safety-guard.sh"))
	b.WriteString("\n")

	switch rt {
	case "claude":
		b.WriteString(styles.TextSecondaryStyle.Render("  .claude/settings.json"))
	case "opencode":
		b.WriteString(styles.TextSecondaryStyle.Render("  opencode.json"))
	}
	b.WriteString("\n")
	b.WriteString(styles.TextSecondaryStyle.Render("  agtop.toml"))

	kb := []border.Keybind{
		{Key: "Enter", Label: " proceed"},
	}
	if len(m.runtimes) > 1 {
		kb = append(kb, border.Keybind{Key: "Esc", Label: " back"})
	} else {
		kb = append(kb, border.Keybind{Key: "Esc", Label: " skip"})
	}
	return border.RenderPanel("Setup agtop", b.String(), kb, m.width, m.height, true)
}
