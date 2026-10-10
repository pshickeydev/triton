package status

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pshickeydev/triton/internal/ui"
)

const (
	maxLogLines     = 500
	trimmedLogLines = 250
	minLogLines     = 1

	logTitle     = " console "
	noLogs       = " (no output)"
	errLogPrefix = "!! "
)

type Log struct {
	palette       ui.Palette
	viewport      viewport.Model
	width         int
	height        int
	lines         []string
	rendered      string
	renderedWidth int
}

func NewLog(palette ui.Palette) *Log {
	vp := viewport.New(0, 0)
	return &Log{
		palette:  palette,
		viewport: vp,
	}
}

func (l *Log) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.viewport.Width = w
	l.viewport.Height = max(h-1, minLogLines)
	l.refresh()
}

func (l *Log) AddOutput(text string) {
	var added []string
	for line := range strings.SplitSeq(text, "\n") {
		l.lines = append(l.lines, line)
		added = append(added, line)
	}
	if len(l.lines) > maxLogLines {
		l.lines = l.lines[len(l.lines)-trimmedLogLines:]
		l.refresh()
	} else {
		l.appendRendered(added)
	}
	l.viewport.GotoBottom()
}

func (l *Log) AddError(text string) {
	l.AddOutput(errLogPrefix + text)
}

// appendRendered renders only the new lines and appends them to the cached
// content, falling back to a full refresh when the cache does not apply
// (empty log placeholder or a width change not yet rendered).
func (l *Log) appendRendered(added []string) {
	if len(l.lines) == len(added) || l.renderedWidth != l.width {
		l.refresh()
		return
	}
	wrap := lipgloss.NewStyle().Width(max(l.width, minLogLines))
	for _, line := range added {
		l.rendered += "\n" + wrap.Render(line)
	}
	l.viewport.SetContent(l.rendered)
}

func (l *Log) Update(msg tea.Msg) (*Log, tea.Cmd) {
	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

func (l *Log) refresh() {
	contentWidth := max(l.width, minLogLines)
	var b strings.Builder

	if len(l.lines) == 0 {
		idleStyle := lipgloss.NewStyle().Foreground(l.palette.Idle)
		b.WriteString(idleStyle.Render(noLogs))
	} else {
		wrap := lipgloss.NewStyle().Width(contentWidth)
		for i, line := range l.lines {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(wrap.Render(line))
		}
	}

	l.rendered = b.String()
	l.renderedWidth = l.width
	l.viewport.SetContent(l.rendered)
}

func (l Log) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(l.palette.Label)

	var b strings.Builder
	b.WriteString(titleStyle.Render(logTitle))
	b.WriteString("\n")
	b.WriteString(l.viewport.View())

	return b.String()
}
