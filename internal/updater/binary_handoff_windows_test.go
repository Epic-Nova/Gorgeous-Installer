//go:build windows

package updater

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsBinaryUpdateWaitsAndHandlesLiteralPaths(t *testing.T) {
	if os.Getenv("GORGEOUS_NATIVE_UPDATE_TEST") != "1" {
		t.Skip("set GORGEOUS_NATIVE_UPDATE_TEST=1 for the native executable replacement test")
	}
	root := filepath.Join(t.TempDir(), "update with spaces %PATH% ! () ' ü")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "restarted.txt")
	t.Setenv("GORGEOUS_UPDATE_MARKER", marker)
	mainFile := filepath.Join(root, "fixture.go")
	if err := os.WriteFile(mainFile, []byte(`package main
import ("os"; "time"; "bytes")
func main() { if len(os.Args)>1 && os.Args[1]=="--hold" { time.Sleep(2*time.Second); return }; exe,_:=os.Executable();data,_:=os.ReadFile(exe);if bytes.HasSuffix(data,[]byte("BROKEN_UPDATE")){os.Exit(42)}; _=os.WriteFile(os.Getenv("GORGEOUS_UPDATE_MARKER"), []byte("restarted"), 0600) }
`), 0600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(root, "installer.exe")
	if out, err := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", destination, mainFile).CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v\n%s", err, out)
	}
	oldBytes, err := os.ReadFile(destination)
	if err != nil {
		t.Fatal(err)
	}
	newBytes := append(append([]byte(nil), oldBytes...), []byte("new build overlay")...)
	stage, err := os.MkdirTemp("", "gorgeous-installer-extracted-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(stage)
	source := filepath.Join(stage, "gorgeous-installer.exe")
	if err := os.WriteFile(source, newBytes, 0600); err != nil {
		t.Fatal(err)
	}
	owner := exec.Command(destination, "--hold")
	configureUpdateHelper(owner)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	defer owner.Process.Kill()
	// Verify this really is the Windows running-executable lock, rather than a simulated copy.
	if err := os.WriteFile(destination, []byte("must not replace while running"), 0600); err == nil {
		t.Fatal("fixture executable was not locked")
	}
	errorPath := filepath.Join(root, "update error.txt")
	cmd, err := windowsBinaryUpdateCommand(binaryUpdateHandoff{ParentPID: owner.Process.Pid, Source: source, Destination: destination, Fallback: destination, ErrorPath: errorPath, StageDirectory: stage})
	if err != nil {
		t.Fatal(err)
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		log, _ := os.ReadFile(errorPath)
		t.Fatalf("native handoff: %v\n%s\n%s", err, out, log)
	}
	_ = owner.Wait()
	data, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(data, newBytes) {
		t.Fatalf("new executable was not installed: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if data, err := os.ReadFile(marker); err == nil && string(data) == "restarted" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("updated executable did not relaunch")
		}
		time.Sleep(25 * time.Millisecond)
	}
	if _, err := os.Stat(stage); !os.IsNotExist(err) {
		t.Fatalf("private staging directory was not cleaned: %v", err)
	}
	if _, err := os.Stat(errorPath); !os.IsNotExist(err) {
		t.Fatalf("unexpected error log: %v", err)
	}
	t.Run("failed new launch rolls back", func(t *testing.T) {
		_ = os.Remove(marker)
		stage, err := os.MkdirTemp("", "gorgeous-installer-extracted-*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(stage)
		source := filepath.Join(stage, "gorgeous-installer.exe")
		if err := os.WriteFile(source, append(append([]byte(nil), newBytes...), []byte("BROKEN_UPDATE")...), 0600); err != nil {
			t.Fatal(err)
		}
		cmd, err := windowsBinaryUpdateCommand(binaryUpdateHandoff{Source: source, Destination: destination, Fallback: destination, ErrorPath: errorPath, StageDirectory: stage})
		if err != nil {
			t.Fatal(err)
		}
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("broken update reported success: %s", out)
		}
		data, err := os.ReadFile(destination)
		if err != nil || !bytes.Equal(data, newBytes) {
			t.Fatalf("previous executable was not restored: %v", err)
		}
		if log, err := os.ReadFile(errorPath); err != nil || len(log) == 0 {
			t.Fatalf("failed launch was not recorded: %q %v", log, err)
		}
		deadline := time.Now().Add(10 * time.Second)
		for {
			if data, err := os.ReadFile(marker); err == nil && string(data) == "restarted" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("previous executable did not relaunch")
			}
			time.Sleep(25 * time.Millisecond)
		}
	})
}
