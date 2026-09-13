package ghci

import (
	"testing"
)

func TestGroupStatements_SingleLine(t *testing.T) {
	groups := groupStatements([]string{"d1 $ sound \"bd sn\""})
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Fatalf("expected group of 1 line, got %d", len(groups[0]))
	}
}

func TestGroupStatements_IndependentLines(t *testing.T) {
	lines := []string{
		`d1 $ sound "bd sn"`,
		`d2 $ sound "cp"`,
	}
	groups := groupStatements(lines)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestGroupStatements_MultiLineExpression(t *testing.T) {
	lines := []string{
		`d1 $ sound "bd sn"`,
		`  # gain "0.8"`,
		`  # speed "2"`,
	}
	groups := groupStatements(lines)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 3 {
		t.Fatalf("expected group of 3 lines, got %d", len(groups[0]))
	}
}

func TestGroupStatements_MixedStatements(t *testing.T) {
	lines := []string{
		`d1 $ sound "bd sn"`,
		`  # gain "0.8"`,
		`d2 $ sound "cp"`,
		`  # speed "1.5"`,
		`d3 $ sound "hh*4"`,
	}
	groups := groupStatements(lines)
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(groups))
	}
	if len(groups[0]) != 2 {
		t.Fatalf("group 0: expected 2 lines, got %d", len(groups[0]))
	}
	if len(groups[1]) != 2 {
		t.Fatalf("group 1: expected 2 lines, got %d", len(groups[1]))
	}
	if len(groups[2]) != 1 {
		t.Fatalf("group 2: expected 1 line, got %d", len(groups[2]))
	}
}

func TestGroupStatements_SkipsBlankLines(t *testing.T) {
	lines := []string{
		`d1 $ sound "bd"`,
		``,
		`d2 $ sound "cp"`,
	}
	groups := groupStatements(lines)
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (blank skipped), got %d", len(groups))
	}
}

func TestGroupStatements_TabIndented(t *testing.T) {
	lines := []string{
		`d1 $ sound "bd sn"`,
		"\t# gain \"0.8\"",
	}
	groups := groupStatements(lines)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group with tab continuation, got %d", len(groups))
	}
}
