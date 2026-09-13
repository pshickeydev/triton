package textarea

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Highlight is a buffer range rendered with a custom style, used for the
// pattern tracing cursor. Positions are 1-based buffer coordinates: Line
// and EndLine name buffer lines (EndLine >= Line), Start is the first
// highlighted column on Line and End the column after the last highlighted
// column on EndLine, both inclusive/exclusive respectively.
type Highlight struct {
	Line    int
	EndLine int
	Start   int
	End     int
	Style   lipgloss.Style
}

// SetHighlights replaces the set of rendered highlight spans.
func (m *Model) SetHighlights(highlights []Highlight) {
	m.highlights = highlights
}

// YOffset returns the vertical scroll position of the viewport in rows.
func (m Model) YOffset() int {
	return m.viewport.YOffset
}

// colRange is a 0-based rune index range [start, end) within a logical
// (unwrapped) line, together with the style to render it in.
type colRange struct {
	start int
	end   int
	style lipgloss.Style
}

// highlightedRanges expands the highlights into rune index ranges per
// logical line (0-based), mapping 1-based buffer coordinates and clamping
// to the actual line contents. Later ranges render on top of earlier ones.
func (m Model) highlightedRanges() map[int][]colRange {
	if len(m.highlights) == 0 {
		return nil
	}
	ranges := make(map[int][]colRange)
	for _, hl := range m.highlights {
		if hl.Line < 1 || hl.Line > len(m.value) {
			continue
		}
		endLine := max(hl.EndLine, hl.Line)
		endLine = min(endLine, len(m.value))
		for l := hl.Line; l <= endLine; l++ {
			lineLen := len(m.value[l-1])
			start := 0
			if l == hl.Line {
				start = hl.Start - 1
			}
			end := lineLen
			if l == endLine {
				end = hl.End - 1
			}
			start = max(start, 0)
			end = min(end, lineLen)
			if start >= end {
				continue
			}
			ranges[l-1] = append(ranges[l-1], colRange{start: start, end: end, style: hl.Style})
		}
	}
	return ranges
}

// matchRange returns the range covering the given rune index, or nil.
func matchRange(idx int, ranges []colRange) *colRange {
	for i := range ranges {
		if idx >= ranges[i].start && idx < ranges[i].end {
			return &ranges[i]
		}
	}
	return nil
}

// renderSpans writes one wrapped line, splitting it into runs that share a
// range so each run can be styled. With no ranges it renders the whole
// line in the base style. offset is the rune index the wrapped line starts
// at within its logical line, so soft-wrapped spans style correctly.
func renderSpans(s *strings.Builder, base lipgloss.Style, wrapped []rune, offset int, ranges []colRange) {
	if len(ranges) == 0 {
		s.WriteString(base.Render(string(wrapped)))
		return
	}
	i := 0
	for i < len(wrapped) {
		r := matchRange(offset+i, ranges)
		j := i + 1
		for j < len(wrapped) && matchRange(offset+j, ranges) == r {
			j++
		}
		style := base
		if r != nil {
			style = r.style
		}
		s.WriteString(style.Render(string(wrapped[i:j])))
		i = j
	}
}

// renderCursorRow renders the wrapped line holding the cursor with
// highlight spans applied around the cursor cell, mirroring the unspanned
// cursor-line rendering.
func (m Model) renderCursorRow(s *strings.Builder, style lipgloss.Style, wrappedLine []rune, offset int, ranges []colRange, lineInfo LineInfo, line []rune) {
	co := lineInfo.ColumnOffset
	renderSpans(s, style, wrappedLine[:co], offset, ranges)
	if m.col >= len(line) && lineInfo.CharOffset >= m.width {
		m.Cursor.SetChar(" ")
		s.WriteString(m.Cursor.View())
		return
	}
	m.Cursor.SetChar(string(wrappedLine[co]))
	cellStyle := style
	if r := matchRange(offset+co, ranges); r != nil {
		cellStyle = r.style
	}
	s.WriteString(cellStyle.Render(m.Cursor.View()))
	renderSpans(s, style, wrappedLine[co+1:], offset+co+1, ranges)
}
