package model

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/pshickeydev/triton/internal/cursor"
	"github.com/pshickeydev/triton/internal/deltamini"
	"github.com/pshickeydev/triton/internal/doc"
	"github.com/pshickeydev/triton/internal/ghci"
	"github.com/pshickeydev/triton/internal/osc"
	"github.com/pshickeydev/triton/internal/status"
	"github.com/pshickeydev/triton/internal/textarea"
	"github.com/pshickeydev/triton/ui"
)

const (
	helpBarHeight = 2
	panePadding   = 2

	tempoStep            = 0.05
	minTempo             = 0.1
	placeholderText      = `d1 $ sound "bd sn"`
	errPrefix            = "!! "
	statusPrefix         = "-- "
	errGhciExited        = "GHCi process exited"
	errEval              = "eval error: "
	errHush              = "hush error: "
	errMuteAll           = "muteAll error: "
	errUnmuteAll         = "unmuteAll error: "
	errMuteToggle        = "mute toggle error: "
	errSoloToggle        = "solo toggle error: "
	errTempo             = "tempo error: "
	errGhciRestart       = "failed to restart ghci: "
	errCursorListener    = "cursor listener: "
	msgGhciRestarting    = "restarting GHCi..."
	msgGhciRestarted     = "GHCi restarted"
	quitWarningText      = "Press Ctrl-C again to quit, Esc to cancel"
	quitDirtyWarningText = "Unsaved changes. " + quitWarningText
	shutdownText         = "Closing GHCi..."
	shutdownTextSC       = "Closing GHCi and sclang..."
	markerHint           = statusPrefix + "* marks a pattern with edits not yet evaluated"
	helpTextFmt          = " Ctrl-J: eval block | Ctrl-L: eval line | Ctrl-\\: hush | %s: mute | Alt-M: mute all | Ctrl-U: unmute all | %s: solo | Ctrl-R: restart | Tab: focus | PgUp/PgDn: scroll | Ctrl-C: quit"
	helpTextSmallFmt     = " Ctrl-J: eval block | Ctrl-L: eval line | Ctrl-\\: hush | %s: mute | Alt-M: mute all | Ctrl-U: unmute all | %s: solo | Ctrl-R: restart | Tab: switch view | PgUp/PgDn: scroll | Ctrl-C: quit"

	// keyStreamCount is the number of d-streams reachable through F-key
	// mute and Alt-digit solo shortcuts.
	keyStreamCount = 8

	patternAssign = `^\s*d(\d+)\s*\$`

	// Highlight spans live while Tidal keeps refreshing them: they are
	// dropped after ~2 silent clock frames, checked every frame.
	spanTTL  = 80 * time.Millisecond
	spanTick = 40 * time.Millisecond
)

var patternRe = regexp.MustCompile(patternAssign)

var (
	muteKeys = streamKeys("f")
	soloKeys = streamKeys("alt+")
)

// streamKeys maps shortcut key names to stream numbers 1..keyStreamCount.
func streamKeys(prefix string) map[string]int {
	keys := make(map[string]int, keyStreamCount)
	for i := 1; i <= keyStreamCount; i++ {
		keys[prefix+strconv.Itoa(i)] = i
	}
	return keys
}

type focusArea int

const (
	focusEditor focusArea = iota
	focusStatus
	focusConsole
)

type evalMode int

const (
	evalBlock evalMode = iota
	evalSingleLine
)

type viewArea int

const (
	viewEditor viewArea = iota
	viewStatusConsole
)

type focusStep int

const (
	focusNext focusStep = iota
	focusPrev
)

type ghciOutputMsg struct {
	text    string
	isError bool
	epoch   int
}

type ghciDoneMsg struct {
	epoch int
}

// ghciRestartedMsg delivers the outcome of an asynchronous GHCi restart.
type ghciRestartedMsg struct {
	proc *ghci.Process
	err  error
}

// sclangWarnMsg carries a startup warning from the SuperDirt process,
// such as a missing sc3-plugins installation.
type sclangWarnMsg struct {
	text string
}

// highlightMsg carries one decoded OSCContext message from Tidal.
type highlightMsg struct {
	span cursor.Highlight
}

// spanTickMsg expires highlight spans that stopped being refreshed.
type spanTickMsg struct{}

// cursorDoneMsg reports the highlight channel closing.
type cursorDoneMsg struct{}

