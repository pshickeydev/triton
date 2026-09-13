//go:build windows

package superdirt

import (
	"os/exec"
	"strconv"
)

func setProcGroup(cmd *exec.Cmd) {}

func killProcGroup(cmd *exec.Cmd) error {
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		return cmd.Process.Kill()
	}
	return nil
}
