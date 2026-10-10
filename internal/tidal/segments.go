package tidal

import (
	"regexp"
	"strconv"
	"strings"
)

const patternAssign = `^\s*d(\d+)\s*\$`

var patternRe = regexp.MustCompile(patternAssign)

// Segments splits code into per-stream pattern segments. A segment
// starts at a d-stream assignment line and runs until the next
// assignment, a blank line, or the end of the code, so continuation
// lines belong to the pattern above them. Streams assigned more than
// once keep every segment, in order.
func Segments(code string) map[int][]string {
	segments := make(map[int][]string)
	stream := 0
	var current []string
	flush := func() {
		if stream > 0 && len(current) > 0 {
			segments[stream] = append(segments[stream], strings.Join(current, "\n"))
		}
		current = nil
	}
	for line := range strings.SplitSeq(code, "\n") {
		if strings.TrimSpace(line) == "" {
			flush()
			stream = 0
			continue
		}
		if n, ok := patternStream(line); ok {
			flush()
			stream = n
		}
		if stream > 0 {
			current = append(current, line)
		}
	}
	flush()
	return segments
}

// Segmented returns Segments with every segment normalized, ready for
// playing-vs-edited comparison.
func Segmented(code string) map[int][]string {
	segments := Segments(code)
	for _, segs := range segments {
		for i, seg := range segs {
			segs[i] = Normalize(seg)
		}
	}
	return segments
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
