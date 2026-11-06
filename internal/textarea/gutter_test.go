package textarea

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
	m = step(m, keyMsg(tea.KeyCtrlEnd))
	m.SetCursor(0)
	view := stripANSI(m.View())
	if !strings.Contains(view, lastLine) {
		t.Fatalf("line 100 content was clipped:\n%s", view)
	}
}
