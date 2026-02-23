package ui

import "github.com/justinpbarnett/agtop/internal/ui/panels"

// Type aliases to panels message types — single source of truth.

// RunStoreUpdatedMsg is sent when any run in the store changes.
type RunStoreUpdatedMsg = panels.RunStoreUpdatedMsg

// LogLineMsg is sent when new log content is available for a run.
type LogLineMsg = panels.LogLineMsg

// CloseModalMsg signals that the modal should be closed.
type CloseModalMsg = panels.CloseModalMsg

// ClearFlashMsg signals the status bar flash should be cleared.
type ClearFlashMsg = panels.ClearFlashMsg

// DiffResultMsg delivers async diff results for a run.
type DiffResultMsg = panels.DiffResultMsg

// SubmitNewRunMsg is sent when the user confirms the new run modal.
type SubmitNewRunMsg = panels.SubmitNewRunMsg

// YankMsg is sent when text has been yanked (copied) from a panel.
type YankMsg = panels.YankMsg

// InitAcceptedMsg signals the user accepted the init prompt.
type InitAcceptedMsg = panels.InitAcceptedMsg

// SubmitFollowUpMsg is sent when the user confirms the follow-up modal.
type SubmitFollowUpMsg = panels.SubmitFollowUpMsg

// UpdateAvailableMsg is sent when a newer version is available.
type UpdateAvailableMsg = panels.UpdateAvailableMsg
