package tui

// RunStoreUpdatedMsg is sent when any run in the store changes.
// Components that care about run data re-read the store on this message.
type RunStoreUpdatedMsg struct{}

// LogLineMsg is sent when new log content is available for a run.
// The log viewer re-reads the ring buffer for the specified run.
type LogLineMsg struct {
	RunID string
}
