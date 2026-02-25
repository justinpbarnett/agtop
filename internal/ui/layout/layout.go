package layout

// Layout holds the computed pixel dimensions for all panels.
//
// The layout arranges panels as:
//
//	┌─RunList──┬──LogView──┐
//	│          │           │
//	├─Detail───┤           │
//	│          │           │
//	└──────StatusBar───────┘
//
// RunList and Detail share the left column width.
// LogView spans the full usable height on the right.
type Layout struct {
	TermWidth  int
	TermHeight int
	TooSmall   bool

	// Left column — top
	RunListWidth  int
	RunListHeight int

	// Left column — bottom
	DetailWidth  int
	DetailHeight int

	// Right column — full height
	LogViewWidth  int
	LogViewHeight int

	// Status bar
	StatusBarWidth int
}

const (
	MinWidth  = 80
	MinHeight = 24

	LeftColWidth     = 30
	RunListRowWeight = 0.50
)

// Calculate computes panel dimensions from terminal size.
// Subtracts 1 row for the status bar before splitting.
// Returns Layout with TooSmall=true if under minimum.
func Calculate(termWidth, termHeight int) Layout {
	l := Layout{
		TermWidth:  termWidth,
		TermHeight: termHeight,
	}

	if termWidth < MinWidth || termHeight < MinHeight {
		l.TooSmall = true
		return l
	}

	usableHeight := termHeight - 1 // status bar

	leftWidth := LeftColWidth
	rightWidth := termWidth - leftWidth

	runListHeight := int(float64(usableHeight) * RunListRowWeight)
	detailHeight := usableHeight - runListHeight

	l.RunListWidth = leftWidth
	l.RunListHeight = runListHeight
	l.DetailWidth = leftWidth
	l.DetailHeight = detailHeight

	l.LogViewWidth = rightWidth
	l.LogViewHeight = usableHeight

	l.StatusBarWidth = termWidth

	return l
}
