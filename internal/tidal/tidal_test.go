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

func TestSegmented_NormalizesSegments(t *testing.T) {
	code := "d1  $   sound \"bd\"\n  #  speed \"2\"\n\nlet x = 2\nd2 $ sound \"cp\""

	segs := Segmented(code)
	wantFirst := "d1 $ sound \"bd\"\n# speed \"2\""
	if segs[1][0] != wantFirst {
		t.Errorf("normalized d1 segment = %q, want %q", segs[1][0], wantFirst)
	}
	if segs[2][0] != "d2 $ sound \"cp\"" {
		t.Errorf("normalized d2 segment = %q", segs[2][0])
	}
}
