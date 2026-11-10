//go:build windows

package processcontrol

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsCancellationIncludesBuildDescendants(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child-pid.txt")
	t.Setenv("GORGEOUS_PROCESS_CHILD_PID", pidFile)
	_, done := fixture(t, context.Background(), true)
	deadline := time.Now().Add(10 * time.Second)
	var pid int
	for {
		data, err := os.ReadFile(pidFile)
		if err == nil {
			pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
			if err == nil {
				break
			}
		}
		if time.Now().After(deadline) {
			StopAll()
			t.Fatal("build child did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		StopAll()
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	defer windows.TerminateProcess(handle, 1)
	StopAll()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("build parent did not exit")
	}
	status, err := windows.WaitForSingleObject(handle, 5000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("compiler descendant survived cancellation: status=%d error=%v", status, err)
	}
}
