package textarea

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

func keyMsg(kind tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg{Type: kind}
}

func areaWithLines(t *testing.T, text string, width, height int) Model {
	t.Helper()
	m := New()
	m.Prompt = ""
	m.Placeholder = ""
	m.ShowLineNumbers = false
	m.SetWidth(width)
	m.SetHeight(height)
	m.SetValue(text)
	m.Focus()
	// Render once so the viewport knows the row count, as the app does after
	// every update.
	_ = m.View()
	return m
}

// step drives one message through the update loop and renders, which is what
// the program does for every message it handles.
func step(m Model, msg tea.Msg) Model {
	m, _ = m.Update(msg)
	_ = m.View()
	return m
}

// Editing keys must advance the revision; navigation keys must not, or the
// editor would re-segment the buffer on every keystroke that changed nothing.
func TestRevisionTracksEdits(t *testing.T) {
	doc := "d1 $ sound \"bd sn\"\nd2 $ sound \"hh*4\"\n"
	edits := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("x")},
		keyMsg(tea.KeyEnter),
		keyMsg(tea.KeyBackspace),
		keyMsg(tea.KeyDelete),
		keyMsg(tea.KeyCtrlK),
		keyMsg(tea.KeyCtrlU),
		keyMsg(tea.KeyCtrlW),
		keyMsg(tea.KeyCtrlT),
	}
	for _, msg := range edits {
		m := areaWithLines(t, doc, 40, 4)
		m = step(m, msg)
		if m.Revision() == 0 {
			t.Errorf("%v changed the buffer but did not advance the revision", msg)
		}
	}

	moves := []tea.KeyMsg{
		keyMsg(tea.KeyLeft),
		keyMsg(tea.KeyRight),
		keyMsg(tea.KeyUp),
		keyMsg(tea.KeyDown),
		keyMsg(tea.KeyHome),
		keyMsg(tea.KeyEnd),
		keyMsg(tea.KeyCtrlA),
		keyMsg(tea.KeyCtrlE),
		keyMsg(tea.KeyCtrlHome),
		keyMsg(tea.KeyCtrlEnd),
		{Type: tea.KeyLeft, Alt: true},
		{Type: tea.KeyRight, Alt: true},
	}
	for _, msg := range moves {
		m := areaWithLines(t, doc, 40, 4)
		before := m.Revision()
		m = step(m, msg)
		m = step(m, msg)
		if m.Revision() != before {
			t.Errorf("%v only moved the cursor but advanced the revision %d -> %d", msg, before, m.Revision())
		}
	}
}

// The revision counter is what tells the editor's callers that the buffer
// changed, so every mutating key binding must advance it and no others.
func TestRevisionMatchesContentChange(t *testing.T) {
	fixture := "d1 $ sound \"bd sn\" # gain 0.9\n\n-- comment\n\nd2 $ s \"hh*4\" ~ bass\n"
	bindings := []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyRunes, Runes: []rune(" ")},
		keyMsg(tea.KeyEnter),
		keyMsg(tea.KeyTab),
		keyMsg(tea.KeyBackspace),
		keyMsg(tea.KeyDelete),
		keyMsg(tea.KeyCtrlK),
		keyMsg(tea.KeyCtrlU),
		keyMsg(tea.KeyCtrlW),
		{Type: tea.KeyRunes, Runes: []rune("d"), Alt: true},
		keyMsg(tea.KeyCtrlT),
		{Type: tea.KeyRunes, Runes: []rune("u"), Alt: true},
		{Type: tea.KeyRunes, Runes: []rune("l"), Alt: true},
		{Type: tea.KeyRunes, Runes: []rune("c"), Alt: true},
		keyMsg(tea.KeyCtrlA),
		keyMsg(tea.KeyCtrlE),
		keyMsg(tea.KeyCtrlHome),
		keyMsg(tea.KeyCtrlEnd),
		keyMsg(tea.KeyLeft),
		keyMsg(tea.KeyRight),
		keyMsg(tea.KeyUp),
		keyMsg(tea.KeyDown),
		{Type: tea.KeyLeft, Alt: true},
		{Type: tea.KeyRight, Alt: true},
		keyMsg(tea.KeyHome),
		keyMsg(tea.KeyEnd),
		keyMsg(tea.KeyEsc),
	}

	// Start from every cursor position in the fixture, not just the end.
	type cell struct{ row, col int }
	var cells []cell
	for row, line := range strings.Split(fixture, "\n") {
		for col := 0; col <= len([]rune(line)); col++ {
			cells = append(cells, cell{row, col})
		}
	}

	for _, at := range cells {
		for _, msg := range bindings {
			m := areaWithLines(t, fixture, 30, 4)
			m.row, m.col = at.row, at.col
			before, valueBefore := m.Revision(), m.Value()
			m = step(m, msg)
			edited := m.Value() != valueBefore
			if bumped := m.Revision() != before; bumped != edited {
				t.Fatalf("cursor %+v, key %v: content changed=%v revision changed=%v\nbefore:%q\nafter: %q",
					at, msg, edited, bumped, valueBefore, m.Value())
			}
		}
	}
}

// Repeated frames that change nothing (cursor blinks, highlight refreshes)
// must reuse the memoized rows for every line, offscreen ones included,
// without re-wrapping any of them.
func TestWrapCacheReuseAcrossFrames(t *testing.T) {
	m := areaWithLines(t, "alpha beta gamma\ndelta epsilon", 40, 4)
	_ = m.View() // first frame wraps and memoizes every line

	for i := range m.value {
		rows := m.memoizedWrap(i)
		_ = m.View() // frame with no buffer change
		if again := m.memoizedWrap(i); &rows[0] != &again[0] {
			t.Errorf("unchanged frame re-wrapped line %d", i)
		}
	}
}

