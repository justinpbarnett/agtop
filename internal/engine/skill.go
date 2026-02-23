package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	PriorityProjectAgtop    = 0 // .agtop/skills/*/SKILL.md
	PriorityProjectClaude   = 1 // .claude/skills/*/SKILL.md
	PriorityProjectOpenCode = 2 // .opencode/skills/*/SKILL.md
	PriorityProjectAgents   = 3 // .agents/skills/*/SKILL.md
	PriorityUserAgtop       = 4 // ~/.config/agtop/skills/*/SKILL.md
	PriorityUserClaude      = 5 // ~/.claude/skills/*/SKILL.md
	PriorityUserOpenCode    = 6 // ~/.config/opencode/skills/*/SKILL.md
	PriorityBuiltIn         = 7 // <binary-dir>/skills/*/SKILL.md
)

type Skill struct {
	Name         string
	Description  string
	Model        string
	Timeout      int
	Parallel     bool
	AllowedTools []string
	Content      string // Full markdown body (everything after frontmatter)
	Source       string // Filesystem path where this skill was loaded from
	Priority     int    // Precedence level (0 = highest, 5 = lowest)
}

type skillFrontmatter struct {
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description"`
	Model         string   `yaml:"model"`
	Timeout       int      `yaml:"timeout"`
	ParallelGroup string   `yaml:"parallel-group"`
	AllowedTools  []string `yaml:"allowed-tools"`
}

// ParseSkill parses a SKILL.md file's content into a Skill struct.
// It extracts YAML frontmatter (delimited by ---) and the markdown body.
// If no frontmatter is present, the entire content is treated as the body
// and the name is derived from the source path.
func ParseSkill(data []byte, source string, priority int) (*Skill, error) {
	content := string(data)
	trimmed := strings.TrimSpace(content)

	skill := &Skill{
		Source:   source,
		Priority: priority,
	}

	if strings.HasPrefix(trimmed, "---") {
		// Find the closing delimiter
		rest := trimmed[3:]
		// Skip the newline after opening ---
		if idx := strings.Index(rest, "\n"); idx >= 0 {
			rest = rest[idx+1:]
		} else {
			// Only --- on the line, no content
			rest = ""
		}

		closeIdx := strings.Index(rest, "---")
		if closeIdx < 0 {
			// No closing delimiter — treat everything as body
			skill.Content = strings.TrimSpace(trimmed)
			skill.Name = SkillNameFromPath(source)
			return skill, nil
		}

		frontmatterYAML := rest[:closeIdx]
		body := rest[closeIdx+3:]
		// Skip the newline after closing ---
		if len(body) > 0 && body[0] == '\n' {
			body = body[1:]
		}

		var fm skillFrontmatter
		if err := yaml.Unmarshal([]byte(frontmatterYAML), &fm); err != nil {
			return nil, fmt.Errorf("parse frontmatter in %s: %w", source, err)
		}

		skill.Name = fm.Name
		skill.Description = fm.Description
		skill.Model = fm.Model
		skill.Timeout = fm.Timeout
		skill.AllowedTools = fm.AllowedTools
		skill.Content = strings.TrimSpace(body)
	} else {
		skill.Content = strings.TrimSpace(trimmed)
	}

	if skill.Name == "" {
		skill.Name = SkillNameFromPath(source)
	}

	return skill, nil
}

// ParseSkillFile reads a SKILL.md file from disk and parses it.
func ParseSkillFile(path string, priority int) (*Skill, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill file %s: %w", path, err)
	}
	return ParseSkill(data, path, priority)
}

// SkillNameFromPath extracts the skill name from a SKILL.md file path
// by returning the name of the parent directory.
func SkillNameFromPath(path string) string {
	dir := filepath.Dir(path)
	return filepath.Base(dir)
}
