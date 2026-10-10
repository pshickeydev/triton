package model

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/pshickeydev/triton/internal/cursor"
	"github.com/pshickeydev/triton/internal/doc"
	"github.com/pshickeydev/triton/internal/status"
	"github.com/pshickeydev/triton/internal/ui"
)

func newTestModel() Model {
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	return NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{})
}

func TestCtrlC_ShowsQuitWarning(t *testing.T) {
	m := newTestModel()

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Errorf("first ctrl+c should not produce a command (no quit)")
	}
	um := updated.(Model)
	if !um.confirmQ {
		t.Errorf("first ctrl+c should set confirmQ")
	}
	if um.quitting {
		t.Errorf("first ctrl+c should not quit")
	}
}

func TestCtrlC_SecondPressQuits(t *testing.T) {
	m := newTestModel()
	m.confirmQ = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(Model)
	if !um.quitting {
		t.Errorf("second ctrl+c should set quitting")
	}
	if cmd == nil {
		t.Fatalf("second ctrl+c should return tea.Quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("expected tea.QuitMsg, got %v", msg)
	}
}

func TestInterrupt_MirrorsCtrlC(t *testing.T) {
	m := newTestModel()

	updated, cmd := m.Update(tea.InterruptMsg{})
	um := updated.(Model)
	if !um.confirmQ || um.quitting {
		t.Errorf("first interrupt should show warning, got confirmQ=%v quitting=%v", um.confirmQ, um.quitting)
	}
	if cmd != nil {
		t.Errorf("first interrupt should not quit")
	}

	updated, cmd = um.Update(tea.InterruptMsg{})
	um = updated.(Model)
	if !um.quitting {
		t.Errorf("second interrupt should quit")
	}
	if cmd == nil {
		t.Fatalf("second interrupt should return tea.Quit command")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("expected tea.QuitMsg, got %v", msg)
	}
}

func TestEsc_DismissesQuitWarning(t *testing.T) {
	m := newTestModel()
	m.confirmQ = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := updated.(Model)
	if um.confirmQ {
		t.Errorf("esc should clear confirmQ")
	}
	if um.quitting {
		t.Errorf("esc should not quit")
	}
	if cmd != nil {
		t.Errorf("esc should not produce a command")
	}
}

func TestToggleMute_UnknownStream(t *testing.T) {
	m := newTestModel()
	m.tracker = status.NewTracker(4)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF5})
	um := updated.(Model)
	if _, ok := um.tracker.Streams()[5]; ok {
		t.Errorf("stream 5 should not appear in the tracker")
	}
}

func TestQuitWarning_BlocksOtherKeys(t *testing.T) {
	m := newTestModel()
	m.confirmQ = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	um := updated.(Model)
	if !um.confirmQ {
		t.Errorf("unrelated key should not dismiss the warning")
	}
	if um.quitting {
		t.Errorf("unrelated key should not quit")
	}
	if cmd != nil {
		t.Errorf("unrelated key should not produce a command")
	}
}

func TestQuitWarning_ViewContainsWarning(t *testing.T) {
	m := newTestModel()
	m.confirmQ = true
	m.width = 100
	m.height = 30
	m.layout.SetSize(100, 30-2)

	view := m.View()
	if !strings.Contains(view, quitWarningText) {
		t.Errorf("view should contain quit warning text %q", quitWarningText)
	}
}

func TestQuitWarning_HelpTextMentionsCtrlC(t *testing.T) {
	m := newTestModel()
	if !strings.Contains(m.helpText(), "Ctrl-C: quit") {
		t.Errorf("help text should mention Ctrl-C: quit, got %q", m.helpText())
	}
}

