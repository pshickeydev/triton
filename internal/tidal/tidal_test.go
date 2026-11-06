package tidal

import "testing"

func TestExtractBlock_SingleLine(t *testing.T) {
	lines := []string{"d1 $ sound \"bd sn\"", "", "d2 $ sound \"cp\""}
	code, start := ExtractBlock(lines, 0)
	if code != "d1 $ sound \"bd sn\"" {
		t.Errorf("expected single line, got %q", code)
	}
	if start != 0 {
		t.Errorf("expected start 0, got %d", start)
	}
}

func TestExtractBlock_MultiLine(t *testing.T) {
	lines := []string{
		"-- comment",
		"",
		"d1 $ sound \"bd sn\"",
		"  # gain \"0.8\"",
		"  # speed \"2\"",
		"",
		"-- next block",
	}
	code, start := ExtractBlock(lines, 2)
	expected := "d1 $ sound \"bd sn\"\n  # gain \"0.8\"\n  # speed \"2\""
	if code != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, code)
	}
	if start != 2 {
		t.Errorf("expected start 2, got %d", start)
	}
}

func TestExtractBlock_EmptyLine(t *testing.T) {
	lines := []string{"d1 $ sound \"bd\"", "", "d2 $ sound \"cp\""}
	code, start := ExtractBlock(lines, 1)
	if code != "" {
		t.Errorf("expected empty string for blank line, got %q", code)
	}
	if start != 1 {
		t.Errorf("expected start 1, got %d", start)
	}
}

func TestExtractBlock_EmptyBuffer(t *testing.T) {
	code, start := ExtractBlock([]string{}, 0)
	if code != "" {
		t.Errorf("expected empty string, got %q", code)
	}
	if start != 0 {
		t.Errorf("expected start 0, got %d", start)
	}
}

func TestExtractBlock_CursorOutOfBounds(t *testing.T) {
	lines := []string{"d1 $ sound \"bd\""}
	code, start := ExtractBlock(lines, 5)
	if code != "d1 $ sound \"bd\"" {
		t.Errorf("expected clamped line, got %q", code)
	}
	if start != 0 {
		t.Errorf("expected start 0, got %d", start)
	}
}

func TestExtractBlock_NegativeCursor(t *testing.T) {
	lines := []string{"d1 $ sound \"bd\""}
	code, start := ExtractBlock(lines, -1)
	if code != "d1 $ sound \"bd\"" {
		t.Errorf("expected clamped line, got %q", code)
	}
	if start != 0 {
		t.Errorf("expected start 0, got %d", start)
	}
}

func TestExtractBlock_LastBlock(t *testing.T) {
	lines := []string{
		"d1 $ sound \"bd\"",
		"",
		"d2 $ sound \"cp\"",
		"d3 $ sound \"hh\"",
	}
	code, start := ExtractBlock(lines, 3)
	expected := "d2 $ sound \"cp\"\nd3 $ sound \"hh\""
	if code != expected {
		t.Errorf("expected:\n%q\ngot:\n%q", expected, code)
	}
	if start != 2 {
		t.Errorf("expected start 2, got %d", start)
	}
}

func TestPatternStream(t *testing.T) {
	cases := []struct {
		line string
		want int
		ok   bool
	}{
		{`d1 $ sound "bd"`, 1, true},
		{`  d2  $ sound "bd"`, 2, true},
		{`d12$ s "cp"`, 12, true},
		{`-- d1 $ sound "bd"`, 0, false},
		{`sound "bd"`, 0, false},
		{`d1 = sound "bd"`, 0, false},
	}
	for _, c := range cases {
		n, ok := patternStream(c.line)
		if ok != c.ok || n != c.want {
			t.Errorf("patternStream(%q) = (%d, %v), want (%d, %v)", c.line, n, ok, c.want, c.ok)
		}
	}
}