// An edit bumps the revision, which must invalidate rows memoized before it,
// even for lines the edit never touched.
func TestWrapCacheInvalidatedByEdit(t *testing.T) {
	m := areaWithLines(t, "alpha beta gamma\ndelta epsilon", 40, 4)
	_ = m.View()
	stale := m.memoizedWrap(1)
	before := m.Revision()

	m = step(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})

	if _, ok := m.wraps.get(1, before, m.width); ok {
		t.Errorf("rows memoized before the edit are still served after it")
	}
	if fresh := m.memoizedWrap(1); &fresh[0] == &stale[0] {
		t.Errorf("edit must invalidate rows for untouched lines")
	}
	// Rows already handed out must not be rewritten in place.
	if got := string(stale[0]); strings.Contains(got, "z") {
		t.Errorf("memoized rows were mutated: %q", got)
	}
}

// A resize re-wraps every line, so the memoizer must drop the old rows.
func TestWrapCacheInvalidatedByResize(t *testing.T) {
	m := areaWithLines(t, "alpha beta gamma", 40, 4)
	_ = m.View()
	wide := m.memoizedWrap(0)

	m.SetWidth(12)
	_ = m.View()

	if narrow := m.memoizedWrap(0); &narrow[0] == &wide[0] || len(narrow) <= 1 {
		t.Errorf("resize must re-wrap at the new width, got %d rows", len(narrow))
	}
}

// Frames with no buffer change must render byte-identical views; the wrap
// memoizer may not perturb the output between frames.
func TestViewStableAcrossFrames(t *testing.T) {
	doc := "d1 $ sound \"bd sn\"\nd2 $ sound \"cp\"\n" + strings.Repeat("pat 1 pat 2\n", 30)
	m := areaWithLines(t, strings.TrimRight(doc, "\n"), 30, 4)

	first := m.View()
	_ = m.View()
	third := m.View()
	if third != first || m.View() != first {
		t.Fatalf("view changed across unchanged frames:\nfirst: %q\nthird:  %q", first, third)
	}
}

// Buffers can hold more lines than MaxHeight (loading a document does not
// shrink to the cap), so a 100-line buffer must render three-digit line
// numbers without pushing the text out of the frame.
func TestLineNumberGutterFitsThreeDigitLines(t *testing.T) {
	var document strings.Builder
	for line := 1; line < 100; line++ {
		fmt.Fprintf(&document, "line %d\n", line)
	}
	lastLine := strings.Repeat("x", 36)
	document.WriteString(lastLine)

	m := areaWithLines(t, document.String(), 40, 4)
	m.ShowLineNumbers = true
	m.SetWidth(40)
	m = step(m, keyMsg(tea.KeyCtrlEnd))
	m.SetCursor(0)

	view := stripANSI(m.View())
	if !strings.Contains(view, lastLine) {
		t.Fatalf("line 100 content was clipped:\n%s", view)
	}
	for _, row := range strings.Split(view, "\n") {
		if cols := uniseg.StringWidth(row); cols > 40 {
			t.Errorf("row is %d columns wide, wider than the 40-column frame: %q", cols, row)
		}
	}
}

// The gutter reservation and the rendered line-number width must agree, or
// the widest numbers eat into the text area one column at a time.
func TestLineNumberGutterMatchesRenderedNumbers(t *testing.T) {
	m := New()
	m.Prompt = ""
	m.ShowLineNumbers = true
	m.SetValue(strings.Repeat("line\n", 99) + "last")
	m.SetWidth(40)

	number := m.formatLineNumber(100)
	if got := uniseg.StringWidth(number); got != 4 {
		t.Errorf("line 100 number renders %d columns, want 4: %q", got, number)
	}
	if want := 40 - 4; m.width != want {
		t.Errorf("text width = %d, want %d (frame minus gutter)", m.width, want)
	}
}

func TestViewShowsOnlyVisibleRows(t *testing.T) {
	var b strings.Builder
	for i := 1; i <= 20; i++ {
		fmt.Fprintf(&b, "row %02d\n", i)
	}
	m := areaWithLines(t, strings.TrimRight(b.String(), "\n"), 40, 4)

	for i := 0; i < 19; i++ {
		m = step(m, keyMsg(tea.KeyDown))
	}
	view := stripANSI(m.View())

	if rows := len(strings.Split(view, "\n")); rows != m.Height() {
		t.Errorf("view height: got %d rows, want %d", rows, m.Height())
	}
	if !strings.Contains(view, "row 20") {
		t.Errorf("view should show the cursor row, got:\n%s", view)
	}
	if strings.Contains(view, "row 01") {
		t.Errorf("view should not render rows scrolled above the fold, got:\n%s", view)
	}
}

func TestViewCountsWrappedRows(t *testing.T) {
	long := strings.Repeat("word ", 40)
	m := areaWithLines(t, long+"\ntail", 20, 3)

	for i := 0; i < 10; i++ {
		m = step(m, keyMsg(tea.KeyEnd))
		m = step(m, keyMsg(tea.KeyDown))
	}
	view := stripANSI(m.View())

	if rows := len(strings.Split(view, "\n")); rows != m.Height() {
		t.Errorf("soft-wrapped buffer: got %d rows, want %d", rows, m.Height())
	}
	if m.Line() == len(m.value)-1 && !strings.Contains(view, "tail") {
		t.Errorf("cursor is on the last line but the view does not show it:\n%s", view)
	}
}
