package textarea

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestLineNumberGutterDoesNotClipLine100(t *testing.T) {
	var document strings.Builder
	for line := 1; line < 100; line++ {
		fmt.Fprintf(&document, "line %d\n", line)
	}
	lastLine := strings.Repeat("x", 36)
	document.WriteString(lastLine)
	m := areaWithLines(t, document.String(), 40, 4)
	m.ShowLineNumbers = true
	m.SetWidth(40)
	m = step(m, tea.KeyPressMsg{Code: tea.KeyEnd, Mod: tea.ModCtrl})
	m.SetCursor(0)
	view := stripANSI(m.View())
	if !strings.Contains(view, lastLine) {
		t.Fatalf("line 100 content was clipped:\n%s", view)
	}
}
