package model

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/pshickeydev/triton/internal/cursor"
	"github.com/pshickeydev/triton/internal/doc"
	"github.com/pshickeydev/triton/internal/ghci"
	"github.com/pshickeydev/triton/internal/status"
	"github.com/pshickeydev/triton/internal/ui"
)

// reviewEvaluator records the code sent to GHCi and replays canned
// output/done channels, standing in for the process.
type reviewEvaluator struct {
	evaluations []string
	output      chan ghci.OutputMsg
	done        chan struct{}
}

func (e *reviewEvaluator) Output() <-chan ghci.OutputMsg { return e.output }
func (e *reviewEvaluator) Done() <-chan struct{}         { return e.done }
func (e *reviewEvaluator) Close() error                  { return nil }
func (e *reviewEvaluator) Evaluate(code string) error {
	e.evaluations = append(e.evaluations, code)
	return nil
}
func (e *reviewEvaluator) EvaluateMulti(code string) error {
	e.evaluations = append(e.evaluations, code)
	return nil
}

func TestReviewRejectsEvaluationDuringRestart(t *testing.T) {
	evaluator := &reviewEvaluator{done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciProc = evaluator
	m.restarting = true
	m.textarea.SetValue(`d1 $ sound "bd"`)

	updated, _ := m.Update(ctrlMsg('j'))
	updatedModel := updated.(Model)
	if len(evaluator.evaluations) != 0 {
		t.Fatalf("evaluation reached retiring process: %q", evaluator.evaluations)
	}
	if _, ok := updatedModel.tracker.Playing()[1]; ok {
		t.Fatal("restart-pending evaluation was recorded as playing")
	}
}

// A failed global transport command must not leave the tracker claiming
// a state Tidal did not apply.
func TestReviewFailedGlobalTransportPreservesState(t *testing.T) {
	t.Run("hush", func(t *testing.T) {
		m := newDispatchModel()
		m.oscClient = errTransport{}
		m.tracker.SetPlaying(1, `d1 $ sound "bd"`)
		updated, _ := m.Update(ctrlMsg('\\'))
		if _, ok := updated.(Model).tracker.Playing()[1]; !ok {
			t.Fatal("failed hush erased playing state")
		}
	})
	t.Run("mute all", func(t *testing.T) {
		m := newDispatchModel()
		m.oscClient = errTransport{}
		m.tracker.SetPlaying(1, `d1 $ sound "bd"`)
		updated, _ := m.Update(altKey('m'))
		if got := updated.(Model).tracker.Streams()[1]; got != status.StreamActive {
			t.Fatalf("failed mute-all changed stream state to %d", got)
		}
	})
	t.Run("unmute all", func(t *testing.T) {
		m := newDispatchModel()
		m.oscClient = errTransport{}
		m.tracker.SetPlaying(1, `d1 $ sound "bd"`)
		m.tracker.SetMuted(1)
		updated, _ := m.Update(ctrlMsg('u'))
		if got := updated.(Model).tracker.Streams()[1]; got != status.StreamMuted {
			t.Fatalf("failed unmute-all changed stream state to %d", got)
		}
	})
}

func TestReviewClearsPlayingOnGhciExit(t *testing.T) {
	m := newDispatchModel()
	m.tracker.SetPlaying(1, `d1 $ sound "bd"`)
	updated, _ := m.Update(ghciDoneMsg{epoch: m.epoch})
	if _, ok := updated.(Model).tracker.Playing()[1]; ok {
		t.Fatal("dead process left an active playing pattern")
	}
}

// The final output of a dying GHCi must not be lost to the Done signal
// winning the select: output is drained until the channel closes.
func TestReviewDrainsFinalOutputBeforeDone(t *testing.T) {
	for range 32 {
		output := make(chan ghci.OutputMsg, 1)
		output <- ghci.OutputMsg{Text: "final error"}
		close(output)
		done := make(chan struct{})
		close(done)
		m := newTestModel()
		m.ghciProc = &reviewEvaluator{output: output, done: done}
		if _, ok := m.waitForGhciOutput()().(ghciOutputMsg); !ok {
			t.Fatal("done won selection before buffered final output was drained")
		}
	}
}

func TestReviewDocumentedLargeLayoutThreshold(t *testing.T) {
	m := newTestModel()
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 110, Height: 34})
	if mode := updated.(Model).layout.Mode(); mode != ui.ModeLarge {
		t.Fatalf("110x34 layout mode = %v, want large", mode)
	}
}

