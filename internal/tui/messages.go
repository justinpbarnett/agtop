package tui

// RunStoreUpdatedMsg is sent when any run in the store changes.
// Components that care about run data re-read the store on this message.
type RunStoreUpdatedMsg struct{}