func TestStartup_EditorAcceptsTypingWithoutTab(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if !m.textarea.Focused() {
		t.Fatalf("textarea should start focused so typing works immediately")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	um := updated.(Model)
	if um.textarea.Value() != "d" {
		t.Errorf("typing at startup should reach the editor, got %q", um.textarea.Value())
	}
}

func TestTab_SmallLayoutTogglesView(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = updated.(Model)

	if m.layout.Mode() != ui.ModeSmall {
		t.Fatalf("expected ModeSmall at 90x26, got %v", m.layout.Mode())
	}
	if m.view != viewEditor || m.focus != focusEditor {
		t.Fatalf("initial view/focus = %v/%v, want editor", m.view, m.focus)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.view != viewStatusConsole {
		t.Errorf("tab in small mode should switch to status+console view, got %v", m.view)
	}
	if m.focus != focusConsole {
		t.Errorf("tab in small mode should focus console, got %v", m.focus)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.view != viewEditor || m.focus != focusEditor {
		t.Errorf("second tab should return to editor, got view=%v focus=%v", m.view, m.focus)
	}
}

func TestTab_LargeLayoutCyclesPanes(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = updated.(Model)

	if m.layout.Mode() != ui.ModeLarge {
		t.Fatalf("expected ModeLarge at 120x40, got %v", m.layout.Mode())
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.focus != focusStatus {
		t.Errorf("tab in large mode should move focus to status, got %v", m.focus)
	}
	if m.view != viewEditor {
		t.Errorf("large mode should not change view, got %v", m.view)
	}
}

func TestView_SmallLayoutShowsOneView(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = updated.(Model)
	m.textarea.SetValue("d1 $ sound \"bd\"")

	view := m.View()
	if !strings.Contains(view, "d1 $ sound") {
		t.Errorf("editor view should contain editor content")
	}
	if strings.Contains(view, "streams:") {
		t.Errorf("editor view should not contain status panel")
	}
}

func TestView_TooSmallShowsNotice(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Errorf("too-small terminal should show notice, got %q", view)
	}
	if !strings.Contains(view, "60x20") {
		t.Errorf("notice should state the user-facing minimum size, got %q", view)
	}
	if !strings.Contains(view, "Ctrl-C: quit") {
		t.Errorf("too-small terminal should still show the help bar")
	}
}

func TestQuitting_ViewShowsShutdownMessage(t *testing.T) {
	m := newTestModel()
	m.quitting = true

	view := m.View()
	if !strings.Contains(view, shutdownText) {
		t.Errorf("quitting view should show shutdown message %q, got %q", shutdownText, view)
	}
	if strings.Contains(view, "sclang") {
		t.Errorf("without sclang managed, message should not mention sclang, got %q", view)
	}

	m2 := newTestModel()
	m2.sclang = true
	m2.quitting = true
	view = m2.View()
	if !strings.Contains(view, shutdownTextSC) {
		t.Errorf("with sclang managed, quitting view should show %q, got %q", shutdownTextSC, view)
	}
}

func TestQuitWarning_OverlaysTooSmallNotice(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
	m = updated.(Model)
	m.confirmQ = true

	view := m.View()
	if !strings.Contains(view, quitWarningText) {
		t.Errorf("quit warning should display over the too-small notice, got %q", view)
	}
}

func TestView_SmallLayoutHelpMentionsSwitchView(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = updated.(Model)

	view := m.View()
	if !strings.Contains(view, "Tab: switch view") {
		t.Errorf("small layout help should say 'Tab: switch view'")
	}
}

func TestSmallLayout_HiddenEditorDoesNotConsumeKeys(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = updated.(Model)
	m.textarea.SetValue("d1")

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)
	if m.view != viewStatusConsole {
		t.Fatalf("expected status view after tab")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	m = updated.(Model)
	if m.textarea.Value() != "d1" {
		t.Errorf("hidden editor should not receive typed input, got %q", m.textarea.Value())
	}
}

func TestResize_SmallToLargeKeepsState(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 26})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(Model)

	updated, _ = m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)
	if m.layout.Mode() != ui.ModeLarge {
		t.Fatalf("expected ModeLarge after resize, got %v", m.layout.Mode())
	}
	if m.focus != focusConsole {
		t.Errorf("focus should stay on console after resize, got %v", m.focus)
	}
	view := m.View()
	if !strings.Contains(view, "streams:") {
		t.Errorf("large layout should render status panel, got missing")
	}
}

func TestRefreshEdited(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd sn\"\n\nd2 $ sound \"cp\"")

	m.refreshEdited()
	edited := m.tracker.Edited()
	if !edited[1] || !edited[2] {
		t.Fatalf("unevaluated editor patterns should be marked edited, got %v", edited)
	}

	m.tracker.SetPlaying(1, "d1 $ sound \"bd sn\"")
	m.refreshEdited()
	edited = m.tracker.Edited()
	if edited[1] {
		t.Error("stream matching its evaluated code should not be marked edited")
	}
	if !edited[2] {
		t.Error("never-evaluated stream should stay edited")
	}

	m.textarea.SetValue("d1 $ sound \"bd cp\"\n\nd2 $ sound \"cp\"")
	m.refreshEdited()
	edited = m.tracker.Edited()
	if !edited[1] {
		t.Error("stream edited after evaluation should be marked edited")
	}
}

func TestRefreshEdited_IgnoresWhitespace(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"")
	m.tracker.SetPlaying(1, "  d1  $ sound \"bd\"  ")

	m.refreshEdited()
	if m.tracker.Edited()[1] {
		t.Error("whitespace-only differences should not mark a stream edited")
	}
}

func TestRefreshEdited_ClearsRemovedPattern(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"")
	m.tracker.SetPlaying(1, "d1 $ sound \"bd\"")
	m.tracker.SetEdited(1)

	m.textarea.SetValue("-- nothing here")
	m.refreshEdited()
	if m.tracker.Edited()[1] {
		t.Error("stream removed from the editor should not stay marked edited")
	}
}

func TestMarkEvaluated(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd sn\"")
	m.markEvaluated("d1 $ sound \"bd sn\"")

	if code := m.tracker.Playing()[1]; code != "d1 $ sound \"bd sn\"" {
		t.Errorf("evaluated code should be recorded as playing, got %q", code)
	}
	if m.tracker.Streams()[1] != status.StreamActive {
		t.Errorf("evaluated stream should be active, got %d", m.tracker.Streams()[1])
	}
	if m.tracker.Edited()[1] {
		t.Error("freshly evaluated stream should not be marked edited")
	}
}

