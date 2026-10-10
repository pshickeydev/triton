package status

import (
	"fmt"
	"strings"
	"testing"

	"github.com/pshickeydev/triton/internal/ui"
)

func newTestLog() *Log {
	l := NewLog(ui.DarkTheme().Palette)
	l.SetSize(40, 10)
	return l
}

func TestLogAddOutputShowsLines(t *testing.T) {
	l := newTestLog()
	l.AddOutput("one")
	l.AddOutput("two\nthree")

	view := l.View()
	for _, want := range []string{"one", "two", "three"} {
		if !strings.Contains(view, want) {
			t.Errorf("log view missing %q:\n%s", want, view)
		}
	}
}

func TestLogIncrementalMatchesFullRefresh(t *testing.T) {
	l := newTestLog()
	for i := range 10 {
		l.AddOutput(fmt.Sprintf("line %d", i))
	}
	incremental := l.rendered

	l.refresh()
	if incremental != l.rendered {
		t.Errorf("incremental rendering diverged from full refresh:\n%q\nwant:\n%q", incremental, l.rendered)
	}
}

func TestLogTrimsAndReRenders(t *testing.T) {
	l := newTestLog()
	for i := range maxLogLines {
		l.AddOutput(fmt.Sprintf("line %d", i))
	}
	if len(l.lines) != maxLogLines {
		t.Fatalf("expected %d lines before trim, got %d", maxLogLines, len(l.lines))
	}
	l.AddOutput("overflow")
	if len(l.lines) != trimmedLogLines {
		t.Fatalf("expected trim to %d lines, got %d", trimmedLogLines, len(l.lines))
	}
	view := l.View()
	if strings.Contains(view, "line 0") {
		t.Error("oldest lines should be trimmed from the rendered log")
	}
	if !strings.Contains(view, "overflow") {
		t.Error("newest line should be rendered")
	}
}

func TestLogWidthChangeReRenders(t *testing.T) {
	l := newTestLog()
	l.AddOutput("hello")
	l.SetSize(20, 10)

	if l.renderedWidth != 20 {
		t.Errorf("expected re-render at width 20, cached width %d", l.renderedWidth)
	}
	if !strings.Contains(l.View(), "hello") {
		t.Error("content lost after resize")
	}
}
