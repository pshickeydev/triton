package proc

import (
	"strings"
	"testing"
	"time"
)

// stdinFillSize exceeds any pipe buffer so a child that never reads
// stdin leaves its writer blocked inside the kernel.
const stdinFillSize = 8 * 1024 * 1024

func TestCloseUnblocksBehindBlockedWrite(t *testing.T) {
	h, err := Start(Options{
		Name:    "unresponsive",
		Exec:    "sleep",
		Args:    []string{"30"},
		QuitCmd: "quit\n",
		Timeout: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}

	writeResult := make(chan error, 1)
	go func() {
		writeResult <- h.Write(strings.Repeat("x", stdinFillSize))
	}()
	time.Sleep(50 * time.Millisecond)
	closeResult := make(chan error, 1)
	go func() {
		closeResult <- h.Close()
	}()

	select {
	case err := <-closeResult:
		if err != nil {
			t.Fatalf("Close returned an error: %v", err)
		}
	case <-time.After(250 * time.Millisecond):
		_ = h.cmd.Process.Kill()
		select {
		case <-h.done:
		case <-time.After(time.Second):
			t.Fatal("forced process kill did not settle the handle")
		}
		t.Fatal("Close remained blocked behind a child stdin write")
	}
}
