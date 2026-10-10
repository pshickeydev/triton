package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/muesli/reflow/ansi"
)

type Mode int

const (
	ModeTooSmall Mode = iota
	ModeSmall
	ModeLarge
)

const (
	// Pane-area minimum for the small layout. The help bar below the panes
	// takes 2 more rows, so the user-facing minimum terminal size is 60x20.
	smallMinWidth  = 60
	smallMinHeight = 18

	largeMinWidth  = 110
	largeMinHeight = 34

	editorRatioNum = 2
	editorRatioDen = 3
	minEditorWidth = 30
	minStatusWidth = 20
	paneGap        = 1
	frameSize      = 2 // border (1) + padding (1) on each axis
	minLogHeight   = 3

	tooSmallText = "Terminal too small (minimum 60x20)"
)

type Layout struct {
	width  int
	height int
	mode   Mode
}

func NewLayout() *Layout {
	return &Layout{mode: ModeTooSmall}
}

func (l *Layout) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.mode = modeForSize(w, h)
}

func modeForSize(w, h int) Mode {
	if w >= largeMinWidth && h >= largeMinHeight {
		return ModeLarge
	}
	if w >= smallMinWidth && h >= smallMinHeight {
		return ModeSmall
	}
	return ModeTooSmall
}

func (l *Layout) Mode() Mode {
	return l.mode
}

func (l *Layout) EditorWidth() int {
	if l.mode == ModeSmall {
		w := l.width - frameSize
		return max(w, minEditorWidth)
	}
	w := l.width*editorRatioNum/editorRatioDen - paneGap - frameSize
	return max(w, minEditorWidth)
}

func (l *Layout) StatusWidth() int {
	if l.mode == ModeSmall {
		w := l.width - frameSize
		return max(w, minStatusWidth)
	}
	w := l.width - l.EditorWidth() - paneGap - 2*frameSize
	return max(w, minStatusWidth)
}

// NeedsWidthAdjust reports whether the rendered output overflows the
// terminal width (a trailing line wider than the editor pane), which
// happens when terminal cells are wider than tall.
func (l *Layout) NeedsWidthAdjust(rendered string) bool {
	return lipgloss.Width(rendered) > l.width
}

func (l *Layout) StatusHeight() int {
	h := (l.TopHeight()-paneGap)/2 - frameSize
	return max(h, 1)
}

func (l *Layout) LogHeight() int {
	h := l.TopHeight() - paneGap - l.StatusHeight() - 2*frameSize
	return max(h, minLogHeight)
}

func (l *Layout) TopHeight() int {
	h := l.height - frameSize
	return max(h, 1)
}

// Render joins the panes side by side: editor on the left, status and
// console stacked on the right. widthAdjust widens the editor pane to
// absorb an overflow reported by NeedsWidthAdjust.
func (l *Layout) Render(editorView, statusView, logView string, styles Styles, focused Pane, widthAdjust int) string {
	editorPane := styles.PaneStyle(PaneEditor, focused == PaneEditor).
		Width(l.EditorWidth() + widthAdjust).
		Height(l.TopHeight()).
		Render(editorView)

	statusPane := styles.PaneStyle(PaneStatus, focused == PaneStatus).
		Width(l.StatusWidth()).
		Height(l.StatusHeight()).
		Render(statusView)

	logPane := styles.PaneStyle(PaneConsole, focused == PaneConsole).
		Width(l.StatusWidth()).
		Height(l.LogHeight()).
		Render(logView)

	right := lipgloss.JoinVertical(lipgloss.Left, statusPane, logPane)

	return lipgloss.JoinHorizontal(lipgloss.Top, editorPane, right)
}

// RenderSmall renders a single active pane filling the available space for
// the small layout mode. The status and console views are joined into one
// combined pane.
func (l *Layout) RenderSmall(editorView, statusView, logView string, styles Styles, focused Pane, active Pane) string {
	if active == PaneEditor {
		return styles.PaneStyle(PaneEditor, true).
			Width(l.EditorWidth()).
			Height(l.TopHeight()).
			Render(editorView)
	}

	statusPane := styles.PaneStyle(PaneStatus, focused == PaneStatus).
		Width(l.StatusWidth()).
		Height(l.StatusHeight()).
		Render(statusView)

	logPane := styles.PaneStyle(PaneConsole, focused == PaneConsole).
		Width(l.StatusWidth()).
		Height(l.LogHeight()).
		Render(logView)

	return lipgloss.JoinVertical(lipgloss.Left, statusPane, logPane)
}

// RenderTooSmall renders a minimal notice when the terminal is below the
// minimum required size for the small layout.
func (l *Layout) RenderTooSmall(styles Styles) string {
	w := max(l.width-frameSize, 1)
	h := max(l.height-frameSize, 1)
	return styles.Warning.
		Width(w).
		Height(h).
		Render(tooSmallText)
}

func (l *Layout) HelpBar(styles Styles, items string) string {
	return styles.Help.Render(items)
}

// Overlay centers content on top of the rendered base view, replacing the
// base lines it covers so the result keeps the same dimensions. The content
// is snapped to whole base cells so it never splits a base glyph.
func (l *Layout) Overlay(base, content string) string {
	baseLines := strings.Split(base, "\n")
	contentLines := strings.Split(content, "\n")
	contentWidth := lipgloss.Width(content)

	baseWidth := 0
	for _, line := range baseLines {
		baseWidth = max(baseWidth, lipgloss.Width(line))
	}

	top := max((len(baseLines)-len(contentLines))/2, 0)
	left := max((baseWidth-contentWidth)/2, 0)

	for i, line := range contentLines {
		row := top + i
		if row >= len(baseLines) {
			break
		}
		baseLines[row] = overlayLine(baseLines[row], line, left)
	}
	return strings.Join(baseLines, "\n")
}

func overlayLine(base, content string, left int) string {
	baseWidth := lipgloss.Width(base)
	contentWidth := lipgloss.Width(content)

	start := cellOffset(base, left)
	end := cellOffset(base, left+contentWidth)

	var b strings.Builder
	b.WriteString(base[:start])
	b.WriteString("\x1b[0m")
	b.WriteString(content)
	b.WriteString("\x1b[0m")
	if left+contentWidth < baseWidth {
		b.WriteString(lastColorSGR(base[:end]))
		b.WriteString(base[end:])
	}
	return b.String()
}

// cellOffset returns the byte offset in s where the printable cell at or
// after target begins, so cutting there never splits a glyph.
func cellOffset(s string, target int) int {
	inSeq := false
	cell := 0
	for i, r := range s {
		switch {
		case r == ansi.Marker:
			inSeq = true
		case inSeq:
			if ansi.IsTerminator(r) {
				inSeq = false
			}
		default:
			if cell >= target {
				return i
			}
			cell += runewidth.RuneWidth(r)
		}
	}
	return len(s)
}

// lastColorSGR returns the last color-setting SGR sequence in s, ignoring
// resets. This approximates the style of the region at the end of s, which
// lipgloss terminates with a reset.
func lastColorSGR(s string) string {
	last := ""
	for i := 0; i+2 < len(s); i++ {
		if s[i] != '\x1b' || s[i+1] != '[' {
			continue
		}
		j := i + 2
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == ';') {
			j++
		}
		if j < len(s) && s[j] == 'm' {
			seq := s[i : j+1]
			if seq != "\x1b[0m" && seq != "\x1b[m" {
				last = seq
			}
			i = j
		}
	}
	return last
}