func TestMarkEvaluated_MultiStreamBlock(t *testing.T) {
	m := newTestModel()
	code := "d1 $ sound \"bd\"\nd2 $ sound \"cp\""
	m.textarea.SetValue(code)
	m.markEvaluated(code)

	if _, ok := m.tracker.Playing()[2]; !ok {
		t.Error("multi-stream block should record playing code for every stream")
	}
	if m.tracker.Edited()[1] || m.tracker.Edited()[2] {
		t.Errorf("fully evaluated block should leave no edited streams, got %v", m.tracker.Edited())
	}
}

func TestMarkEvaluated_EvalLineWithinBlock(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\nd2 $ sound \"cp\"")
	m.refreshEdited()
	if !m.tracker.Edited()[1] || !m.tracker.Edited()[2] {
		t.Fatalf("unevaluated editor patterns should be marked edited, got %v", m.tracker.Edited())
	}

	m.markEvaluated("d1 $ sound \"bd\"")
	if m.tracker.Edited()[1] {
		t.Error("stream whose line was evaluated should not be marked edited")
	}
	if !m.tracker.Edited()[2] {
		t.Error("unevaluated stream should stay edited")
	}
}

func TestMarkEvaluated_PartialPatternStaysEdited(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n  # speed \"2\"")

	m.markEvaluated("d1 $ sound \"bd\"")
	if !m.tracker.Edited()[1] {
		t.Error("evaluating part of a multi-line pattern should leave the stream edited")
	}

	m.markEvaluated("d1 $ sound \"bd\"\n  # speed \"2\"")
	if m.tracker.Edited()[1] {
		t.Error("evaluating the full pattern should clear the marker")
	}
}

func TestMarkEvaluated_RecordsPerStreamSegments(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n  # speed \"2\"\nd2 $ sound \"cp\"")

	m.markEvaluated("d1 $ sound \"bd\"\n  # speed \"2\"\nd2 $ sound \"cp\"")
	if code := m.tracker.Playing()[1]; code != "d1 $ sound \"bd\"\n  # speed \"2\"" {
		t.Errorf("d1 playing code should be its own segment, got %q", code)
	}
	if code := m.tracker.Playing()[2]; code != "d2 $ sound \"cp\"" {
		t.Errorf("d2 playing code should be its own segment, got %q", code)
	}
	if m.tracker.Edited()[1] || m.tracker.Edited()[2] {
		t.Errorf("fully evaluated block should leave no edited streams, got %v", m.tracker.Edited())
	}
}

func TestRefreshEdited_MatchesAnySegment(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n\nd1 $ sound \"sn\"")

	m.markEvaluated("d1 $ sound \"sn\"")
	if m.tracker.Edited()[1] {
		t.Error("stream whose evaluated code still appears in the editor should not be marked edited")
	}
}

func TestRefreshEdited_BlankLineEndsSegment(t *testing.T) {
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n\n")
	m.tracker.SetPlaying(1, "d1 $ sound \"bd\"")

	m.refreshEdited()
	if m.tracker.Edited()[1] {
		t.Error("a trailing blank line should not break the match with the playing code")
	}
}

func TestNewModel_LogsMarkerHint(t *testing.T) {
	m := newTestModel()
	m.console.SetSize(80, 10)
	if !strings.Contains(m.console.View(), markerHint) {
		t.Errorf("console should explain the * marker on startup, missing %q", markerHint)
	}
}

func TestHighlightMsgSuppressesEditedStream(t *testing.T) {
	tracker := status.NewTracker(8)
	tracker.SetEdited(1)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{}, WithCursor(make(chan cursor.Highlight, 4), nil))

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(highlightMsg{span: cursor.Highlight{
		Stream: 1, Line: 1, EndLine: 1, StartCol: 13, EndCol: 15,
	}})
	um := updated.(Model)

	if !um.spans.Empty() {
		t.Errorf("spans on an edited stream should be suppressed, got %+v", um.spans.All())
	}
}

func TestHighlightMsgWithoutCursorIgnored(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(highlightMsg{span: cursor.Highlight{
		Stream: 1, Line: 1, EndLine: 1, StartCol: 1, EndCol: 3,
	}})
	um := updated.(Model)
	if !um.spans.Empty() {
		t.Errorf("highlights should be ignored with the cursor disabled")
	}
}

func TestCursorInitWaitsForHighlights(t *testing.T) {
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{}, WithCursor(make(chan cursor.Highlight, 4), nil))
	if m.Init() == nil {
		t.Fatal("cursor model should schedule commands on Init")
	}

	plain := newTestModel()
	if plain.Init() == nil {
		t.Fatal("plain model should schedule commands on Init")
	}
}

func TestHighlightMsgDroppedWhenPatternAbsent(t *testing.T) {
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{}, WithCursor(make(chan cursor.Highlight, 4), nil))

	// The pattern was evaluated once but has since been deleted from
	// the editor while it keeps sounding.
	tracker.SetPlaying(1, `d1 $ sound "bd sn"`)

	m.textarea.SetValue("-- nothing here")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(highlightMsg{span: cursor.Highlight{
		Stream: 1, Line: 1, EndLine: 1, StartCol: 13, EndCol: 15,
	}})
	um := updated.(Model)

	if !um.spans.Empty() {
		t.Errorf("spans for a pattern absent from the editor should be dropped, got %+v", um.spans.All())
	}
}

