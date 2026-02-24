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
	Images   []string // paths to temp image files pasted into the modal
}

// YankMsg is sent when text has been yanked (copied) from a panel.
type YankMsg struct {
	Text string
}

// InitAcceptedMsg signals the user accepted the init prompt.
type InitAcceptedMsg struct{}

// AnimTickMsg is sent at a fast interval to drive spinner/animation frames.
type AnimTickMsg struct{}

// UpdateAvailableMsg is sent when a newer version is available on GitHub.
type UpdateAvailableMsg struct {
	Version string
}

// FullscreenMsg requests that a panel be expanded to fill the terminal.
type FullscreenMsg struct{ Panel int }

// ExitFullscreenMsg requests returning to the normal 3-panel layout.
type ExitFullscreenMsg struct{}

// SelectRunMsg is sent when the user picks a run from the run picker dropdown.
type SelectRunMsg struct {
	RunID string
}
