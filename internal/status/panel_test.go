package status

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/pshickeydev/triton/internal/ui"
)

func TestPanel_PlaysWithoutEdits(t *testing.T) {
	tr := NewTracker(2)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")

	view := ansi.Strip(NewPanel(tr, ui.DarkTheme().Palette).View())
	if !strings.Contains(view, "d1 ") {
		t.Errorf("playing stream should render without edited marker, got %q", view)
	}
}

func TestPanel_MarksEditedStreams(t *testing.T) {
	tr := NewTracker(2)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetPlaying(2, "d2 $ sound \"cp\"")
	tr.SetEdited(2)

	view := ansi.Strip(NewPanel(tr, ui.DarkTheme().Palette).View())
	if !strings.Contains(view, "d2"+editedMarker) {
		t.Errorf("edited stream should render with %q marker, got %q", editedMarker, view)
	}
	if strings.Contains(view, "d1"+editedMarker) {
		t.Errorf("in-sync stream should not render edited marker, got %q", view)
	}
}

func TestPanel_MutedAndEditedCoexist(t *testing.T) {
	tr := NewTracker(2)
	tr.SetPlaying(1, "d1 $ sound \"bd\"")
	tr.SetMuted(1)
	tr.SetEdited(1)

	view := ansi.Strip(NewPanel(tr, ui.DarkTheme().Palette).View())
	if !strings.Contains(view, "d1"+editedMarker) {
		t.Errorf("muted stream with edits should still show edited marker, got %q", view)
	}
}

// A soloed stream shows as soloed even when it is also muted: solo
// takes precedence in Tidal.
func TestPanel_MutedSoloedShowsSolo(t *testing.T) {
	tr := NewTracker(2)
	tr.SetMuted(1)
	tr.SetSoloed(1)

	view := ansi.Strip(NewPanel(tr, ui.DarkTheme().Palette).View())
	if !strings.Contains(view, "d1 ") {
		t.Errorf("muted+soloed stream should render without edited marker, got %q", view)
	}
}

// The panel lists only the configured number of streams, even when
// hidden streams carry tracked state.
func TestPanel_ShowsOnlyVisibleStreams(t *testing.T) {
	tr := NewTracker(2)
	tr.SetPlaying(5, `d5 $ sound "bd"`)

	view := ansi.Strip(NewPanel(tr, ui.DarkTheme().Palette).View())
	if strings.Contains(view, "d5") {
		t.Errorf("hidden stream should not render in the panel, got %q", view)
	}
}
