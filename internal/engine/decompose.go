package engine

type DecomposeResult struct {
	Tasks []DecomposeTask
}

type DecomposeTask struct {
	Name          string
	ParallelGroup string
	Dependencies  []string
}
