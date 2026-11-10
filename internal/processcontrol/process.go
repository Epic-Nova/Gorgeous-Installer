// Package processcontrol confines cancellation to build processes started by this installer.
package processcontrol

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"
)

type processTree interface {
	attach(*os.Process) error
	stop() error
	close()
}

var owned = struct {
	sync.Mutex
	commands map[*exec.Cmd]processTree
}{commands: make(map[*exec.Cmd]processTree)}

func Start(cmd *exec.Cmd) error {
	tree, err := newProcessTree(cmd)
	if err != nil {
		return err
	}
	if cmd.Cancel != nil {
		cmd.Cancel = tree.stop
	}
	cmd.WaitDelay = 5 * time.Second
	owned.Lock()
	owned.commands[cmd] = tree
	owned.Unlock()
	forget := func() { owned.Lock(); delete(owned.commands, cmd); owned.Unlock(); tree.close() }
	if err := cmd.Start(); err != nil {
		forget()
		return err
	}
	if err := tree.attach(cmd.Process); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		forget()
		return fmt.Errorf("track build process: %w", err)
	}
	return nil
}

func Wait(cmd *exec.Cmd) error {
	err := cmd.Wait()
	owned.Lock()
	tree := owned.commands[cmd]
	delete(owned.commands, cmd)
	owned.Unlock()
	if tree != nil {
		tree.close()
	}
	return err
}

// StopAll never searches by executable name or touches builds owned by other applications.
func StopAll() {
	owned.Lock()
	trees := make([]processTree, 0, len(owned.commands))
	for _, tree := range owned.commands {
		trees = append(trees, tree)
	}
	owned.Unlock()
	for _, tree := range trees {
		_ = tree.stop()
	}
}