func TestStreamSegments(t *testing.T) {
	code := "d1 $ sound \"bd\"\n  # speed \"2\"\n\nlet x = 2\nd2 $ sound \"cp\"\nd1 $ sound \"sn\""

	segs := Segments(code)
	if len(segs) != 2 {
		t.Fatalf("expected 2 streams, got %d", len(segs))
	}
	if len(segs[1]) != 2 {
		t.Fatalf("expected 2 segments for stream 1, got %d", len(segs[1]))
	}
	wantFirst := "d1 $ sound \"bd\"\n  # speed \"2\""
	if segs[1][0] != wantFirst {
		t.Errorf("first d1 segment = %q, want %q", segs[1][0], wantFirst)
	}
	if segs[1][1] != "d1 $ sound \"sn\"" {
		t.Errorf("second d1 segment = %q", segs[1][1])
	}
	if segs[2][0] != "d2 $ sound \"cp\"" {
		t.Errorf("d2 segment = %q, want %q", segs[2][0], "d2 $ sound \"cp\"")
	}
}

func TestRanged_NormalizesSegments(t *testing.T) {
	code := "d1  $   sound \"bd\"\n  #  speed \"2\"\n\nlet x = 2\nd2 $ sound \"cp\""

	segs := Ranged(code)
	wantFirst := Segment{Text: "d1 $ sound \"bd\"\n# speed \"2\"", Start: 0, End: 1}
	if segs[1][0] != wantFirst {
		t.Errorf("normalized d1 segment = %+v, want %+v", segs[1][0], wantFirst)
	}
	if segs[2][0] != (Segment{Text: `d2 $ sound "cp"`, Start: 4, End: 4}) {
		t.Errorf("normalized d2 segment = %+v", segs[2][0])
	}
}

func TestRanged_TracksLineRanges(t *testing.T) {
	code := "d1 $ sound \"bd\"\n  # speed \"2\"\n\nlet x = 2\nd2 $ sound \"cp\"\nd1 $ sound \"sn\""

	segs := Ranged(code)
	if segs[1][1] != (Segment{Text: `d1 $ sound "sn"`, Start: 5, End: 5}) {
		t.Errorf("second d1 segment = %+v, want lines 5-5", segs[1][1])
	}
	if segs[2][0] != (Segment{Text: `d2 $ sound "cp"`, Start: 4, End: 4}) {
		t.Errorf("d2 segment = %+v, want lines 4-4", segs[2][0])
	}
}

// GHCi groups unindented lines into independent statements, so a
// non-assignment line after a d-stream segment must not be absorbed
// into the segment above it.
func TestSegmentsIndependentStatement(t *testing.T) {
	segs := Segments("d1 $ sound \"bd\"\nsetcps 0.8")
	if got := segs[1][0]; got != `d1 $ sound "bd"` {
		t.Fatalf("d1 segment absorbed independent statement: %q", got)
	}
}

func TestSegmentsMixedStatements(t *testing.T) {
	code := "d1 $ sound \"bd\"\n  # speed \"2\"\nsetcps 0.8\n-- a note\nd2 $ sound \"cp\""
	segs := Segments(code)

	if got := segs[1][0]; got != "d1 $ sound \"bd\"\n  # speed \"2\"" {
		t.Errorf("d1 segment = %q, want only its two-line statement", got)
	}
	if _, ok := segs[2]; !ok {
		t.Fatal("d2 segment missing")
	}
	if got := segs[2][0]; got != `d2 $ sound "cp"` {
		t.Errorf("d2 segment = %q, want %q", got, `d2 $ sound "cp"`)
	}
	if len(segs) != 2 {
		t.Errorf("unassigned statements belong to no stream, got streams %v", segs)
	}
}

func TestSegmentsUnindentedAssignmentEndsSegment(t *testing.T) {
	code := "d1 $ sound \"bd\"\nd2 $ sound \"cp\""
	segs := Segments(code)
	if len(segs[1]) != 1 || segs[1][0] != `d1 $ sound "bd"` {
		t.Errorf("d1 segment = %v, want a single unabsorbed statement", segs[1])
	}
	if len(segs[2]) != 1 || segs[2][0] != `d2 $ sound "cp"` {
		t.Errorf("d2 segment = %v, want a single unabsorbed statement", segs[2])
	}
}
