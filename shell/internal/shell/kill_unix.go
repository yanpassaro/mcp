//go:build unix

package shell

import (
	"os"
	"syscall"
)

func killTree(p *os.Process) error {
	return syscall.Kill(-p.Pid, syscall.SIGKILL)
}
