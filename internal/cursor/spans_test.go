package cursor

import (
	"testing"
	"time"
)

func TestSpansAddDistinct(t *testing.T) {
	var s Spans
	now := time.Now()

	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 2, StartCol: 10, EndCol: 13}, now)
	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 2, StartCol: 20, EndCol: 24}, now)
	s.Add(Highlight{Stream: 2, Line: 2, EndLine: 2, StartCol: 10, EndCol: 13}, now)

	got := s.All()
	if len(got) != 3 {
		t.Fatalf("expected 3 spans, got %d", len(got))
	}
}

func TestSpansUnionOverlaps(t *testing.T) {
	var s Spans
	now := time.Now()

	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 2, StartCol: 10, EndCol: 13}, now)
	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 2, StartCol: 12, EndCol: 16}, now)

	got := s.All()
	if len(got) != 1 {
		t.Fatalf("overlapping spans should union into one, got %d", len(got))
	}
	if got[0].StartCol != 10 || got[0].EndCol != 16 {
		t.Errorf("union should cover both spans, got [%d,%d)", got[0].StartCol, got[0].EndCol)
	}
}

func TestSpansUnionMultiLine(t *testing.T) {
	var s Spans
	now := time.Now()

	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 2, StartCol: 10, EndCol: 14}, now)
	s.Add(Highlight{Stream: 1, Line: 2, EndLine: 3, StartCol: 8, EndCol: 5}, now)

	got := s.All()
	if len(got) != 1 {
		t.Fatalf("expected merged span, got %d", len(got))
	}
	if got[0].Line != 2 || got[0].StartCol != 8 || got[0].EndLine != 3 || got[0].EndCol != 5 {
		t.Errorf("unexpected merge result: %+v", got[0])
	}
}

func TestSpansRefreshExtendsLife(t *testing.T) {
	var s Spans
	t0 := time.Now()
	ttl := 80 * time.Millisecond

	s.Add(Highlight{Stream: 1, Line: 1, EndLine: 1, StartCol: 1, EndCol: 3}, t0)
	s.Add(Highlight{Stream: 1, Line: 1, EndLine: 1, StartCol: 1, EndCol: 3}, t0.Add(60*time.Millisecond))

	if s.Prune(t0.Add(100*time.Millisecond), ttl) {
		t.Fatal("span refreshed at 60ms should survive pruning at 100ms (age 40ms)")
	}
	if !s.Prune(t0.Add(160*time.Millisecond), ttl) {
		t.Fatal("span refreshed at 60ms should be pruned at 160ms (age 100ms)")
	}
	if !s.Empty() {
		t.Fatal("pruned spans should leave the set empty")
	}
}

func TestSpansClear(t *testing.T) {
	var s Spans
	s.Add(Highlight{Stream: 3, Line: 1, EndLine: 1, StartCol: 1, EndCol: 4}, time.Now())

	if !s.Clear() {
		t.Fatal("clear should report active spans")
	}
	if !s.Empty() {
		t.Fatal("clear should empty the set")
	}
	if s.Clear() {
		t.Fatal("clearing an empty set should report no activity")
	}
}
