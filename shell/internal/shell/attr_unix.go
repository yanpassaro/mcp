//go:build unix

package shell

import (
	"os/exec"
	"syscall"
)

func applyAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
