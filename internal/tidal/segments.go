package tidal

import (
	"regexp"
	"strconv"
	"strings"
)

const patternAssign = `^\s*d(\d+)\s*\$`

var patternRe = regexp.MustCompile(patternAssign)

// Segment is one d-stream statement block: its normalized text plus the
// 0-based range of source lines it occupies.
type Segment struct {
	Text  string
	Start int // first line, 0-based
	End   int // last line, 0-based, inclusive
}

// segmentSpan is one statement block collected by scan, before it is
// shaped into Segments or Ranged output.
type segmentSpan struct {
	stream int
	start  int
	lines  []string
}

// Segments splits code into per-stream pattern segments. A segment
// starts at a d-stream assignment line and runs until the next
// assignment, a blank line, an unindented independent statement, or the
// end of the code, so continuation lines belong to the pattern above
// them. Streams assigned more than once keep every segment, in order.
func Segments(code string) map[int][]string {
	segments := make(map[int][]string)
	for _, span := range scan(code) {
		segments[span.stream] = append(segments[span.stream], strings.Join(span.lines, "\n"))
	}
	return segments
}

// Ranged returns the per-stream segments with their source line ranges,
// normalized for comparison, ready for playing-vs-edited and
// highlight-coordinate checks.
func Ranged(code string) map[int][]Segment {
	segments := make(map[int][]Segment)
	for _, span := range scan(code) {
		segments[span.stream] = append(segments[span.stream], Segment{
			Text:  Normalize(strings.Join(span.lines, "\n")),
			Start: span.start,
			End:   span.start + len(span.lines) - 1,
		})
	}
	return segments
}

// scan splits code into d-stream statement blocks aligned with GHCi's
// statement grouping: only indented lines continue the statement above
// them, so an unindented line between assignments is an independent
// statement that belongs to no stream.
func scan(code string) []segmentSpan {
	lines := strings.Split(code, "\n")
	var spans []segmentSpan
	stream := 0
	start := 0
	var current []string
	flush := func() {
		if stream > 0 && len(current) > 0 {
			spans = append(spans, segmentSpan{stream: stream, start: start, lines: current})
		}
		stream = 0
		current = nil
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if n, ok := patternStream(line); ok {
			flush()
			stream, start = n, i
			current = []string{line}
			continue
		}
		if stream > 0 && !indented(line) {
			flush()
			continue
		}
		if stream > 0 {
			current = append(current, line)
		}
	}
	flush()
	return spans
}

// indented reports whether line continues the statement above it, the
// way GHCi groups statements: only whitespace-led lines continue.
func indented(line string) bool {
	return len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
}

// patternStream reports the d-stream a line assigns, e.g. d1 $ sound "bd".
func patternStream(line string) (int, bool) {
	match := patternRe.FindStringSubmatch(line)
	if match == nil {
		return 0, false
	}
	n, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// Normalize collapses whitespace so patterns that differ only in
// indentation and spacing compare as equal.
func Normalize(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}

// ExtractBlock returns the contiguous non-empty lines around the cursor
// line and the line the block starts on.
func ExtractBlock(lines []string, cursorLine int) (string, int) {
	if len(lines) == 0 {
		return "", 0
	}
	if cursorLine < 0 {
		cursorLine = 0
	}
	if cursorLine >= len(lines) {
		cursorLine = len(lines) - 1
	}

	if strings.TrimSpace(lines[cursorLine]) == "" {
		return "", cursorLine
	}

	start := cursorLine
	for start > 0 && strings.TrimSpace(lines[start-1]) != "" {
		start--
	}
	end := cursorLine
	for end < len(lines)-1 && strings.TrimSpace(lines[end+1]) != "" {
		end++
	}
	return strings.Join(lines[start:end+1], "\n"), start
}