func TestHighlightMsgKeptWhilePatternInEditor(t *testing.T) {
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{}, WithCursor(make(chan cursor.Highlight, 4), nil))

	tracker.SetPlaying(1, `d1 $ sound "bd sn"`)

	m.textarea.SetValue(`d1 $ sound "bd sn"`)
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 130, Height: 40})
	m = updated.(Model)

	updated, _ = m.Update(highlightMsg{span: cursor.Highlight{
		Stream: 1, Line: 1, EndLine: 1, StartCol: 13, EndCol: 15,
	}})
	um := updated.(Model)

	if um.spans.Empty() {
		t.Fatal("span for a pattern present in the editor should be kept")
	}
	if got := um.spans.All()[0]; got.Stream != 1 || got.StartCol != 13 || got.EndCol != 15 {
		t.Errorf("unexpected span: %+v", got)
	}
}

func TestSclangWarnMsgLoggedToConsole(t *testing.T) {
	warns := make(chan string, 1)
	warns <- "sc3-plugins not found"
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{}, WithSclangWarnings(warns))

	updated, _ := m.Update(sclangWarnMsg{text: "sc3-plugins not found"})
	um := updated.(Model)

	um.console.SetSize(60, 5)
	if !strings.Contains(um.console.View(), "sc3-plugins not found") {
		t.Errorf("warning missing from console view: %q", um.console.View())
	}
}

func newDispatchModel() Model {
	tracker := status.NewTracker(8)
	tracker.SetCPS(0.6)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	m := NewModel(ui.DarkTheme(), nil, okTransport{}, tracker, panel, console, GhciConfig{})
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return updated.(Model)
}

func altKey(r rune) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}, Alt: true}
}

func TestMuteAllKey(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(altKey('m'))
	um := updated.(Model)
	for n, state := range um.tracker.Streams() {
		if state != status.StreamMuted {
			t.Errorf("alt+m should mute stream %d, got %d", n, state)
		}
	}
}

func TestCtrlMInsertsNewlineNotMuteAll(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlM})
	um := updated.(Model)
	if um.textarea.Value() != "\n" {
		t.Errorf("ctrl+m (enter) should insert a newline, got %q", um.textarea.Value())
	}
	for n, state := range um.tracker.Streams() {
		if state == status.StreamMuted {
			t.Errorf("ctrl+m must not mute stream %d", n)
		}
	}
}

func TestUnmuteAllKey(t *testing.T) {
	m := newDispatchModel()
	m.tracker.SetMuted(1)
	m.tracker.SetMuted(3)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	um := updated.(Model)
	for n, state := range um.tracker.Streams() {
		if state == status.StreamMuted {
			t.Errorf("ctrl+u should unmute stream %d, got %d", n, state)
		}
	}
}

func TestHushKey(t *testing.T) {
	m := newDispatchModel()
	m.tracker.SetPlaying(1, `d1 $ sound "bd"`)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlBackslash})
	um := updated.(Model)
	for n, state := range um.tracker.Streams() {
		if state != status.StreamIdle {
			t.Errorf("ctrl+\\ should idle stream %d, got %d", n, state)
		}
	}
	if len(um.tracker.Playing()) != 0 {
		t.Errorf("ctrl+\\ should clear playing code, got %v", um.tracker.Playing())
	}
}

func TestFKeyTogglesMute(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	um := updated.(Model)
	if um.tracker.Streams()[1] != status.StreamMuted {
		t.Errorf("f1 should mute d1, got %d", um.tracker.Streams()[1])
	}

	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyF1})
	um = updated.(Model)
	if um.tracker.Streams()[1] != status.StreamIdle {
		t.Errorf("second f1 should unmute d1 to idle (nothing playing), got %d", um.tracker.Streams()[1])
	}
}

func TestSoloKey(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(altKey('1'))
	um := updated.(Model)
	if um.tracker.Streams()[1] != status.StreamSoloed {
		t.Errorf("alt+1 should solo d1, got %d", um.tracker.Streams()[1])
	}

	updated, _ = um.Update(altKey('1'))
	um = updated.(Model)
	if um.tracker.Streams()[1] != status.StreamIdle {
		t.Errorf("second alt+1 should unsolo d1 to idle (nothing playing), got %d", um.tracker.Streams()[1])
	}
}

func TestTempoKeys(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(altKey(']'))
	um := updated.(Model)
	if um.tracker.CPS() != 0.65 {
		t.Errorf("alt+] should raise cps to 0.65, got %g", um.tracker.CPS())
	}

	updated, _ = um.Update(altKey('['))
	um = updated.(Model)
	if um.tracker.CPS() != 0.6 {
		t.Errorf("alt+[ should lower cps back to 0.6, got %g", um.tracker.CPS())
	}
}

