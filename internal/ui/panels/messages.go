package panels

// RunStoreUpdatedMsg is sent when any run in the store changes.
type RunStoreUpdatedMsg struct{}

// LogLineMsg is sent when new log content is available for a run.
type LogLineMsg struct {
	RunID string
}

// CloseModalMsg signals that the modal should be closed.
type CloseModalMsg struct{}

// ClearFlashMsg signals the status bar flash should be cleared.
type ClearFlashMsg struct{}

// DiffResultMsg delivers async diff results for a run.
type DiffResultMsg struct {
	RunID    string
	Diff     string
	DiffStat string
	Err      error
}

// SubmitNewRunMsg is sent when the user confirms the new run modal.
type SubmitNewRunMsg struct {
	Prompt   string
	Workflow string
	Model    string
}

// YankMsg is sent when text has been yanked (copied) from a panel.
type YankMsg struct {
	Text string
}

// InitAcceptedMsg signals the user accepted the init prompt.
type InitAcceptedMsg struct{}
