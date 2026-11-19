//go:build unix

package git

import (
	"os"
	"os/exec"
	"syscall"
)

func detachGitProcess(cmd *exec.Cmd) {
	// SSH can open the controlling terminal directly to read credentials.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	cmd.Cancel = func() error {
		err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if err == syscall.ESRCH {
			return os.ErrProcessDone
		}
		return err
	}
}