func TestCursorErrorDisablesCursor(t *testing.T) {
	tracker := status.NewTracker(8)
	panel := status.NewPanel(tracker, ui.DarkTheme().Palette)
	console := status.NewLog(ui.DarkTheme().Palette)
	errs := make(chan error, 1)
	errs <- errTestListener
	m := NewModel(ui.DarkTheme(), nil, nil, tracker, panel, console, GhciConfig{},
		WithCursor(make(chan cursor.Highlight, 4), errs))

	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init should arm commands")
	}
	updated, _ := m.Update(cursorErrMsg{err: errTestListener})
	um := updated.(Model)
	if um.cursorOn {
		t.Error("cursor error should disable the cursor")
	}
}

var errTestListener = errors.New("listen udp: address already in use")

func writeTidalFile(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "set.tidal")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func withDoc(t *testing.T, content string) (Model, *doc.Doc) {
	t.Helper()
	path := writeTidalFile(t, t.TempDir(), content)
	d := doc.New()
	if _, err := d.Load(path); err != nil {
		t.Fatal(err)
	}
	m := newTestModel()
	WithDocument(d)(&m)
	return m, d
}

func typePrompt(m Model, text string) Model {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
	return updated.(Model)
}

func TestCtrlO_OpensOpenPrompt(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	if !um.promptOn {
		t.Fatal("ctrl+o should open the filename prompt")
	}
	if um.promptKind != promptOpen {
		t.Errorf("prompt kind = %v, want promptOpen", um.promptKind)
	}
	if got := um.promptInput.Value(); got != "" {
		t.Errorf("open prompt should start empty, got %q", got)
	}
}

func TestCtrlS_NoPath_OpensSaveAsPrompt(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	um := updated.(Model)
	if !um.promptOn {
		t.Fatal("ctrl+s without a bound file should open the prompt")
	}
	if um.promptKind != promptSaveAs {
		t.Errorf("prompt kind = %v, want promptSaveAs", um.promptKind)
	}
	if got := um.promptInput.Value(); got != untitledFile {
		t.Errorf("save-as prefill = %q, want %q", got, untitledFile)
	}
}

func TestPrompt_TypingGoesToInputNotEditor(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), "d1 $ sound")

	if got := um.textarea.Value(); got != "" {
		t.Errorf("editor should stay empty while the prompt is open, got %q", got)
	}
	if got := um.promptInput.Value(); got != "d1 $ sound" {
		t.Errorf("prompt input = %q, want %q", got, "d1 $ sound")
	}
}

func TestPrompt_EscCancels(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), "set.tidal")
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um = updated.(Model)

	if um.promptOn {
		t.Error("esc should close the prompt")
	}
	if got := um.textarea.Value(); got != "" {
		t.Errorf("cancelling should not touch the editor, got %q", got)
	}
}

func TestPrompt_EnterLoadsFile(t *testing.T) {
	content := "d1 $ sound \"can\"\n"
	path := writeTidalFile(t, t.TempDir(), content)

	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if got := um.textarea.Value(); got != content {
		t.Errorf("editor = %q, want %q", got, content)
	}
	if um.doc.Path() != path {
		t.Errorf("doc path = %q, want %q", um.doc.Path(), path)
	}
	if um.promptOn {
		t.Error("enter should close the prompt")
	}
}

func TestLoad_ControlCharactersDoNotDirtyBuffer(t *testing.T) {
	path := writeTidalFile(t, t.TempDir(), "d1 $ sound \"bd\"\x0c\n")

	m := newTestModel()
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if um.dirty() {
		t.Errorf("loaded file should match the buffer, buffer = %q", um.textarea.Value())
	}
}

func TestPrompt_EnterMissingFileKeepsEditor(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), filepath.Join(t.TempDir(), "missing.tidal"))
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if got := um.textarea.Value(); got != "" {
		t.Errorf("failed open should not touch the editor, got %q", got)
	}
	if um.doc.Path() != "" {
		t.Errorf("failed open should not bind a path, got %q", um.doc.Path())
	}
	if um.promptOn {
		t.Error("enter should close the prompt even on error")
	}
	um.console.SetSize(80, 10)
	if !strings.Contains(um.console.View(), errOpen) {
		t.Errorf("console should report the open error, missing %q", errOpen)
	}
}

func TestPrompt_EmptyPathSubmitKeepsPromptOpen(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if !um.promptOn {
		t.Error("empty submit should keep the prompt open")
	}
	um.console.SetSize(80, 10)
	if !strings.Contains(um.console.View(), errEmptyPath) {
		t.Errorf("console should report the empty path, missing %q", errEmptyPath)
	}
}

