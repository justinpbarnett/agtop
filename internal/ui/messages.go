package ui

import "github.com/justinpbarnett/agtop/internal/ui/panels"

// Type aliases to panels message types — single source of truth.

// RunStoreUpdatedMsg is sent when any run in the store changes.
type RunStoreUpdatedMsg = panels.RunStoreUpdatedMsg

// LogLineMsg is sent when new log content is available for a run.
type LogLineMsg = panels.LogLineMsg

// CloseModalMsg signals that the modal should be closed.
type CloseModalMsg = panels.CloseModalMsg
