package proc

import (
	"io"
	"strings"
	"testing"
)

func TestReadPipe_KeepsNewestWhenFull(t *testing.T) {
	h := &Handle{outputCh: make(chan Line, 2), name: "child"}
	h.outputWg.Add(1)

	go h.readPipe(io.NopCloser(strings.NewReader("l0\nl1\nl2\nl3\nl4\n")), pipeKindStdout)
	h.outputWg.Wait()

	var got []Line
	for {
		select {
		case msg := <-h.outputCh:
			got = append(got, msg)
			continue
		default:
		}
		break
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 buffered messages, got %d", len(got))
	}
	if got[0].Text != "l3" || got[1].Text != "l4" {
		t.Errorf("expected newest lines l3,l4 kept, got %q", texts(got))
	}
}

func texts(msgs []Line) []string {
	var out []string
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

func TestReadPipe_FlagsStderr(t *testing.T) {
	h := &Handle{outputCh: make(chan Line, 2), name: "child"}
	h.outputWg.Add(1)

	h.readPipe(io.NopCloser(strings.NewReader("boom\n")), pipeKindStderr)

	msg := <-h.outputCh
	if msg.Text != "boom" || !msg.IsError {
		t.Errorf("stderr line = %+v, want boom flagged as error", msg)
	}
}

func TestReadPipe_CallsOnLine(t *testing.T) {
	h := &Handle{outputCh: make(chan Line, 2), name: "child"}
	h.outputWg.Add(1)
	var seen []Line
	h.onLine = func(line Line) {
		seen = append(seen, line)
	}

	h.readPipe(io.NopCloser(strings.NewReader("a\nb\n")), pipeKindStdout)

	if len(seen) != 2 || seen[0].Text != "a" || seen[1].Text != "b" {
		t.Errorf("onLine saw %v, want a and b", texts(seen))
	}
}

func TestWrite_NotRunning(t *testing.T) {
	h := &Handle{name: "ghci"}
	err := h.Write("d1 $ sound \"bd sn\"")
	if err == nil {
		t.Fatal("expected error when writing to a non-running process")
	}
}

func TestStart_MissingExecutable(t *testing.T) {
	_, err := Start(Options{Name: "child", Exec: "/nonexistent/child"})
	if err == nil {
		t.Fatal("expected error for nonexistent executable")
	}
}
