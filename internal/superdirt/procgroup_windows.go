//go:build windows

package superdirt

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
)

func setProcGroup(cmd *exec.Cmd) {}

func killProcGroup(cmd *exec.Cmd) error {
	if err := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run(); err != nil {
		// The process may already have exited and been waited on.
		if killErr := cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return killErr
		}
	}
	return nil
}
