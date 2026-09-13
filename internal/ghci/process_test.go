package ghci

import (
	"io"
	"strings"
	"testing"
)

func drainOutput(p *Process) []OutputMsg {
	var out []OutputMsg
	for {
		select {
		case msg := <-p.outputCh:
			out = append(out, msg)
			continue
		default:
		}
		return out
	}
}

func TestReadPipe_KeepsNewestWhenFull(t *testing.T) {
	p := &Process{outputCh: make(chan OutputMsg, 2)}
	p.outputWg.Add(1)

	go p.readPipe(io.NopCloser(strings.NewReader("l0\nl1\nl2\nl3\nl4\n")), pipeKindStdout)
	p.outputWg.Wait()

	got := drainOutput(p)
	if len(got) != 2 {
		t.Fatalf("expected 2 buffered messages, got %d", len(got))
	}
	if got[0].Text != "l3" || got[1].Text != "l4" {
		t.Errorf("expected newest lines l3,l4 kept, got %q", texts(got))
	}
}

func texts(msgs []OutputMsg) []string {
	var out []string
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

func TestNew_MissingExecutable(t *testing.T) {
	_, err := New("/nonexistent/cabal", "", "/tmp")
	if err == nil {
		t.Fatal("expected error for nonexistent executable")
	}
}

func TestEvaluate_NotRunning(t *testing.T) {
	p := &Process{}
	err := p.Evaluate("d1 $ sound \"bd sn\"")
	if err == nil {
		t.Fatal("expected error when evaluating on non-running process")
	}
}

func TestEvaluateMulti_NotRunning(t *testing.T) {
	p := &Process{}
	err := p.EvaluateMulti("d1 $ sound \"bd sn\"\nd2 $ sound \"cp\"")
	if err == nil {
		t.Fatal("expected error when evaluating multi-line on non-running process")
	}
}
