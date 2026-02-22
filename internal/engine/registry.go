package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

type SkillSource struct {
	Dir      string
	Priority int
}

type Registry struct {
	skills map[string]*Skill
	cfg    *config.Config
}

func NewRegistry(cfg *config.Config) *Registry {
	return &Registry{
		skills: make(map[string]*Skill),
		cfg:    cfg,
	}
}

// Load discovers SKILL.md files from the six precedence-ordered directories,
// loads them, and resolves name conflicts (highest precedence wins).
// Directories are scanned in reverse precedence order (lowest priority first)
// so that simple map assignment handles overrides — later writes win.
func (r *Registry) Load(projectRoot string) error {
	home, _ := os.UserHomeDir()

	binaryDir := ""
	if exe, err := os.Executable(); err == nil {
		binaryDir = filepath.Dir(exe)
	}

	// Build sources in reverse precedence order (lowest priority first).
	// Later entries overwrite earlier ones in the map.
	sources := []SkillSource{}

	if binaryDir != "" {
		sources = append(sources, SkillSource{
			Dir:      filepath.Join(binaryDir, "skills"),
			Priority: PriorityBuiltIn,
		})
	}
	if home != "" {
		sources = append(sources, SkillSource{
			Dir:      filepath.Join(home, ".claude", "skills"),
			Priority: PriorityUserClaude,
		})
		sources = append(sources, SkillSource{
			Dir:      filepath.Join(home, ".config", "agtop", "skills"),
			Priority: PriorityUserAgtop,
		})
	}
	sources = append(sources,
		SkillSource{Dir: filepath.Join(projectRoot, ".agents", "skills"), Priority: PriorityProjectAgents},
		SkillSource{Dir: filepath.Join(projectRoot, ".claude", "skills"), Priority: PriorityProjectClaude},
		SkillSource{Dir: filepath.Join(projectRoot, ".agtop", "skills"), Priority: PriorityProjectAgtop},
	)

	for _, src := range sources {
		r.loadFromDir(src.Dir, src.Priority)
	}

	r.mergeConfig()
	return nil
}

func (r *Registry) loadFromDir(dir string, priority int) {
	pattern := filepath.Join(dir, "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, path := range matches {
		skill, err := ParseSkillFile(path, priority)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			continue
		}
		r.skills[skill.Name] = skill
	}
}

func (r *Registry) mergeConfig() {
	for name, skill := range r.skills {
		sc, ok := r.cfg.Skills[name]
		if !ok {
			continue
		}
		if sc.Model != "" {
			skill.Model = sc.Model
		}
		if sc.Timeout > 0 {
			skill.Timeout = sc.Timeout
		}
		if sc.Parallel {
			skill.Parallel = sc.Parallel
		}
		if len(sc.AllowedTools) > 0 {
			skill.AllowedTools = sc.AllowedTools
		}
	}
}

// Get returns the skill with the given name.
func (r *Registry) Get(name string) (*Skill, bool) {
	s, ok := r.skills[name]
	return s, ok
}

// List returns all loaded skills sorted by name.
func (r *Registry) List() []*Skill {
	result := make([]*Skill, 0, len(r.skills))
	for _, s := range r.skills {
		result = append(result, s)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

// Names returns all skill names sorted alphabetically.
func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.skills))
	for name := range r.skills {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// SkillForRun returns the skill and fully-resolved RunOptions for a given
// skill name. Model resolution order: skill config → skill frontmatter →
// runtime default. WorkDir is NOT set — the caller provides it.
// The runtime config used depends on config.Runtime.Default.
func (r *Registry) SkillForRun(name string) (*Skill, runtime.RunOptions, bool) {
	skill, ok := r.skills[name]
	if !ok {
		return nil, runtime.RunOptions{}, false
	}

	if r.cfg.Runtime.Default == "opencode" {
		return r.skillForOpenCode(skill)
	}
	return r.skillForClaude(skill)
}

func (r *Registry) skillForClaude(skill *Skill) (*Skill, runtime.RunOptions, bool) {
	model := r.cfg.Runtime.Claude.Model
	if skill.Model != "" {
		model = skill.Model
	}

	allowedTools := r.cfg.Runtime.Claude.AllowedTools
	if len(skill.AllowedTools) > 0 {
		allowedTools = skill.AllowedTools
	}

	opts := runtime.RunOptions{
		Model:          model,
		AllowedTools:   allowedTools,
		MaxTurns:       r.cfg.Runtime.Claude.MaxTurns,
		PermissionMode: r.cfg.Runtime.Claude.PermissionMode,
	}
	return skill, opts, true
}

func (r *Registry) skillForOpenCode(skill *Skill) (*Skill, runtime.RunOptions, bool) {
	model := r.cfg.Runtime.OpenCode.Model
	if skill.Model != "" {
		model = skill.Model
	}

	opts := runtime.RunOptions{
		Model: model,
		Agent: r.cfg.Runtime.OpenCode.Agent,
	}
	return skill, opts, true
}
