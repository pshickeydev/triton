package status

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pshickeydev/triton/ui"
)

const (
	panelTitle   = " triton "
	labelStreams = " streams: "
	labelCPS     = " cps:     "
	labelGhci    = " ghci:    "
	ghciRunning  = "● running"
	ghciStopped  = "● stopped"
	cpsFormat    = "%.3f"
	editedMarker = "*"
)

type Panel struct {
	tracker *Tracker
	palette ui.Palette
	width   int
	height  int
}

func NewPanel(tracker *Tracker, palette ui.Palette) *Panel {
	return &Panel{
		tracker: tracker,
		palette: palette,
	}
}

func (p *Panel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p Panel) View() string {
	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(p.palette.Label)
	labelStyle := lipgloss.NewStyle().Foreground(p.palette.Label)
	activeStyle := lipgloss.NewStyle().Foreground(p.palette.Active)
	mutedStyle := lipgloss.NewStyle().Foreground(p.palette.Muted)
	soloedStyle := lipgloss.NewStyle().Foreground(p.palette.Soloed).Bold(true)
	idleStyle := lipgloss.NewStyle().Foreground(p.palette.Idle)
	editedStyle := lipgloss.NewStyle().Foreground(p.palette.Edited)
	ghciOKStyle := lipgloss.NewStyle().Foreground(p.palette.GhciOK)
	ghciErrStyle := lipgloss.NewStyle().Foreground(p.palette.GhciErr)

	b.WriteString(titleStyle.Render(panelTitle))
	b.WriteString("\n\n")

	streams := p.tracker.Streams()
	edited := p.tracker.Edited()
	b.WriteString(labelStyle.Render(labelStreams))
	for i := 1; i <= len(streams); i++ {
		state := streams[i]
		label := fmt.Sprintf("d%d", i)
		var s lipgloss.Style
		switch state {
		case StreamActive:
			s = activeStyle
		case StreamMuted:
			s = mutedStyle
		case StreamSoloed:
			s = soloedStyle
		default:
			s = idleStyle
		}
		b.WriteString(s.Render(label))
		if edited[i] {
			b.WriteString(editedStyle.Render(editedMarker))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n")

	cps := p.tracker.CPS()
	b.WriteString(labelStyle.Render(labelCPS))
	if cps > 0 {
		b.WriteString(fmt.Sprintf(cpsFormat, cps))
	} else {
		b.WriteString("--")
	}
	b.WriteString("\n")

	b.WriteString(labelStyle.Render(labelGhci))
	if p.tracker.GhciOK() {
		b.WriteString(ghciOKStyle.Render(ghciRunning))
	} else {
		b.WriteString(ghciErrStyle.Render(ghciStopped))
	}
	b.WriteString("\n")

	return b.String()
}
