package ghci

import (
	"testing"

	"github.com/pshickeydev/triton/internal/proc"
)

func TestNew_MissingExecutable(t *testing.T) {
	_, err := New("/nonexistent/cabal", "", "/tmp")
	if err == nil {
		t.Fatal("expected error for nonexistent executable")
	}
}

func TestEvaluate_NotRunning(t *testing.T) {
	p := &Process{handle: &proc.Handle{}}
	err := p.Evaluate("d1 $ sound \"bd sn\"")
	if err == nil {
		t.Fatal("expected error when evaluating on non-running process")
	}
}

func TestEvaluateMulti_NotRunning(t *testing.T) {
	p := &Process{handle: &proc.Handle{}}
	err := p.EvaluateMulti("d1 $ sound \"bd sn\"\nd2 $ sound \"cp\"")
	if err == nil {
		t.Fatal("expected error when evaluating multi-line on non-running process")
	}
}
