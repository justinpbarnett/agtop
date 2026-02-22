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
