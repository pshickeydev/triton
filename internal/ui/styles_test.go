package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestPaneStyle_FocusedUsesFocusBorder(t *testing.T) {
	theme := DarkTheme()
	styles := NewStyles(theme)

	focused := styles.PaneStyle(PaneEditor, true)
	unfocused := styles.PaneStyle(PaneEditor, false)

	if focused.GetBorderTopForeground() != theme.FocusBorder {
		t.Errorf("focused pane border = %v, want focus border %v",
			focused.GetBorderTopForeground(), theme.FocusBorder)
	}
	if unfocused.GetBorderTopForeground() != theme.Border {
		t.Errorf("unfocused pane border = %v, want border %v",
			unfocused.GetBorderTopForeground(), theme.Border)
	}
}

func TestPaneStyle_FocusDoesNotMutateBase(t *testing.T) {
	styles := NewStyles(DarkTheme())

	_ = styles.PaneStyle(PaneStatus, true)

	again := styles.PaneStyle(PaneStatus, false)
	if again.GetBorderTopForeground() != DarkTheme().Border {
		t.Errorf("base style mutated after focused render, border = %v",
			again.GetBorderTopForeground())
	}
}

func TestLayout_RenderHighlightsFocusedPane(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	theme := DarkTheme()
	styles := NewStyles(theme)

	layout := NewLayout()
	layout.SetSize(100, 30)

	marker := lipgloss.NewStyle().Foreground(theme.FocusBorder).Render("x")
	if marker == "x" {
		t.Skip("color profile not producing ANSI sequences")
	}
	colorSeq := marker[:strings.Index(marker, "x")]

	out := layout.Render("code", "status", "log", styles, PaneStatus, 0)
	if !strings.Contains(out, colorSeq) {
		t.Fatalf("focused render missing focus border color %v", theme.FocusBorder)
	}

	out = layout.Render("code", "status", "log", styles, PaneEditor, 0)
	if !strings.Contains(out, colorSeq) {
		t.Fatalf("focused render missing focus border color %v", theme.FocusBorder)
	}
}

func TestModeForSize(t *testing.T) {
	tests := []struct {
		name string
		w, h int
		want Mode
	}{
		{"too small below 60x18", 59, 17, ModeTooSmall},
		{"too small width only", 59, 40, ModeTooSmall},
		{"too small height only", 100, 17, ModeTooSmall},
		{"small at minimum", 60, 18, ModeSmall},
		{"small below large threshold", 109, 33, ModeSmall},
		{"small wide but short", 120, 30, ModeSmall},
		{"large at minimum", 110, 34, ModeLarge},
		{"large above minimum", 140, 40, ModeLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := modeForSize(tt.w, tt.h); got != tt.want {
				t.Errorf("modeForSize(%d, %d) = %v, want %v", tt.w, tt.h, got, tt.want)
			}
		})
	}
}

func TestRenderSmall_EditorViewOmitsStatus(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	styles := NewStyles(DarkTheme())
	layout := NewLayout()
	layout.SetSize(90, 26)

	out := layout.RenderSmall("code", "status-marker", "log-marker", styles, PaneEditor, PaneEditor)
	if !strings.Contains(out, "code") {
		t.Errorf("editor view missing editor content")
	}
	if strings.Contains(out, "status-marker") || strings.Contains(out, "log-marker") {
		t.Errorf("editor view should not contain status or console content")
	}
}

func TestRenderSmall_StatusViewOmitsEditor(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	styles := NewStyles(DarkTheme())
	layout := NewLayout()
	layout.SetSize(90, 26)

	out := layout.RenderSmall("code-marker", "status", "log", styles, PaneConsole, PaneStatus)
	if strings.Contains(out, "code-marker") {
		t.Errorf("status view should not contain editor content")
	}
	if !strings.Contains(out, "status") || !strings.Contains(out, "log") {
		t.Errorf("status view should contain status and console content")
	}
}

func TestRenderTooSmall_ShowsNotice(t *testing.T) {
	styles := NewStyles(DarkTheme())
	layout := NewLayout()
	layout.SetSize(50, 15)

	out := layout.RenderTooSmall(styles)
	if !strings.Contains(out, tooSmallText) {
		t.Errorf("too-small render should contain %q", tooSmallText)
	}
}

func TestOverlay_PreservesDimensions(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	styles := NewStyles(DarkTheme())
	layout := NewLayout()
	layout.SetSize(120, 28)
	base := layout.Render("code", "status", "log", styles, PaneEditor, 0)

	out := layout.Overlay(base, styles.Warning.Render("Press Ctrl-C again to quit, Esc to cancel"))

	baseLines := strings.Split(base, "\n")
	outLines := strings.Split(out, "\n")
	if len(outLines) != len(baseLines) {
		t.Fatalf("line count changed: base %d, overlay %d", len(baseLines), len(outLines))
	}
	for i := range baseLines {
		if lipgloss.Width(outLines[i]) != lipgloss.Width(baseLines[i]) {
			t.Errorf("line %d width changed: base %d, overlay %d",
				i, lipgloss.Width(baseLines[i]), lipgloss.Width(outLines[i]))
		}
	}
	if !strings.Contains(out, "Press Ctrl-C again to quit, Esc to cancel") {
		t.Errorf("overlay missing warning text")
	}
}

func TestOverlay_PreservesUnderlyingColors(t *testing.T) {
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI256)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	theme := DarkTheme()
	styles := NewStyles(theme)
	layout := NewLayout()
	layout.SetSize(120, 28)

	consoleText := lipgloss.NewStyle().Foreground(theme.Foreground).Render("ghci> some output")
	base := layout.Render("code", "status", consoleText, styles, PaneEditor, 0)

	out := layout.Overlay(base, styles.Warning.Render("Press Ctrl-C again to quit, Esc to cancel"))

	fgSeq := lipgloss.NewStyle().Foreground(theme.Foreground).Render("x")
	fgSeq = fgSeq[:strings.Index(fgSeq, "x")]
	borderSeq := lipgloss.NewStyle().Foreground(theme.Border).Render("x")
	borderSeq = borderSeq[:strings.Index(borderSeq, "x")]

	if !strings.Contains(out, fgSeq) {
		t.Errorf("overlay dropped underlying foreground color %v", theme.Foreground)
	}
	if !strings.Contains(out, borderSeq) {
		t.Errorf("overlay dropped underlying border color %v", theme.Border)
	}
}
