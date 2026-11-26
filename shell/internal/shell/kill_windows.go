//go:build windows

package shell

import (
	"os"
	"os/exec"
	"strconv"
)

func killTree(p *os.Process) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(p.Pid))
	return cmd.Run()
}
