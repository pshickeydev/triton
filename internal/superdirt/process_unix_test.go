//go:build !windows

package superdirt

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCloseKillsProcessGroup(t *testing.T) {
	cmd := exec.Command("sh", "-c", "sleep 300 & wait")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	p := &Process{
		cmd:      cmd,
		stdin:    stdin,
		outputCh: make(chan OutputMsg, outputChanBuffer),
		done:     make(chan struct{}),
		readyCh:  make(chan struct{}),
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting process: %v", err)
	}
	p.running = true
	go func() {
		cmd.Wait()
		close(p.done)
		p.mu.Lock()
		p.running = false
		p.mu.Unlock()
	}()

	if _, err := fmt.Fprintf(stdin, "sleep 300 & echo started >&2\n"); err != nil {
		t.Fatalf("spawning group member: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	if err := p.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if out, _ := exec.Command("pgrep", "-g", pidString(cmd.Process.Pid)).Output(); len(out) > 0 {
		t.Errorf("process group members survived Close(): %q", strings.TrimSpace(string(out)))
	}
}

func pidString(pid int) string {
	return strconv.Itoa(pid)
}
