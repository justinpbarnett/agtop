package ui

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Top       key.Binding
	Bottom    key.Binding
	GPrefix   key.Binding
	Left      key.Binding
	Right     key.Binding
	FocusNext key.Binding
	Filter    key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("k", "up"),
			key.WithHelp("k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("j", "down"),
			key.WithHelp("j", "down"),
		),
		Top: key.NewBinding(
			key.WithHelp("gg", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys("G"),
			key.WithHelp("G", "bottom"),
		),
		GPrefix: key.NewBinding(
			key.WithKeys("g"),
		),
		Left: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("h", "panel left"),
		),
		Right: key.NewBinding(
			key.WithKeys("l", "right"),
			key.WithHelp("l", "panel right"),
		),
		FocusNext: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next panel"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
	}
}