func TestReviewHighlightCoordinatesAfterLineInsertion(t *testing.T) {
	m := newDispatchModel()
	code := `d1 $ sound "bd"`
	m.textarea.SetValue(code)
	m.markEvaluated(code)
	m.textarea.SetValue("\n" + code)
	m.refreshEdited()
	m.spans.Add(cursor.Highlight{Stream: 1, Line: 1, EndLine: 1, StartCol: 12, EndCol: 14}, time.Now())
	m.refreshSpans()
	if spans := m.spans.All(); len(spans) != 0 {
		t.Fatalf("stale line-1 highlight survived after pattern moved to line 2: %+v", spans)
	}
}

// Valid d-streams beyond the configured visible count stay tracked, so
// they can be highlighted and compared for edits while the panel shows
// only the configured number of streams.
func TestReviewTracksHiddenValidStream(t *testing.T) {
	m := newDispatchModel()
	m.tracker = status.NewTracker(4)
	m.markEvaluated(`d5 $ sound "bd"`)
	if _, ok := m.tracker.Playing()[5]; !ok {
		t.Fatal("visible_streams prevented tracking valid d5 evaluation")
	}
}

// Evaluating must run as a command, not inside Update: a blocked write
// to GHCi must not freeze the update loop. The result records playing
// state when it comes back as evalDoneMsg.
func TestReviewEvaluateRunsAsCommand(t *testing.T) {
	evaluator := &reviewEvaluator{done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciProc = evaluator
	m.textarea.SetValue(`d1 $ sound "bd"`)

	updated, cmd := m.Update(ctrlMsg('j'))
	um := updated.(Model)
	if cmd == nil {
		t.Fatal("ctrl+j should return an evaluation command")
	}
	if len(evaluator.evaluations) != 0 {
		t.Fatal("the write must happen in the command, not in Update")
	}
	if _, ok := um.tracker.Playing()[1]; ok {
		t.Fatal("playing state must be recorded when the send succeeds, not before")
	}

	msg := cmd()
	eval, ok := msg.(evalDoneMsg)
	if !ok {
		t.Fatalf("expected evalDoneMsg, got %T", msg)
	}
	if eval.code != `d1 $ sound "bd"` {
		t.Fatalf("evalDoneMsg should carry the original code, got %q", eval.code)
	}
	if eval.epoch != m.epoch {
		t.Fatalf("evalDoneMsg should carry the model's epoch %d, got %d", m.epoch, eval.epoch)
	}

	updated, _ = um.Update(eval)
	if _, ok := updated.(Model).tracker.Playing()[1]; !ok {
		t.Fatal("a successful evaluation should record playing state")
	}
}

func TestReviewEvaluateFailureLoggedNotPlaying(t *testing.T) {
	evaluator := &reviewEvaluator{done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciProc = evaluator
	m.textarea.SetValue(`d1 $ sound "bd"`)

	updated, cmd := m.Update(ctrlMsg('j'))
	um := updated.(Model)
	msg := cmd().(evalDoneMsg)
	msg.err = errors.New("write timeout")

	updated, _ = um.Update(msg)
	um = updated.(Model)
	if _, ok := um.tracker.Playing()[1]; ok {
		t.Fatal("a failed evaluation should not record playing state")
	}
	um.console.SetSize(80, 10)
	if !strings.Contains(um.console.View(), errEval) {
		t.Errorf("console should report the eval error, got %q", um.console.View())
	}
}

// Evaluation results from a superseded process must be ignored.
func TestReviewStaleEvaluationEpochIgnored(t *testing.T) {
	m := newDispatchModel()
	updated, _ := m.Update(evalDoneMsg{code: `d1 $ sound "bd"`, epoch: m.epoch + 1})
	if _, ok := updated.(Model).tracker.Playing()[1]; ok {
		t.Fatal("a stale-epoch evaluation result must be ignored")
	}
}

// While a restart is in flight, transport commands must not reach the
// retiring process either; editing and saving stay available.
func TestReviewTransportBlockedDuringRestart(t *testing.T) {
	m := newDispatchModel()
	m.ghciProc = &reviewEvaluator{done: make(chan struct{})}
	m.restarting = true
	m.tracker.SetPlaying(1, `d1 $ sound "bd"`)

	for _, msg := range []tea.Msg{ctrlMsg('\\'), altKey('m'),
		ctrlMsg('u'), keyMsg(tea.KeyF1), altKey('1'),
		altKey(']'), altKey('['), ctrlMsg('r')} {
		updated, _ := m.Update(msg)
		um := updated.(Model)
		if got := um.tracker.Streams()[1]; got != status.StreamActive {
			t.Fatalf("transport command %v changed stream 1 to %d during restart", msg, got)
		}
		if got := um.tracker.CPS(); got != 0.6 {
			t.Fatalf("transport command %v changed cps to %g during restart", msg, got)
		}
	}

	// Editing and saving stay available during a restart.
	updated, _ := m.Update(textMsg("x"))
	if got := updated.(Model).textarea.Value(); got != `x` {
		t.Fatalf("editing should keep working during a restart, buffer = %q", got)
	}
}

// A successful restart resets the tracker's cps to the configured tempo
// and tells the fresh GHCi to run at it.
func TestReviewRestartAppliesStartupTempo(t *testing.T) {
	output := make(chan ghci.OutputMsg)
	close(output)
	evaluator := &reviewEvaluator{output: output, done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciCfg = GhciConfig{Tempo: 0.9}
	m.spawnGhci = func(GhciConfig) (Evaluator, error) { return evaluator, nil }

	updated, cmd := m.Update(ctrlMsg('r'))
	um := updated.(Model)
	msg := cmd().(ghciRestartedMsg)
	if msg.err != nil {
		t.Fatalf("restart should succeed: %v", msg.err)
	}

	updated, cmd = um.Update(msg)
	um = updated.(Model)
	if um.tracker.CPS() != 0.9 {
		t.Fatalf("restart should reset cps to the configured tempo, got %g", um.tracker.CPS())
	}
	if cmd == nil {
		t.Fatal("restart should command the fresh GHCi to apply the configured tempo")
	}
	if len(evaluator.evaluations) != 0 {
		t.Fatalf("the setcps send must run in a command, got %q", evaluator.evaluations)
	}

	// The returned command batch carries the tempo command (and the
	// output listener, which reports the fake's closed channel).
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected a command batch, got %T", cmd())
	}
	var applied bool
	for _, sub := range batch {
		switch sub().(type) {
		case tempoAppliedMsg:
			applied = true
		case ghciDoneMsg:
			// expected on the closed output channel
		}
	}
	if !applied {
		t.Fatal("the restart batch should include the startup tempo command")
	}
	if len(evaluator.evaluations) != 1 || evaluator.evaluations[0] != "setcps 0.9" {
		t.Fatalf("fresh GHCi should receive the startup tempo, got %q", evaluator.evaluations)
	}
}

// A failed setcps during restart is reported without undoing the
// tracker's tempo reset.
func TestReviewTempoAppliedFailureLogged(t *testing.T) {
	evaluator := &errorEvaluator{done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciProc = evaluator

	m.tracker.SetCPS(0.9)
	updated, _ := m.Update(tempoAppliedMsg{epoch: m.epoch, err: errSend})
	um := updated.(Model)
	if um.tracker.CPS() != 0.9 {
		t.Fatalf("a failed setcps should keep the tracker tempo, got %g", um.tracker.CPS())
	}
	um.console.SetSize(80, 10)
	if !strings.Contains(um.console.View(), errTempo) {
		t.Errorf("console should report the tempo error, got %q", um.console.View())
	}

	// A result from a superseded process must be ignored.
	updated, _ = um.Update(tempoAppliedMsg{epoch: m.epoch + 1, err: errSend})
	if got := updated.(Model).console.View(); got != um.console.View() {
		t.Errorf("a stale-epoch tempo result must be ignored, got %q", got)
	}
}

// errorEvaluator fails every send, standing in for an unreachable GHCi.
type errorEvaluator struct {
	done chan struct{}
}

func (e *errorEvaluator) Output() <-chan ghci.OutputMsg   { return nil }
func (e *errorEvaluator) Done() <-chan struct{}           { return e.done }
func (e *errorEvaluator) Close() error                    { return nil }
func (e *errorEvaluator) Evaluate(code string) error      { return errSend }
func (e *errorEvaluator) EvaluateMulti(code string) error { return errSend }

// Mute and solo are independent flags with solo precedence, so muting a
// soloed stream keeps it displayed as soloed, and unsoloing it returns
// it to muted.
func TestReviewMuteSoloIndependent(t *testing.T) {
	m := newDispatchModel()

	updated, _ := m.Update(altKey('1'))
	updated, _ = updated.(Model).Update(keyMsg(tea.KeyF1))
	um := updated.(Model)
	if got := um.tracker.Streams()[1]; got != status.StreamSoloed {
		t.Fatalf("muting a soloed stream should keep solo precedence, got %d", got)
	}

	updated, _ = um.Update(altKey('1'))
	um = updated.(Model)
	if got := um.tracker.Streams()[1]; got != status.StreamMuted {
		t.Fatalf("unsoloing a muted stream should return it to muted, got %d", got)
	}
}

// Global mute keeps solo flags (solo precedence) and global unmute
// leaves soloed streams alone.
func TestReviewMuteAllKeepsSolo(t *testing.T) {
	m := newDispatchModel()
	m.tracker.SetPlaying(2, `d2 $ sound "cp"`)

	updated, _ := m.Update(altKey('1'))
	updated, _ = updated.(Model).Update(altKey('m'))
	um := updated.(Model)
	if got := um.tracker.Streams()[1]; got != status.StreamSoloed {
		t.Fatalf("muteAll should keep the soloed stream soloed, got %d", got)
	}
	if got := um.tracker.Streams()[2]; got != status.StreamMuted {
		t.Fatalf("muteAll should mute d2, got %d", got)
	}

	updated, _ = um.Update(ctrlMsg('u'))
	um = updated.(Model)
	if got := um.tracker.Streams()[1]; got != status.StreamSoloed {
		t.Fatalf("unmuteAll should leave the soloed stream alone, got %d", got)
	}
	if got := um.tracker.Streams()[2]; got != status.StreamActive {
		t.Fatalf("unmuteAll should restore the playing d2, got %d", got)
	}
}

// A file passed on the command line is unevaluated at startup, just like
// one opened interactively, so its patterns start marked with *.
func TestReviewCommandLineDocumentStartsUnevaluated(t *testing.T) {
	document := doc.New()
	path := filepath.Join(t.TempDir(), "startup.tidal")
	if err := document.Save(path, `d1 $ sound "bd"`); err != nil {
		t.Fatal(err)
	}
	if _, err := document.Load(path); err != nil {
		t.Fatal(err)
	}

	m := newDispatchModel()
	WithDocument(document)(&m)
	if !m.tracker.Edited()[1] {
		t.Fatal("a command-line-loaded pattern should start marked unevaluated")
	}
}

type closingEvaluator struct {
	evaluations []string
	done        chan struct{}
	closeCount  int
}

func (e *closingEvaluator) Output() <-chan ghci.OutputMsg   { return nil }
func (e *closingEvaluator) Done() <-chan struct{}           { return e.done }
func (e *closingEvaluator) Evaluate(code string) error      { return nil }
func (e *closingEvaluator) EvaluateMulti(code string) error { return nil }

func (e *closingEvaluator) Close() error {
	e.closeCount++
	return nil
}

func TestModelCloseClosesCurrentGhci(t *testing.T) {
	original := &closingEvaluator{done: make(chan struct{})}
	m := newDispatchModel()
	m.ghciProc = original
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if original.closeCount != 1 {
		t.Fatalf("original close count = %d, want 1", original.closeCount)
	}

	replacement := &closingEvaluator{done: make(chan struct{})}
	m.ghciProc = replacement
	if err := m.Close(); err != nil {
		t.Fatal(err)
	}
	if replacement.closeCount != 1 {
		t.Fatalf("replacement close count = %d, want 1", replacement.closeCount)
	}
	if original.closeCount != 1 {
		t.Fatalf("replacement close should not reclose the original, original count = %d", original.closeCount)
	}
}
