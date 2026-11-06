//go:build !windows

package superdirt

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcGroup(cmd *exec.Cmd) error {
	if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	// The group leader may already have exited and been waited on; the
	// group kill above is what reaps its descendants.
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}