// cursorErrMsg reports the cursor OSC listener failing (e.g. the port is
// already in use). The cursor is disabled afterwards.
type cursorErrMsg struct {
	err error
}

type Model struct {
	textarea    textarea.Model
	layout      *ui.Layout
	styles      ui.Styles
	theme       ui.Theme
	focus       focusArea
	view        viewArea
	width       int
	height      int
	ghciProc    *ghci.Process
	oscClient   *osc.Client
	tracker     *status.Tracker
	panel       *status.Panel
	console     *status.Log
	ghciCfg     GhciConfig
	quitting    bool
	confirmQ    bool
	doc         *doc.Doc
	promptOn    bool
	promptKind  promptKind
	promptInput textinput.Model
	confirm     confirmKind
	pendingPath string
	sclang      bool
	epoch       int
	restarting  bool
	spans       cursor.Spans
	segs        map[int][]string
	segsFor     string
	hlCh        <-chan cursor.Highlight
	warnCh      <-chan string
	cursorErr   <-chan error
	cursorOn    bool
	tickOn      bool
}

type GhciConfig struct {
	Executable string
	BootScript string
	WorkingDir string
}

func NewModel(theme ui.Theme, ghciProc *ghci.Process, oscClient *osc.Client, tracker *status.Tracker, panel *status.Panel, console *status.Log, ghciCfg GhciConfig, opts ...Option) Model {
	styles := ui.NewStyles(theme)
	layout := ui.NewLayout()

	ta := textarea.New()
	ta.Placeholder = placeholderText
	ta.Prompt = ""
	ta.ShowLineNumbers = true
	ta.CharLimit = 0
	ta.Focus()

	m := Model{
		textarea:    ta,
		layout:      layout,
		styles:      styles,
		theme:       theme,
		focus:       focusEditor,
		view:        viewEditor,
		ghciProc:    ghciProc,
		oscClient:   oscClient,
		tracker:     tracker,
		panel:       panel,
		console:     console,
		ghciCfg:     ghciCfg,
		epoch:       0,
		doc:         doc.New(),
		promptInput: newPromptInput(),
	}
	for _, opt := range opts {
		opt(&m)
	}
	m.console.AddOutput(markerHint)
	return m
}

type Option func(*Model)

// WithSclang records that the TUI manages an sclang subprocess, so the
// shutdown message can mention it.
func WithSclang(managed bool) Option {
	return func(m *Model) {
		m.sclang = managed
	}
}

// WithCursor enables the pattern tracing cursor: evaluated code is
// rewritten with buffer offsets and incoming highlight spans are rendered
// in the editor. The highlights channel delivers decoded OSCContext
// messages from the cursor listener; errs reports a listener failure, after
// which the cursor is disabled.
func WithCursor(highlights <-chan cursor.Highlight, errs <-chan error) Option {
	return func(m *Model) {
		m.hlCh = highlights
		m.cursorErr = errs
		m.cursorOn = true
	}
}

// WithSclangWarnings delivers sclang startup warnings, such as a missing
// sc3-plugins installation, to the console log. The channel comes from
// superdirt.Process.Warnings.
func WithSclangWarnings(warnings <-chan string) Option {
	return func(m *Model) {
		m.warnCh = warnings
	}
}

// WithDocument binds the editor to a document and preloads its content
// into the buffer.
func WithDocument(d *doc.Doc) Option {
	return func(m *Model) {
		m.doc = d
		m.textarea.SetValue(d.Content())
	}
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{textarea.Blink, m.waitForGhciOutput()}
	if m.warnCh != nil {
		cmds = append(cmds, m.waitForSclangWarning())
	}
	if m.cursorOn {
		cmds = append(cmds, m.waitForHighlights())
		if m.cursorErr != nil {
			cmds = append(cmds, m.waitForCursorError())
		}
	}
	return tea.Batch(cmds...)
}

func (m Model) waitForSclangWarning() tea.Cmd {
	return func() tea.Msg {
		text, ok := <-m.warnCh
		if !ok {
			return nil
		}
		return sclangWarnMsg{text: text}
	}
}

func (m Model) waitForHighlights() tea.Cmd {
	return func() tea.Msg {
		hl, ok := <-m.hlCh
		if !ok {
			return cursorDoneMsg{}
		}
		return highlightMsg{span: hl}
	}
}

