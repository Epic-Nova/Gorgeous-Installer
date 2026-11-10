//go:build !windows

package processcontrol

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
)

type unixTree struct {
	sync.Mutex
	pid     int
	stopped bool
}

func newProcessTree(cmd *exec.Cmd) (processTree, error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return &unixTree{}, nil
}
func (t *unixTree) attach(p *os.Process) error {
	t.Lock()
	defer t.Unlock()
	t.pid = p.Pid
	if t.stopped {
		return syscall.Kill(-t.pid, syscall.SIGKILL)
	}
	return nil
}
func (t *unixTree) stop() error {
	t.Lock()
	defer t.Unlock()
	t.stopped = true
	if t.pid == 0 {
		return nil
	}
	return syscall.Kill(-t.pid, syscall.SIGKILL)
}
func (t *unixTree) close() { _ = t.stop(); t.Lock(); t.pid = 0; t.Unlock() }
