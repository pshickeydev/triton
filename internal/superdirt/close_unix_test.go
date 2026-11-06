//go:build !windows

package superdirt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestCloseKillsDescendantsAfterParentExit(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "child.pid")
	scriptPath := filepath.Join(dir, "spawn.sh")
	script := fmt.Sprintf("sleep 30 </dev/null >/dev/null 2>&1 &\necho $! > %s\nexit 0\n", strconv.Quote(pidPath))
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	process, err := New("sh", scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-process.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("parent did not exit")
	}
	pidText, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidText)))
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Kill(pid, syscall.SIGKILL)

	if err := process.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		t.Fatal("Close returned after parent exit but descendant survived")
	}
}
