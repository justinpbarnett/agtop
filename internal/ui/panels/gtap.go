package panels

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const gTimeout = 300 * time.Millisecond

// GTimerExpiredMsg is sent when a double-tap "gg" window expires.
// ID identifies which panel's timer fired.
type GTimerExpiredMsg struct{ ID int }

// Unique panel IDs for double-tap timer disambiguation.
const (
	gTapIDLogView = 1
	gTapIDDiffView = 2
	gTapIDDetail   = 3
)

// DoubleTap tracks state for the "gg" double-tap navigation pattern.
type DoubleTap struct {
	Pending bool
	id      int
}

// NewDoubleTap creates a DoubleTap with the given panel ID.
func NewDoubleTap(id int) DoubleTap {
	return DoubleTap{id: id}
}

// Check handles a "g" keypress. Returns fired=true if this is the second tap,
// or a timer cmd if this is the first tap (to start the timeout window).
func (dt *DoubleTap) Check() (fired bool, cmd tea.Cmd) {
	if dt.Pending {
		dt.Pending = false
		return true, nil
	}
	dt.Pending = true
	id := dt.id
	return false, tea.Tick(gTimeout, func(time.Time) tea.Msg {
		return GTimerExpiredMsg{ID: id}
	})
}

// HandleExpiry clears Pending if the expired message belongs to this panel.
// Returns true if the message was consumed.
func (dt *DoubleTap) HandleExpiry(msg GTimerExpiredMsg) bool {
	if msg.ID == dt.id {
		dt.Pending = false
		return true
	}
	return false
}
