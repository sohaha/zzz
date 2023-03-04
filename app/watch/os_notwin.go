//go:build !windows
// +build !windows

package watch

import (
	"os/exec"
	"syscall"
)

func sCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid: true,
		}
	} else {
		cmd.SysProcAttr.Setpgid = true
	}
}