func TestCtrlS_SavesToBoundFile(t *testing.T) {
	path := writeTidalFile(t, t.TempDir(), "d1 $ sound \"bd\"\n")
	m := newTestModel()
	d := doc.New()
	if _, err := d.Load(path); err != nil {
		t.Fatal(err)
	}
	WithDocument(d)(&m)

	m.textarea.SetValue("d1 $ sound \"hh\"\n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	um := updated.(Model)

	if um.promptOn {
		t.Fatal("ctrl+s with a bound file should save directly")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "d1 $ sound \"hh\"\n" {
		t.Errorf("file content = %q, want %q", got, "d1 $ sound \"hh\"\n")
	}
}

func TestCtrlS_SaveAs_WritesPrefilledName(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	um := updated.(Model)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	path := filepath.Join(dir, untitledFile)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("save-as should write the prefilled untitled.tidal: %v", err)
	}
	if string(got) != "d1 $ sound \"bd\"\n" {
		t.Errorf("file content = %q, want %q", got, "d1 $ sound \"bd\"\n")
	}
	if um.doc.Path() != path {
		t.Errorf("doc path = %q, want %q", um.doc.Path(), path)
	}
}

// overwriteWarningModel drives the save-as flow to an existing file and
// returns the model with the overwrite warning open, plus the file path.
func overwriteWarningModel(t *testing.T) (Model, string) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	path := filepath.Join(dir, "set.tidal")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel()
	m.textarea.SetValue("new\n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	um := updated.(Model)
	um.promptInput.SetValue(path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)
	if um.confirm != confirmOverwrite {
		t.Fatal("precondition: save-as to an existing file should ask before overwriting")
	}
	return um, path
}

func TestCtrlS_SaveAs_ExistingFileAsksToOverwrite(t *testing.T) {
	um, path := overwriteWarningModel(t)

	if got, _ := os.ReadFile(path); string(got) != "old\n" {
		t.Fatalf("file written before confirmation: %q", got)
	}

	updated, _ := um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("confirming should close the overwrite warning")
	}
	if got, _ := os.ReadFile(path); string(got) != "new\n" {
		t.Errorf("file content = %q, want %q", got, "new\n")
	}
	if um.doc.Path() != path {
		t.Errorf("doc path = %q, want %q", um.doc.Path(), path)
	}
}

func TestCtrlS_SaveAs_OverwriteCancelReturnsToPrompt(t *testing.T) {
	um, path := overwriteWarningModel(t)

	updated, _ := um.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("esc should close the overwrite warning")
	}
	if !um.promptOn {
		t.Error("esc should return to the filename prompt")
	}
	if got := um.promptInput.Value(); got != path {
		t.Errorf("prompt input = %q, want %q", got, path)
	}
	if got, _ := os.ReadFile(path); string(got) != "old\n" {
		t.Errorf("cancelled overwrite changed the file: %q", got)
	}
}

func TestCtrlS_SaveAs_OverwriteCtrlCAsksToQuit(t *testing.T) {
	um, _ := overwriteWarningModel(t)

	updated, _ := um.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("ctrl+c should close the overwrite warning")
	}
	if !um.confirmQ {
		t.Error("ctrl+c while the overwrite warning is open should ask to confirm quit")
	}
}

func TestInterrupt_DuringOverwriteCancelsWarning(t *testing.T) {
	um, _ := overwriteWarningModel(t)

	updated, _ := um.Update(tea.InterruptMsg{})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("SIGINT should close the overwrite warning")
	}
	if !um.confirmQ {
		t.Error("SIGINT while the overwrite warning is open should ask to confirm quit")
	}
}

func TestOverwriteWarning_ShownInView(t *testing.T) {
	um, _ := overwriteWarningModel(t)

	view := um.View()
	if !strings.Contains(view, "File exists") {
		t.Error("warning overlay should say the file exists")
	}
	if !strings.Contains(view, "Enter: overwrite") {
		t.Error("help bar should show the overwrite keys")
	}
}

func TestCtrlO_DirtyBufferAsksBeforeDiscarding(t *testing.T) {
	content := "d1 $ sound \"hh\"\n"
	path := writeTidalFile(t, t.TempDir(), content)
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	um.promptInput.SetValue(path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if um.confirm != confirmDiscard {
		t.Fatalf("opening over unsaved changes should ask first, confirm = %v", um.confirm)
	}
	if got := um.textarea.Value(); got != "d1 $ sound \"bd\"\n" {
		t.Errorf("file loaded before confirmation, editor = %q", got)
	}
	view := um.View()
	if !strings.Contains(view, "Unsaved changes will be lost") {
		t.Error("warning overlay should mention the lost edits")
	}
	if !strings.Contains(view, "Enter: open") {
		t.Error("help bar should show the open confirmation keys")
	}

	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("confirming should close the discard warning")
	}
	if got := um.textarea.Value(); got != content {
		t.Errorf("editor = %q, want %q", got, content)
	}
}

