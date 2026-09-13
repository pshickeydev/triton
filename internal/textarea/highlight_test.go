package textarea

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newTestTextarea(t *testing.T, value string) Model {
	t.Helper()
	ta := New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetWidth(40)
	ta.SetHeight(10)
	ta.SetValue(value)
	return ta
}

func TestHighlightedRangesSingleLine(t *testing.T) {
	ta := newTestTextarea(t, `d1 $ sound "bd sn"`)

	ta.SetHighlights([]Highlight{{
		Line: 1, EndLine: 1, Start: 12, End: 14,
		Style: lipgloss.NewStyle().Reverse(true),
	}})
	ranges := ta.highlightedRanges()
	got := ranges[0]
	if len(got) != 1 {
		t.Fatalf("expected 1 range, got %d", len(got))
	}
	if got[0].start != 11 || got[0].end != 13 {
		t.Errorf("range: got [%d,%d), want [11,13)", got[0].start, got[0].end)
	}

	// End past the line end is clamped.
	ta.SetHighlights([]Highlight{{
		Line: 1, EndLine: 1, Start: 12, End: 99,
		Style: lipgloss.NewStyle().Reverse(true),
	}})
	got = ta.highlightedRanges()[0]
	if len(got) != 1 || got[0].start != 11 || got[0].end != 18 {
		t.Errorf("clamped range: got [%d,%d), want [11,18)", got[0].start, got[0].end)
	}
}

func TestHighlightedRangesMultiLine(t *testing.T) {
	ta := newTestTextarea(t, "d1 $ sound \"bd\n~ sn\"")

	ta.SetHighlights([]Highlight{{
		Line: 1, EndLine: 2, Start: 12, End: 4,
		Style: lipgloss.NewStyle().Reverse(true),
	}})

	ranges := ta.highlightedRanges()
	// First line: from column 12 (rune 11) to the end of the line.
	if r := ranges[0]; len(r) != 1 || r[0].start != 11 || r[0].end != 14 {
		t.Errorf("line 1 range: %+v", r)
	}
	// Second line: from column 1 (rune 0) to column 4 (rune 3).
	if r := ranges[1]; len(r) != 1 || r[0].start != 0 || r[0].end != 3 {
		t.Errorf("line 2 range: %+v", r)
	}
}

func TestHighlightedRangesOffBuffer(t *testing.T) {
	ta := newTestTextarea(t, "hush")

	ta.SetHighlights([]Highlight{{
		Line: 5, EndLine: 5, Start: 1, End: 3,
		Style: lipgloss.NewStyle().Reverse(true),
	}})

	if ranges := ta.highlightedRanges(); len(ranges) != 0 {
		t.Errorf("highlight past the buffer should be dropped, got %+v", ranges)
	}
}

func TestViewWithHighlightsKeepsText(t *testing.T) {
	value := "d1 $ sound \"bd sn\"\nd2 $ sound \"cp\""
	plain := newTestTextarea(t, value)
	spanned := newTestTextarea(t, value)
	spanned.SetHighlights([]Highlight{
		{Line: 1, EndLine: 1, Start: 12, End: 14, Style: lipgloss.NewStyle().Reverse(true)},
		{Line: 2, EndLine: 2, Start: 12, End: 14, Style: lipgloss.NewStyle().Reverse(true)},
	})

	want := stripANSI(plain.View())
	got := stripANSI(spanned.View())
	if got != want {
		t.Errorf("highlighted view text changed:\n got: %q\nwant: %q", got, want)
	}
}

func TestViewHighlightAcrossWrap(t *testing.T) {
	ta := New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetWidth(10)
	ta.SetHeight(10)
	ta.SetValue("d1 $ sound \"bd sn\"")

	ta.SetHighlights([]Highlight{{
		Line: 1, EndLine: 1, Start: 9, End: 15,
		Style: lipgloss.NewStyle().Reverse(true),
	}})

	got := stripANSI(ta.View())
	want := stripANSI(func() string {
		plain := New()
		plain.Prompt = ""
		plain.ShowLineNumbers = false
		plain.SetWidth(10)
		plain.SetHeight(10)
		plain.SetValue("d1 $ sound \"bd sn\"")
		return plain.View()
	}())
	if got != want {
		t.Errorf("wrapped highlighted view text changed:\n got: %q\nwant: %q", got, want)
	}
}

func TestYOffset(t *testing.T) {
	ta := New()
	ta.Prompt = ""
	ta.ShowLineNumbers = false
	ta.SetWidth(40)
	ta.SetHeight(2)
	ta.Focus()
	ta.SetValue("one\ntwo\nthree\nfour\nfive")
	ta.View()

	for range 6 {
		ta, _ = ta.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if ta.YOffset() <= 0 {
		t.Errorf("expected viewport to scroll below the fold, YOffset = %d", ta.YOffset())
	}
}

// stripANSI removes ANSI escape sequences so rendered views can be
// compared as plain text.
func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEscape = true
		case inEscape:
			if r == 'm' {
				inEscape = false
			}
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