func (m Model) waitForCursorError() tea.Cmd {
	return func() tea.Msg {
		if err := <-m.cursorErr; err != nil {
			return cursorErrMsg{err: err}
		}
		return nil
	}
}

func nextSpanTick() tea.Cmd {
	return tea.Tick(spanTick, func(time.Time) tea.Msg {
		return spanTickMsg{}
	})
}

func (m Model) waitForGhciOutput() tea.Cmd {
	epoch := m.epoch
	return func() tea.Msg {
		select {
		case out, ok := <-m.ghciProc.Output():
			if !ok {
				return ghciDoneMsg{epoch: epoch}
			}
			return ghciOutputMsg{text: out.Text, isError: out.IsError, epoch: epoch}
		case <-m.ghciProc.Done():
			return ghciDoneMsg{epoch: epoch}
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.quitting {
			// Shutdown is in progress (a deferred quit waits for a
			// pending GHCi restart); ignore further input.
			return m, nil
		}
		if m.promptOn {
			return m.handlePromptKey(msg)
		}
		if m.confirm != confirmNone {
			return m.handleConfirmKey(msg)
		}
		if m.confirmQ {
			switch msg.String() {
			case "ctrl+c":
				return m.confirmQuit()
			case "esc":
				m.confirmQ = false
			}
			return m, nil
		}

		if stream, ok := muteKeys[msg.String()]; ok {
			return m.handleToggleMute(stream)
		}
		if stream, ok := soloKeys[msg.String()]; ok {
			return m.handleToggleSolo(stream)
		}

		switch msg.String() {
		case "ctrl+c":
			m.confirmQ = true
			return m, nil
		case "ctrl+j":
			return m.handleEvaluate(evalBlock)
		case "ctrl+l":
			return m.handleEvaluate(evalSingleLine)
		case "ctrl+\\":
			return m.handleHush()
		case "ctrl+r":
			return m.handleRestartGhci()
		case "ctrl+o":
			cmd := m.openPrompt(promptOpen)
			return m, cmd
		case "ctrl+s":
			return m.handleSave()
		case "ctrl+u":
			return m.handleUnmuteAll()
		case "alt+m":
			return m.handleMuteAll()
		case "tab":
			if m.layout.Mode() == ui.ModeSmall {
				cmds = append(cmds, m.toggleSmallView())
				break
			}
			cmds = append(cmds, m.cycleFocus(focusNext))
		case "shift+tab":
			if m.layout.Mode() == ui.ModeSmall {
				cmds = append(cmds, m.toggleSmallView())
				break
			}
			cmds = append(cmds, m.cycleFocus(focusPrev))
		case "alt+]":
			return m.handleTempoUp()
		case "alt+[":
			return m.handleTempoDown()
		}

	case tea.InterruptMsg:
		// SIGINT received as an OS signal (non-TTY input); mirror ctrl+c.
		if m.confirm != confirmNone {
			m.confirm = confirmNone
			m.confirmQ = true
			return m, nil
		}
		if m.promptOn {
			m.promptOn = false
			m.confirmQ = true
			return m, nil
		}
		if m.confirmQ {
			return m.confirmQuit()
		}
		m.confirmQ = true
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layout.SetSize(msg.Width, msg.Height-helpBarHeight)

		m.textarea.SetWidth(m.layout.EditorWidth() - panePadding)
		m.textarea.SetHeight(m.layout.TopHeight() - panePadding)
		m.panel.SetSize(m.layout.StatusWidth()-panePadding, m.layout.StatusHeight()-panePadding)
		m.console.SetSize(m.layout.StatusWidth()-panePadding, m.layout.LogHeight()-panePadding)

		if m.layout.Mode() == ui.ModeSmall {
			if m.focus == focusStatus || m.focus == focusConsole {
				m.view = viewStatusConsole
			} else {
				m.view = viewEditor
			}
		}

	case ghciOutputMsg:
		if msg.epoch != m.epoch {
			return m, nil
		}
		if msg.isError {
			m.console.AddError(msg.text)
		} else {
			m.console.AddOutput(msg.text)
		}
		return m, m.waitForGhciOutput()

	case ghciDoneMsg:
		if msg.epoch != m.epoch {
			return m, nil
		}
		m.tracker.SetGhciOK(false)
		m.console.AddOutput(errPrefix + errGhciExited)

	case ghciRestartedMsg:
		m.restarting = false
		if msg.err != nil {
			m.console.AddOutput(errPrefix + errGhciRestart + msg.err.Error())
		} else {
			m.ghciProc = msg.proc
			m.tracker.SetGhciOK(true)
			m.console.AddOutput(statusPrefix + msgGhciRestarted)
		}
		if m.quitting {
			// A quit was requested while the restart was in flight;
			// complete it now that the process handle is settled.
			return m, m.quitCmd()
		}
		if msg.err != nil {
			return m, nil
		}
		return m, m.waitForGhciOutput()

	case sclangWarnMsg:
		m.console.AddError(msg.text)
		cmds = append(cmds, m.waitForSclangWarning())

	case highlightMsg:
		if !m.cursorOn {
			break
		}
		if !m.tracker.Edited()[msg.span.Stream] {
			m.spans.Add(msg.span, time.Now())
		}
		cmds = append(cmds, m.waitForHighlights())
		if !m.tickOn && !m.spans.Empty() {
			m.tickOn = true
			cmds = append(cmds, nextSpanTick())
		}

	case spanTickMsg:
		if !m.cursorOn {
			break
		}
		if m.spans.Prune(time.Now(), spanTTL) {
			m.refreshSpans()
		}
		if m.spans.Empty() {
			m.tickOn = false
		} else {
			cmds = append(cmds, nextSpanTick())
		}

	case cursorDoneMsg:
		m.tickOn = false

	case cursorErrMsg:
		m.cursorOn = false
		m.tickOn = false
		m.spans.Clear()
		m.refreshSpans()
		m.console.AddError(errCursorListener + msg.err.Error())
	}

	if m.focus == focusEditor && m.editorVisible() {
		before := m.textarea.Value()
		var cmd tea.Cmd
		m.textarea, cmd = m.textarea.Update(msg)
		if m.textarea.Value() != before {
			m.refreshEdited()
		}
		cmds = append(cmds, cmd)
	}

	if m.focus == focusConsole {
		var cmd tea.Cmd
		m.console, cmd = m.console.Update(msg)
		cmds = append(cmds, cmd)
	}

	if m.cursorOn {
		m.refreshSpans()
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		return m.styles.Help.Render(m.shutdownMessage())
	}

	if m.layout.Mode() == ui.ModeTooSmall {
		main := m.layout.RenderTooSmall(m.styles)
		main = m.overlays(main)
		return main + "\n" + m.layout.HelpBar(m.styles, m.helpMsg())
	}

	editorView := m.textarea.View()
	statusView := m.panel.View()
	logView := m.console.View()

	focused := m.focusedPane()

	var main string
	if m.layout.Mode() == ui.ModeSmall {
		main = m.layout.RenderSmall(editorView, statusView, logView, m.styles, focused, m.activePane())
	} else {
		main = m.layout.Render(editorView, statusView, logView, m.styles, focused, 0)
		if m.layout.NeedsWidthAdjust(main) {
			overflow := lipgloss.Width(main) - m.width
			main = m.layout.Render(editorView, statusView, logView, m.styles, focused, overflow)
		}
	}

	help := m.layout.HelpBar(m.styles, m.helpMsg())

	return m.overlays(main) + "\n" + help
}

func (m Model) focusedPane() ui.Pane {
	switch m.focus {
	case focusStatus:
		return ui.PaneStatus
	case focusConsole:
		return ui.PaneConsole
	}
	return ui.PaneEditor
}

func (m Model) activePane() ui.Pane {
	if m.view == viewStatusConsole {
		return ui.PaneStatus
	}
	return ui.PaneEditor
}

// confirmQuit closes GHCi and exits. While a restart is in flight the
// quit is deferred: the ghciRestartedMsg handler completes it, so the
// replacement process is never leaked.
func (m Model) confirmQuit() (tea.Model, tea.Cmd) {
	m.quitting = true
	if m.restarting {
		return m, nil
	}
	return m, m.quitCmd()
}

// quitCmd closes the current GHCi process and then quits, so the shutdown
// message stays on screen while the process exits.
func (m Model) quitCmd() tea.Cmd {
	proc := m.ghciProc
	return func() tea.Msg {
		if proc != nil {
			proc.Close()
		}
		return tea.Quit()
	}
}

func (m Model) shutdownMessage() string {
	if m.sclang {
		return shutdownTextSC
	}
	return shutdownText
}

// overlays renders the filename prompt and confirmation warnings on top
// of the main view.
func (m Model) overlays(main string) string {
	if m.promptOn {
		main = m.layout.Overlay(main, m.promptView())
	}
	if m.confirm != confirmNone {
		main = m.layout.Overlay(main, m.styles.Warning.Render(m.confirmWarning()))
	}
	if m.confirmQ {
		main = m.layout.Overlay(main, m.styles.Warning.Render(m.quitWarning()))
	}
	return main
}

// quitWarning shows the double-press quit notice, mentioning unsaved
// changes first.
func (m Model) quitWarning() string {
	if m.dirty() {
		return quitDirtyWarningText
	}
	return quitWarningText
}

// helpMsg builds the help bar text: contextual keys while the filename
// prompt or a confirmation warning is open, the file label ahead of the
// key help otherwise.
func (m Model) helpMsg() string {
	if m.promptOn {
		return promptHelpText
	}
	if m.confirm == confirmDiscard {
		return discardHelpText
	}
	if m.confirm == confirmOverwrite {
		return overwriteHelpText
	}
	text := m.helpText()
	if m.doc.Path() != "" || m.dirty() {
		text = m.fileLabel() + " |" + text
	}
	return text
}

// helpText names the mute and solo shortcut ranges for the configured
// number of visible streams, capped at the available shortcut keys.
func (m Model) helpText() string {
	n := min(len(m.tracker.Streams()), keyStreamCount)
	mute := keyRange("F1", "F"+strconv.Itoa(n))
	solo := keyRange("Alt-1", strconv.Itoa(n))
	if m.layout.Mode() == ui.ModeSmall {
		return fmt.Sprintf(helpTextSmallFmt, mute, solo)
	}
	return fmt.Sprintf(helpTextFmt, mute, solo)
}

// keyRange names a shortcut range, e.g. keyRange("F1", "F8") is "F1-F8".
func keyRange(first, last string) string {
	if first == last {
		return first
	}
	return first + "-" + last
}

// toggleSmallView swaps the single visible pane in the small layout between
// the editor and the status+console stack, moving focus with it.
func (m *Model) toggleSmallView() tea.Cmd {
	if m.view == viewEditor {
		m.view = viewStatusConsole
		m.focus = focusConsole
		m.textarea.Blur()
		return nil
	}
	m.view = viewEditor
	m.focus = focusEditor
	return m.textarea.Focus()
}

// cycleFocus moves pane focus one step across editor, status, and console
// in the large layout.
func (m *Model) cycleFocus(step focusStep) tea.Cmd {
	order := []focusArea{focusEditor, focusStatus, focusConsole}
	i := slices.Index(order, m.focus)
	if step == focusPrev {
		i += len(order) - 1
	} else {
		i++
	}
	m.focus = order[i%len(order)]
	if m.focus == focusEditor {
		return m.textarea.Focus()
	}
	m.textarea.Blur()
	return nil
}

func (m Model) editorVisible() bool {
	if m.layout.Mode() == ui.ModeTooSmall {
		return false
	}
	return m.layout.Mode() != ui.ModeSmall || m.view == viewEditor
}

// refreshSpans syncs the active highlight spans into the editor. Spans
// whose positions no longer match the editor are suppressed: streams
// flagged edited, and streams whose playing code is not in the editor
// (e.g. the pattern text was deleted while it keeps sounding). Spans
// only render when the editor pane is visible.
func (m *Model) refreshSpans() {
	if m.spans.Empty() {
		m.textarea.SetHighlights(nil)
		return
	}
	m.dropStaleSpans()
	if !m.editorVisible() {
		m.textarea.SetHighlights(nil)
		return
	}
	m.textarea.SetHighlights(m.spanHighlights())
}

// dropStaleSpans removes spans whose buffer positions went stale.
func (m *Model) dropStaleSpans() {
	segments := m.streamSegs()
	playing := m.tracker.Playing()
	edited := m.tracker.Edited()
	m.spans.DropIf(func(span cursor.Span) bool {
		if edited[span.Stream] {
			return true
		}
		code, wasPlaying := playing[span.Stream]
		if !wasPlaying {
			return true
		}
		segs, inEditor := segments[span.Stream]
		return !inEditor || !containsSegment(segs, normalizeCode(code))
	})
}

// spanHighlights converts the active spans into editor highlights, styled
// as reverse video in the span's stream color.
func (m *Model) spanHighlights() []textarea.Highlight {
	spans := m.spans.All()
	highlights := make([]textarea.Highlight, 0, len(spans))
	for _, span := range spans {
		highlights = append(highlights, textarea.Highlight{
			Line:    span.Line,
			EndLine: span.EndLine,
			Start:   span.StartCol,
			End:     span.EndCol,
			Style:   lipgloss.NewStyle().Reverse(true).Foreground(m.theme.StreamColor(span.Stream)),
		})
	}
	return highlights
}

func (m Model) handleEvaluate(mode evalMode) (tea.Model, tea.Cmd) {
	lines := strings.Split(m.textarea.Value(), "\n")
	lineNum := m.textarea.Line()

	var code string
	var startLine int
	if mode == evalSingleLine {
		if lineNum >= 0 && lineNum < len(lines) {
			code = lines[lineNum]
		}
		startLine = lineNum
	} else {
		code, startLine = extractBlock(lines, lineNum)
	}

	if strings.TrimSpace(code) == "" {
		return m, nil
	}

	err := m.evaluate(code, startLine)
	if err != nil {
		m.console.AddOutput(errPrefix + errEval + err.Error())
		return m, nil
	}
	m.markEvaluated(code)
	m.refreshSpans()
	return m, nil
}

// evaluate sends code to GHCi. With the cursor enabled, the code is first
// rewritten so mininotation events report buffer-absolute positions; only
// the text written to GHCi is rewritten, the original is recorded for the
// playing-vs-edited comparison.
func (m Model) evaluate(code string, startLine int) error {
	if m.cursorOn {
		code = deltamini.Delta(code, startLine)
	}
	if strings.Contains(code, "\n") {
		return m.ghciProc.EvaluateMulti(code)
	}
	return m.ghciProc.Evaluate(code)
}

// markEvaluated records the evaluated code for each stream it assigns so
// the status panel can tell playing patterns from pending edits. When a
// code block assigns the same stream more than once, the last assignment
// is what ends up playing.
func (m Model) markEvaluated(code string) {
	for n, segs := range streamSegments(code) {
		m.tracker.SetPlaying(n, segs[len(segs)-1])
	}
	m.refreshEdited()
}

// refreshEdited flags each stream whose editor code differs from the
// code last evaluated for it: an edited pattern is not yet playing. The
// marker clears when the playing code still appears in the editor as a
// segment for that stream.
func (m *Model) refreshEdited() {
	segments := m.streamSegs()
	playing := m.tracker.Playing()
	for n := range m.tracker.Streams() {
		segs, inEditor := segments[n]
		if !inEditor {
			m.tracker.ClearEdited(n)
			continue
		}
		evaluated, wasPlaying := playing[n]
		if !wasPlaying || !containsSegment(segs, normalizeCode(evaluated)) {
			m.tracker.SetEdited(n)
			continue
		}
		m.tracker.ClearEdited(n)
	}
}

func containsSegment(segs []string, normalizedCode string) bool {
	return slices.Contains(segs, normalizedCode)
}

// streamSegs returns the buffer's per-stream segments, normalized for
// comparison. The segmentation is cached and recomputed only when the
// buffer changed, so cursor highlight ticks do not reparse the buffer.
func (m *Model) streamSegs() map[int][]string {
	value := m.textarea.Value()
	if m.segs == nil || value != m.segsFor {
		m.segs = normalizedSegments(streamSegments(value))
		m.segsFor = value
	}
	return m.segs
}

// normalizedSegments collapses whitespace in every segment.
func normalizedSegments(segments map[int][]string) map[int][]string {
	for _, segs := range segments {
		for i, seg := range segs {
			segs[i] = normalizeCode(seg)
		}
	}
	return segments
}

// streamSegments splits code into per-stream pattern segments. A segment
// starts at a d-stream assignment line and runs until the next assignment,
// a blank line, or the end of the code, so continuation lines belong to the
// pattern above them. Streams assigned more than once keep every segment,
// in order.
func streamSegments(code string) map[int][]string {
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

// normalizeCode collapses whitespace so patterns that differ only in
// indentation and spacing compare as equal.
func normalizeCode(code string) string {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		lines[i] = strings.Join(strings.Fields(line), " ")
	}
	return strings.Join(lines, "\n")
}

func extractBlock(lines []string, cursorLine int) (string, int) {
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

func (m Model) handleHush() (tea.Model, tea.Cmd) {
	if err := m.oscClient.Hush(); err != nil {
		m.console.AddOutput(errPrefix + errHush + err.Error())
	}
	m.tracker.HushAll()
	m.spans.Clear()
	m.refreshEdited()
	m.refreshSpans()
	return m, nil
}

func (m Model) handleMuteAll() (tea.Model, tea.Cmd) {
	if err := m.oscClient.MuteAll(); err != nil {
		m.console.AddOutput(errPrefix + errMuteAll + err.Error())
	}
	for n := range m.tracker.Streams() {
		m.tracker.SetMuted(n)
	}
	return m, nil
}

func (m Model) handleUnmuteAll() (tea.Model, tea.Cmd) {
	if err := m.oscClient.UnmuteAll(); err != nil {
		m.console.AddOutput(errPrefix + errUnmuteAll + err.Error())
	}
	m.tracker.UnmuteAll()
	return m, nil
}

// handleToggleMute toggles a stream's mute in the tracker and tells
// Tidal. When the OSC send fails the tracker is restored to its previous
// state so the panel never claims a state Tidal did not apply.
func (m Model) handleToggleMute(streamNum int) (tea.Model, tea.Cmd) {
	prev, ok := m.tracker.Streams()[streamNum]
	if !ok {
		return m, nil
	}
	newState := m.tracker.ToggleMute(streamNum)
	var err error
	if newState == status.StreamMuted {
		err = m.oscClient.Mute(streamNum)
	} else {
		err = m.oscClient.Unmute(streamNum)
	}
	if err == nil {
		return m, nil
	}
	m.tracker.ToggleMute(streamNum)
	if prev == status.StreamSoloed {
		m.tracker.SetSoloed(streamNum)
	}
	m.console.AddOutput(errPrefix + errMuteToggle + err.Error())
	return m, nil
}

// handleToggleSolo toggles a stream's solo in the tracker and tells Tidal,
// restoring the previous state when the OSC send fails.
func (m Model) handleToggleSolo(streamNum int) (tea.Model, tea.Cmd) {
	prev, ok := m.tracker.Streams()[streamNum]
	if !ok {
		return m, nil
	}
	newState := m.tracker.ToggleSolo(streamNum)
	var err error
	if newState == status.StreamSoloed {
		err = m.oscClient.Solo(streamNum)
	} else {
		err = m.oscClient.Unsolo(streamNum)
	}
	if err == nil {
		return m, nil
	}
	m.tracker.ToggleSolo(streamNum)
	if prev == status.StreamMuted {
		m.tracker.SetMuted(streamNum)
	}
	m.console.AddOutput(errPrefix + errSoloToggle + err.Error())
	return m, nil
}

// handleRestartGhci restarts GHCi asynchronously: the close-and-respawn
// runs in a command so a slow shutdown does not freeze the UI. The epoch
// bump invalidates pending output from the dying process.
func (m Model) handleRestartGhci() (tea.Model, tea.Cmd) {
	if m.restarting {
		return m, nil
	}
	m.console.AddOutput(statusPrefix + msgGhciRestarting)
	m.epoch++
	m.restarting = true
	m.tracker.HushAll()
	m.tracker.SetGhciOK(false)
	m.spans.Clear()
	m.refreshEdited()
	m.refreshSpans()

	old := m.ghciProc
	cfg := m.ghciCfg
	return m, func() tea.Msg {
		if old != nil {
			old.Close()
		}
		proc, err := ghci.New(cfg.Executable, cfg.BootScript, cfg.WorkingDir)
		return ghciRestartedMsg{proc: proc, err: err}
	}
}

func (m Model) handleTempoUp() (tea.Model, tea.Cmd) {
	newCPS := m.tracker.CPS() + tempoStep
	if err := m.oscClient.SetTempo(newCPS); err != nil {
		m.console.AddOutput(errPrefix + errTempo + err.Error())
		return m, nil
	}
	m.tracker.SetCPS(newCPS)
	return m, nil
}

func (m Model) handleTempoDown() (tea.Model, tea.Cmd) {
	newCPS := m.tracker.CPS() - tempoStep
	if newCPS < minTempo {
		newCPS = minTempo
	}
	if err := m.oscClient.SetTempo(newCPS); err != nil {
		m.console.AddOutput(errPrefix + errTempo + err.Error())
		return m, nil
	}
	m.tracker.SetCPS(newCPS)
	return m, nil
}
