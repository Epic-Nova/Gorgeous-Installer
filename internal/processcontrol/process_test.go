package processcontrol

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"
	"time"
)

func TestProcessFixture(t *testing.T) {
	if os.Getenv("GORGEOUS_PROCESS_FIXTURE") != "1" {
		return
	}
	if path := os.Getenv("GORGEOUS_PROCESS_CHILD_PID"); path != "" {
		if os.Getenv("GORGEOUS_PROCESS_IS_CHILD") == "1" {
			_ = os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0600)
		} else {
			child := exec.Command(os.Args[0], "-test.run=^TestProcessFixture$")
			child.Env = append(os.Environ(), "GORGEOUS_PROCESS_IS_CHILD=1")
			if err := child.Start(); err != nil {
				os.Exit(2)
			}
		}
	}
	_, _ = os.Stdout.WriteString("ready\n")
	time.Sleep(60 * time.Second)
	os.Exit(0)
}

func fixture(t *testing.T, ctx context.Context, tracked bool) (*exec.Cmd, <-chan error) {
	t.Helper()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestProcessFixture$")
	cmd.Env = append(os.Environ(), "GORGEOUS_PROCESS_FIXTURE=1")
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if tracked {
		err = Start(cmd)
	} else {
		err = cmd.Start()
	}
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan bool, 1)
	go func() { scanner := bufio.NewScanner(out); ready <- scanner.Scan() && scanner.Text() == "ready" }()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("fixture failed to start")
		}
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("fixture startup timeout")
	}
	done := make(chan error, 1)
	go func() {
		if tracked {
			done <- Wait(cmd)
		} else {
			done <- cmd.Wait()
		}
	}()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return cmd, done
}

func TestStopAllOnlyStopsOwnedProcesses(t *testing.T) {
	_, unrelated := fixture(t, context.Background(), false)
	_, build := fixture(t, context.Background(), true)
	StopAll()
	select {
	case err := <-build:
		if err == nil {
			t.Fatal("build was not cancelled")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("owned build did not exit")
	}
	select {
	case err := <-unrelated:
		t.Fatalf("unrelated process was terminated: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestContextCancellationStopsOwnedBuild(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, build := fixture(t, ctx, true)
	cancel()
	select {
	case err := <-build:
		if err == nil {
			t.Fatal("cancelled build reported success")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("cancelled build did not exit")
	}
}
