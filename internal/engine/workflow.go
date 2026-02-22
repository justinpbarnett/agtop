package engine

import (
	"fmt"

	"github.com/justinpbarnett/agtop/internal/config"
)

type Workflow struct {
	Name   string
	Skills []string
}

// ResolveWorkflow returns the ordered list of skill names for a workflow.
// Returns an error if the workflow name is not found in config.
func ResolveWorkflow(cfg *config.Config, workflowName string) ([]string, error) {
	wf, ok := cfg.Workflows[workflowName]
	if !ok {
		return nil, fmt.Errorf("unknown workflow: %q", workflowName)
	}
	if len(wf.Skills) == 0 {
		return nil, fmt.Errorf("workflow %q has no skills", workflowName)
	}
	return wf.Skills, nil
}

// ValidateWorkflow checks that every skill in the workflow is available
// in the registry. Returns the names of any missing skills.
func ValidateWorkflow(skills []string, reg *Registry) []string {
	var missing []string
	for _, name := range skills {
		if _, ok := reg.Get(name); !ok {
			missing = append(missing, name)
		}
	}
	return missing
}
