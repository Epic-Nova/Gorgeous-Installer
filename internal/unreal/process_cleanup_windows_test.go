//go:build windows

package unreal

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKillUBTDoesNotTargetUnrelatedProcessNames(t *testing.T) {
	if os.Getenv("GORGEOUS_NATIVE_PROCESS_TEST") != "1" {
		t.Skip("set GORGEOUS_NATIVE_PROCESS_TEST=1 to exercise the native command interception fixture")
	}
	root := t.TempDir()
	source := filepath.Join(root, "fake_taskkill.go")
	if err := os.WriteFile(source, []byte(`package main
import ("os";"strings")
func main(){ f,e:=os.OpenFile(os.Getenv("GORGEOUS_TASKKILL_LOG"),os.O_CREATE|os.O_APPEND|os.O_WRONLY,0600);if e==nil{defer f.Close();f.WriteString(strings.Join(os.Args[1:]," ")+"\n")} }
`), 0600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("go", "build", "-ldflags=-H=windowsgui", "-o", filepath.Join(root, "taskkill.exe"), source).CombinedOutput(); err != nil {
		t.Fatalf("build interception fixture: %v %s", err, out)
	}
	log := filepath.Join(root, "calls.txt")
	t.Setenv("GORGEOUS_TASKKILL_LOG", log)
	t.Setenv("PATH", root+string(os.PathListSeparator)+os.Getenv("PATH"))
	// A fake taskkill makes the regression safe: no real user processes are terminated.
	KillUBT()
	data, err := os.ReadFile(log)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(data)), "/im") {
		t.Fatalf("cleanup targets all processes by name: %s", data)
	}
	if len(data) > 0 {
		t.Fatalf("no installer-owned processes exist, but cleanup ran: %s", data)
	}
}
