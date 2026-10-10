//go:build !windows

package superdirt

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCloseKillsProcessGroup(t *testing.T) {
	script := filepath.Join(t.TempDir(), "group.sh")
	if err := os.WriteFile(script, []byte("sleep 300 & wait\n"), 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	p, err := New("sh", script)
	if err != nil {
		t.Fatalf("starting process: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := p.Close(); err != nil {
		t.Fatalf("Close() = %v", err)
	}

	if out, _ := exec.Command("pgrep", "-g", pidString(p.handle.Pid())).Output(); len(out) > 0 {
		t.Errorf("process group members survived Close(): %q", strings.TrimSpace(string(out)))
	}
}

func pidString(pid int) string {
	return strconv.Itoa(pid)
}