func TestCtrlO_DiscardCancelReturnsToPrompt(t *testing.T) {
	path := writeTidalFile(t, t.TempDir(), "d1 $ sound \"hh\"\n")
	m := newTestModel()
	m.textarea.SetValue("d1 $ sound \"bd\"\n")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	um.promptInput.SetValue(path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um = updated.(Model)
	if um.confirm != confirmNone {
		t.Error("esc should close the discard warning")
	}
	if !um.promptOn {
		t.Error("esc should return to the filename prompt")
	}
	if got := um.textarea.Value(); got != "d1 $ sound \"bd\"\n" {
		t.Errorf("cancelled open should keep the buffer, got %q", got)
	}
}

func TestCtrlS_SaveFailureReportsErrorAndKeepsBinding(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "set.tidal")
	if err := os.WriteFile(path, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel()
	d := doc.New()
	if _, err := d.Load(path); err != nil {
		t.Fatal(err)
	}
	WithDocument(d)(&m)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	m.textarea.SetValue("new\n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlS})
	um := updated.(Model)

	if um.doc.Path() != path {
		t.Errorf("failed save should keep the binding, got %q", um.doc.Path())
	}
	if got := um.textarea.Value(); got != "new\n" {
		t.Errorf("failed save should keep the buffer, got %q", got)
	}
	um.console.SetSize(80, 10)
	if !strings.Contains(um.console.View(), errSave) {
		t.Errorf("console should report the save error, missing %q", errSave)
	}
}

func TestPrompt_CtrlCQuitsAfterCancel(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um = updated.(Model)

	if um.promptOn {
		t.Error("ctrl+c should close the prompt")
	}
	if !um.confirmQ {
		t.Error("ctrl+c while the prompt is open should ask to confirm quit")
	}
}

func TestQuitWarning_MentionsUnsavedChanges(t *testing.T) {
	m := newTestModel()
	m.confirmQ = true

	view := m.View()
	if strings.Contains(view, "Unsaved changes") {
		t.Error("clean buffer should not warn about unsaved changes")
	}

	m.textarea.SetValue("d1 $ sound \"bd\"\n")
	view = m.View()
	if !strings.Contains(view, "Unsaved changes") {
		t.Error("dirty buffer should warn about unsaved changes in the quit notice")
	}
}

func TestInterrupt_DuringPromptCancelsPrompt(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	updated, _ = um.Update(tea.InterruptMsg{})
	um = updated.(Model)

	if um.promptOn {
		t.Error("SIGINT while the prompt is open should close the prompt")
	}
	if !um.confirmQ {
		t.Error("SIGINT while the prompt is open should ask to confirm quit")
	}
}

func TestHelpBar_ShowsFileAndDirtyMark(t *testing.T) {
	m, _ := withDoc(t, "d1 $ sound \"bd\"\n")

	view := m.View()
	if !strings.Contains(view, "set.tidal |") {
		t.Error("help bar should show the bound file name")
	}

	m.textarea.SetValue("d1 $ sound \"hh\"\n")
	view = m.View()
	if !strings.Contains(view, "set.tidal*") {
		t.Error("help bar should mark unsaved changes with *")
	}
}

func TestHelpBar_UntitledDirtyShowsLabel(t *testing.T) {
	m := newTestModel()

	if strings.Contains(m.View(), "untitled") {
		t.Error("clean untitled buffer should show no file label")
	}

	m.textarea.SetValue("d1 $ sound \"bd\"\n")
	if !strings.Contains(m.View(), "untitled*") {
		t.Error("dirty untitled buffer should show a marked file label")
	}
}

func TestPrompt_HelpBarShowsPromptKeys(t *testing.T) {
	m := newTestModel()

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := updated.(Model)
	if !strings.Contains(um.View(), "Esc: cancel") {
		t.Error("help bar should show prompt keys while the prompt is open")
	}
}

func TestLoad_FlagsPlayingPatternsEdited(t *testing.T) {
	m, _ := withDoc(t, "d1 $ sound \"bd sn\"\n")
	m.tracker.SetPlaying(1, "d1 $ sound \"bd sn\"")
	m.refreshEdited()
	if m.tracker.Edited()[1] {
		t.Fatal("precondition: playing pattern should not be edited")
	}

	path := writeTidalFile(t, t.TempDir(), "d1 $ sound \"hh\"\n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if !um.tracker.Edited()[1] {
		t.Error("a playing pattern overwritten by the loaded file should be flagged edited")
	}
}

func TestLoad_ClearsMarkerForRemovedPlayingPattern(t *testing.T) {
	m, _ := withDoc(t, "d1 $ sound \"bd sn\"\n")
	m.tracker.SetPlaying(1, "d1 $ sound \"bd sn\"")
	m.refreshEdited()

	path := writeTidalFile(t, t.TempDir(), "d2 $ sound \"hh\"\n")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	um := typePrompt(updated.(Model), path)
	updated, _ = um.Update(tea.KeyMsg{Type: tea.KeyEnter})
	um = updated.(Model)

	if um.tracker.Edited()[1] {
		t.Error("a playing pattern removed by the loaded file should clear its marker, like deleting it from the editor")
	}
}

var errSend = errors.New("send failed")

// okTransport accepts every transport command, standing in for a
// reachable Tidal control port.
type okTransport struct{}

func (okTransport) Hush() error                { return nil }
func (okTransport) MuteAll() error             { return nil }
func (okTransport) UnmuteAll() error           { return nil }
func (okTransport) Mute(streamNum int) error   { return nil }
func (okTransport) Unmute(streamNum int) error { return nil }
func (okTransport) Solo(streamNum int) error   { return nil }
func (okTransport) Unsolo(streamNum int) error { return nil }
func (okTransport) SetTempo(cps float64) error { return nil }

// errTransport fails every send, standing in for an unreachable Tidal
// control port.
type errTransport struct{ okTransport }

func (errTransport) Hush() error                { return errSend }
func (errTransport) MuteAll() error             { return errSend }
func (errTransport) UnmuteAll() error           { return errSend }
func (errTransport) Mute(streamNum int) error   { return errSend }
func (errTransport) Unmute(streamNum int) error { return errSend }
func (errTransport) Solo(streamNum int) error   { return errSend }
func (errTransport) Unsolo(streamNum int) error { return errSend }
func (errTransport) SetTempo(cps float64) error { return errSend }

func TestToggleMute_RevertsOnSendFailure(t *testing.T) {
	m := newDispatchModel()
	m.oscClient = errTransport{}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyF1})
	um := updated.(Model)
	if um.tracker.Streams()[1] != status.StreamIdle {
		t.Errorf("failed mute should restore the previous state, got %d", um.tracker.Streams()[1])
	}
}

