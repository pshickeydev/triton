package proc

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type blockingStdin struct {
	entries atomic.Int32
	release chan struct{}
}

func (w *blockingStdin) Write(p []byte) (int, error) {
	w.entries.Add(1)
	<-w.release
	return len(p), nil
}

func (w *blockingStdin) Close() error {
	return nil
}

func TestWriteSerializesConcurrentSends(t *testing.T) {
	stdin := &blockingStdin{release: make(chan struct{})}
	h := &Handle{name: "child", stdin: stdin, running: true}

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			if err := h.Write("code\n"); err != nil {
				t.Errorf("Write: %v", err)
			}
		}()
	}

	deadline := time.After(100 * time.Millisecond)
	for stdin.entries.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("first write did not start")
		case <-time.After(time.Millisecond):
		}
	}
	time.Sleep(20 * time.Millisecond)
	if got := stdin.entries.Load(); got != 1 {
		t.Fatalf("concurrent writes overlapped: %d entered Write", got)
	}

	close(stdin.release)
	wg.Wait()
}

func TestSendQuitAfterProcessExitDoesNotPanic(t *testing.T) {
	h, err := Start(Options{
		Name:    "exiting",
		Exec:    "sh",
		Args:    []string{"-c", "exit 0"},
		QuitCmd: "quit\n",
		Timeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	<-h.done
	h.sendQuit()
}

func TestWriteNotRunning(t *testing.T) {
	h := &Handle{name: "ghci"}
	if err := h.Write(strings.Repeat("x", 32)); err == nil {
		t.Fatal("expected error when writing to a non-running process")
	}
}
