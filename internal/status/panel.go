package status

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/pshickeydev/triton/internal/ui"
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

// panelStyles holds the pre-built row styles so View does not construct
// them (and parse the same colors) again on every frame.
type panelStyles struct {
	title   lipgloss.Style
	label   lipgloss.Style
	active  lipgloss.Style
	muted   lipgloss.Style
	soloed  lipgloss.Style
	idle    lipgloss.Style
	edited  lipgloss.Style
	ghciOK  lipgloss.Style
	ghciErr lipgloss.Style
}

func newPanelStyles(p ui.Palette) panelStyles {
	return panelStyles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(p.Label),
		label:   lipgloss.NewStyle().Foreground(p.Label),
		active:  lipgloss.NewStyle().Foreground(p.Active),
		muted:   lipgloss.NewStyle().Foreground(p.Muted),
		soloed:  lipgloss.NewStyle().Foreground(p.Soloed).Bold(true),
		idle:    lipgloss.NewStyle().Foreground(p.Idle),
		edited:  lipgloss.NewStyle().Foreground(p.Edited),
		ghciOK:  lipgloss.NewStyle().Foreground(p.GhciOK),
		ghciErr: lipgloss.NewStyle().Foreground(p.GhciErr),
	}
}

type Panel struct {
	tracker *Tracker
	palette ui.Palette
	styles  panelStyles
	width   int
	height  int
}

func NewPanel(tracker *Tracker, palette ui.Palette) *Panel {
	return &Panel{
		tracker: tracker,
		palette: palette,
		styles:  newPanelStyles(palette),
	}
}

func (p *Panel) SetSize(w, h int) {
	p.width = w
	p.height = h
}

func (p Panel) View() string {
	var b strings.Builder
	styles := p.styles

	b.WriteString(styles.title.Render(panelTitle))
	b.WriteString("\n\n")

	streams := p.tracker.Streams()
	edited := p.tracker.Edited()
	b.WriteString(styles.label.Render(labelStreams))
	for i := 1; i <= len(streams); i++ {
		state := streams[i]
		label := fmt.Sprintf("d%d", i)
		var s lipgloss.Style
		switch state {
		case StreamActive:
			s = styles.active
		case StreamMuted:
			s = styles.muted
		case StreamSoloed:
			s = styles.soloed
		default:
			s = styles.idle
		}
		b.WriteString(s.Render(label))
		if edited[i] {
			b.WriteString(styles.edited.Render(editedMarker))
		}
		b.WriteString(" ")
	}
	b.WriteString("\n")

	cps := p.tracker.CPS()
	b.WriteString(styles.label.Render(labelCPS))
	if cps > 0 {
		b.WriteString(fmt.Sprintf(cpsFormat, cps))
	} else {
		b.WriteString("--")
	}
	b.WriteString("\n")

	b.WriteString(styles.label.Render(labelGhci))
	if p.tracker.GhciOK() {
		b.WriteString(styles.ghciOK.Render(ghciRunning))
	} else {
		b.WriteString(styles.ghciErr.Render(ghciStopped))
	}
	b.WriteString("\n")

	return b.String()
}
