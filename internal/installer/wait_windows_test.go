//go:build windows

package installer

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZipUpdateContinuesAfterWaitedProcessExits(t *testing.T) {
	child := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "Start-Sleep -Milliseconds 200")
	configureCommandForPlatform(child)
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer child.Process.Kill()
	go func() { _ = child.Wait() }()
	done := make(chan error, 1)
	go func() {
		done <- ProcessZipUpdate(filepath.Join(t.TempDir(), "missing.zip"), "unused.uproject", child.Process.Pid)
	}()
	select {
	case err := <-done:
		if isProcessRunning(child.Process.Pid) {
			t.Fatal("ZIP update continued before the target process exited")
		}
		if err == nil || !strings.Contains(err.Error(), "failed to extract zip") {
			t.Fatalf("did not continue to package validation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ZIP update kept waiting after the target process exited")
	}
}
