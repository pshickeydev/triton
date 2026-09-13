package cursor

import (
	"slices"
	"time"
)

// Span is a buffer range that is currently sounding on a stream. Spans are
// refresh-based: Tidal re-sends each fragment's position on every clock
// tick, so a span stays active while its messages keep arriving and is
// dropped after a short silence.
type Span struct {
	Stream   int
	Line     int // 1-based, inclusive
	EndLine  int // 1-based, inclusive
	StartCol int // 1-based column on Line, inclusive
	EndCol   int // 1-based column on EndLine, exclusive
	lastSeen time.Time
}

// Spans tracks the set of active highlight spans.
type Spans struct {
	spans []Span
}

// Add records a highlight, refreshing the last-seen time of an overlapping
// span (polymeter events can overlap, so spans are unioned) or appending a
// new one.
func (s *Spans) Add(hl Highlight, now time.Time) {
	span := Span{
		Stream:   hl.Stream,
		Line:     hl.Line,
		EndLine:  hl.EndLine,
		StartCol: hl.StartCol,
		EndCol:   hl.EndCol,
		lastSeen: now,
	}
	for i := range s.spans {
		if s.spans[i].Stream == span.Stream && overlaps(s.spans[i], span) {
			s.spans[i] = union(s.spans[i], span)
			return
		}
	}
	s.spans = append(s.spans, span)
}

// Prune drops spans whose last refresh is older than ttl and reports whether
// anything changed.
func (s *Spans) Prune(now time.Time, ttl time.Duration) bool {
	kept := s.spans[:0]
	changed := false
	for _, span := range s.spans {
		if now.Sub(span.lastSeen) <= ttl {
			kept = append(kept, span)
		} else {
			changed = true
		}
	}
	if changed {
		clear(s.spans[len(kept):])
		s.spans = kept
	}
	return changed
}

// DropIf removes the spans matching pred and reports whether anything was
// removed.
func (s *Spans) DropIf(pred func(Span) bool) bool {
	kept := s.spans[:0]
	changed := false
	for _, span := range s.spans {
		if pred(span) {
			changed = true
			continue
		}
		kept = append(kept, span)
	}
	if changed {
		clear(s.spans[len(kept):])
		s.spans = kept
	}
	return changed
}

// Clear drops all spans and reports whether anything was active.
func (s *Spans) Clear() bool {
	active := len(s.spans) > 0
	s.spans = nil
	return active
}

// Empty reports whether no spans are active.
func (s *Spans) Empty() bool {
	return len(s.spans) == 0
}

// All returns the active spans sorted for stable rendering.
func (s *Spans) All() []Span {
	all := slices.Clone(s.spans)
	slices.SortFunc(all, func(a, b Span) int {
		if a.Stream != b.Stream {
			return a.Stream - b.Stream
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return a.StartCol - b.StartCol
	})
	return all
}

// overlaps reports whether two spans cover at least one buffer cell in
// common, ignoring their last-seen times.
func overlaps(a, b Span) bool {
	if a.EndLine < b.Line || b.EndLine < a.Line {
		return false
	}
	singleA := a.Line == a.EndLine
	singleB := b.Line == b.EndLine
	if singleA && singleB {
		return a.StartCol < b.EndCol && b.StartCol < a.EndCol
	}
	return true
}

// union merges two overlapping spans. The result keeps the newer last-seen
// time so the merged span survives as long as either fragment is still
// being refreshed.
func union(a, b Span) Span {
	merged := Span{
		Stream:   a.Stream,
		Line:     a.Line,
		StartCol: a.StartCol,
		EndLine:  a.EndLine,
		EndCol:   a.EndCol,
	}
	if b.Line < merged.Line {
		merged.Line = b.Line
		merged.StartCol = b.StartCol
	} else if b.Line == merged.Line {
		merged.StartCol = min(merged.StartCol, b.StartCol)
	}
	if b.EndLine > merged.EndLine {
		merged.EndLine = b.EndLine
		merged.EndCol = b.EndCol
	} else if b.EndLine == merged.EndLine {
		merged.EndCol = max(merged.EndCol, b.EndCol)
	}
	merged.lastSeen = latest(a.lastSeen, b.lastSeen)
	return merged
}

// latest returns the newer of two timestamps.
func latest(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
