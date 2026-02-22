package layout

// Layout holds the computed pixel dimensions for all panels.
type Layout struct {
	TermWidth  int
	TermHeight int
	TooSmall   bool

	// Top row panels
	RunListWidth  int
	RunListHeight int
	LogViewWidth  int
	LogViewHeight int

	// Bottom row panel
	DetailWidth  int
	DetailHeight int

	// Status bar
	StatusBarWidth int
}

const (
	MinWidth  = 80
	MinHeight = 24

	TopRowWeight    = 0.65
	BottomRowWeight = 0.35
	LeftColWeight   = 0.40
	RightColWeight  = 0.60
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

	topRowHeight := int(float64(usableHeight) * TopRowWeight)
	bottomRowHeight := usableHeight - topRowHeight

	runListWidth := int(float64(termWidth) * LeftColWeight)
	logViewWidth := termWidth - runListWidth

	l.RunListWidth = runListWidth
	l.RunListHeight = topRowHeight
	l.LogViewWidth = logViewWidth
	l.LogViewHeight = topRowHeight

	l.DetailWidth = termWidth
	l.DetailHeight = bottomRowHeight

	l.StatusBarWidth = termWidth

	return l
}