func TestToggleSolo_RevertsToMutedOnSendFailure(t *testing.T) {
	m := newDispatchModel()
	m.oscClient = errTransport{}
	m.tracker.SetMuted(1)

	updated, _ := m.Update(altKey('1'))
	um := updated.(Model)
	if um.tracker.Streams()[1] != status.StreamMuted {
		t.Errorf("failed solo on a muted stream should restore muted, got %d", um.tracker.Streams()[1])
	}
}

func TestTempo_KeepsCPSOnSendFailure(t *testing.T) {
	m := newDispatchModel()
	m.oscClient = errTransport{}

	updated, _ := m.Update(altKey(']'))
	um := updated.(Model)
	if um.tracker.CPS() != 0.6 {
		t.Errorf("failed tempo change should keep cps at 0.6, got %g", um.tracker.CPS())
	}
}

func TestRestartGhci_RunsAsync(t *testing.T) {
	m := newTestModel()
	m.ghciCfg = GhciConfig{Executable: "nonexistent-ghci-binary"}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	um := updated.(Model)
	if !um.restarting {
		t.Errorf("restart should be in flight after ctrl+r")
	}
	if cmd == nil {
		t.Fatalf("restart should return a command")
	}

	msg := cmd()
	rmsg, ok := msg.(ghciRestartedMsg)
	if !ok {
		t.Fatalf("expected ghciRestartedMsg, got %T", msg)
	}
	if rmsg.err == nil {
		t.Errorf("restart with a missing executable should fail")
	}

	updated, _ = um.Update(msg)
	um = updated.(Model)
	if um.restarting {
		t.Errorf("restart should no longer be in flight")
	}
	if um.tracker.GhciOK() {
		t.Errorf("failed restart should mark ghci not ok")
	}
}

func TestRestartGhci_IgnoredWhileRestarting(t *testing.T) {
	m := newTestModel()
	m.restarting = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	um := updated.(Model)
	if cmd != nil {
		t.Errorf("a second ctrl+r should not start another restart")
	}
	if !um.restarting {
		t.Errorf("restart should still be in flight")
	}
}

func TestQuit_DuringRestartWaitsForCompletion(t *testing.T) {
	m := newTestModel()
	m.restarting = true
	m.confirmQ = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	um := updated.(Model)
	if !um.quitting {
		t.Errorf("second ctrl+c should quit")
	}
	if cmd != nil {
		t.Errorf("quit should wait for the in-flight restart")
	}

	updated, cmd = um.Update(ghciRestartedMsg{err: errors.New("boom")})
	if cmd == nil {
		t.Fatalf("restart completion should finish the deferred quit")
	}
	if msg := cmd(); msg != (tea.QuitMsg{}) {
		t.Errorf("expected tea.QuitMsg, got %v", msg)
	}
}

func TestQuitting_IgnoresKeys(t *testing.T) {
	m := newTestModel()
	m.quitting = true
	m.restarting = true

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	um := updated.(Model)
	if cmd != nil {
		t.Errorf("keys should be ignored while quitting")
	}
	if !um.quitting {
		t.Errorf("quitting should stay set")
	}
}

func TestHelpText_FollowsVisibleStreams(t *testing.T) {
	m := newTestModel()
	if got := m.helpText(); !strings.Contains(got, "F1-F8: mute") || !strings.Contains(got, "Alt-1-8: solo") {
		t.Errorf("8 streams should show F1-F8 and Alt-1-8, got %q", got)
	}

	m.tracker = status.NewTracker(4)
	if got := m.helpText(); !strings.Contains(got, "F1-F4: mute") || !strings.Contains(got, "Alt-1-4: solo") {
		t.Errorf("4 streams should show F1-F4 and Alt-1-4, got %q", got)
	}

	m.tracker = status.NewTracker(12)
	if got := m.helpText(); !strings.Contains(got, "F1-F8: mute") {
		t.Errorf("12 streams should cap mute shortcuts at F1-F8, got %q", got)
	}
}

func TestKeyRange(t *testing.T) {
	if got := keyRange("F1", "F8"); got != "F1-F8" {
		t.Errorf("got %q, want F1-F8", got)
	}
	if got := keyRange("F1", "F1"); got != "F1" {
		t.Errorf("single key should not be a range, got %q", got)
	}
}
