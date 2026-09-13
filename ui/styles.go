package ui

import "github.com/charmbracelet/lipgloss"

type Pane int

const (
	PaneEditor Pane = iota
	PaneStatus
	PaneConsole
)

type Styles struct {
	Editor  lipgloss.Style
	Status  lipgloss.Style
	Console lipgloss.Style
	Help    lipgloss.Style
	Warning lipgloss.Style
	Prompt  lipgloss.Style

	focusBorder lipgloss.Color
}

func NewStyles(theme Theme) Styles {
	base := lipgloss.NewStyle().
		BorderForeground(theme.Border).
		Padding(0, 1)

	return Styles{
		Editor: base.
			Border(lipgloss.RoundedBorder()).
			Padding(0, 0),
		Status: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		Console: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Border).
			Padding(0, 1),
		Help: lipgloss.NewStyle().
			Foreground(theme.Muted).
			Padding(0, 1),
		Warning: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.Error).
			Foreground(theme.Foreground).
			Padding(0, 2),
		Prompt: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(theme.FocusBorder).
			Foreground(theme.Foreground).
			Padding(0, 2),

		focusBorder: theme.FocusBorder,
	}
}

// PaneStyle returns the border style for the given pane, highlighting
// the border when the pane holds focus.
func (s Styles) PaneStyle(p Pane, focused bool) lipgloss.Style {
	switch p {
	case PaneEditor:
		return s.withFocus(s.Editor, focused)
	case PaneStatus:
		return s.withFocus(s.Status, focused)
	case PaneConsole:
		return s.withFocus(s.Console, focused)
	}
	return lipgloss.NewStyle()
}

func (s Styles) withFocus(style lipgloss.Style, focused bool) lipgloss.Style {
	if !focused {
		return style
	}
	return style.BorderForeground(s.focusBorder)
}
