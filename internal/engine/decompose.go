package engine

import (
	"encoding/json"
	"fmt"
	"sort"
)

type DecomposeResult struct {
	Tasks []DecomposeTask `json:"tasks"`
}

type DecomposeTask struct {
	Name          string   `json:"name"`
	ParallelGroup string   `json:"parallel_group"`
	Dependencies  []string `json:"dependencies"`
}

// ParseDecomposeResult parses the JSON output from a decompose skill
// into structured tasks with parallel groups and dependencies.
func ParseDecomposeResult(jsonText string) (*DecomposeResult, error) {
	var result DecomposeResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("parse decompose result: %w", err)
	}
	return &result, nil
}

// GroupByParallel returns tasks grouped by parallel group, ordered
// so that groups with no unresolved dependencies come first.
// Tasks without a parallel group are each placed in their own group.
func (d *DecomposeResult) GroupByParallel() [][]DecomposeTask {
	if len(d.Tasks) == 0 {
		return nil
	}

	// Group tasks by parallel_group
	groups := make(map[string][]DecomposeTask)
	var groupOrder []string
	seen := make(map[string]bool)

	for i, task := range d.Tasks {
		group := task.ParallelGroup
		if group == "" {
			// Tasks without a group get a unique key
			group = fmt.Sprintf("_solo_%d", i)
		}
		if !seen[group] {
			seen[group] = true
			groupOrder = append(groupOrder, group)
		}
		groups[group] = append(groups[group], task)
	}

	// Build a set of all task names per group for dependency resolution
	groupForTask := make(map[string]string)
	for _, task := range d.Tasks {
		group := task.ParallelGroup
		if group == "" {
			for _, g := range groupOrder {
				for _, t := range groups[g] {
					if t.Name == task.Name {
						group = g
						break
					}
				}
				if group != "" {
					break
				}
			}
		}
		groupForTask[task.Name] = group
	}

	// Build group-level dependency graph
	groupDeps := make(map[string]map[string]bool)
	for _, g := range groupOrder {
		groupDeps[g] = make(map[string]bool)
	}
	for _, task := range d.Tasks {
		tGroup := groupForTask[task.Name]
		for _, dep := range task.Dependencies {
			dGroup := groupForTask[dep]
			if dGroup != "" && dGroup != tGroup {
				groupDeps[tGroup][dGroup] = true
			}
		}
	}

	// Topological sort of groups
	var sorted []string
	resolved := make(map[string]bool)
	var visit func(g string)
	visited := make(map[string]bool)

	visit = func(g string) {
		if resolved[g] || visited[g] {
			return
		}
		visited[g] = true
		for dep := range groupDeps[g] {
			visit(dep)
		}
		resolved[g] = true
		sorted = append(sorted, g)
	}

	// Sort groupOrder for deterministic output
	sort.Strings(groupOrder)
	for _, g := range groupOrder {
		visit(g)
	}

	var result [][]DecomposeTask
	for _, g := range sorted {
		result = append(result, groups[g])
	}
	return result
}
