package git

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func detachGitProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS}
}
