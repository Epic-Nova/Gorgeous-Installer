package safepath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInside(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"", ".", "..", "../outside", "Source/../../outside", `..\outside`, `/absolute`, `C:\outside`, `Source/file:stream`, `.. /outside`, `Source/.././.. /outside`, `Source/trailing.`, `Source/trailing `, `Source/NUL.txt`, `Source/CoM1.cpp`} {
		if p, e := Inside(root, name); e == nil {
			t.Errorf("accepted unsafe %q as %q", name, p)
		}
	}
	if p, e := Inside(root, "Source/Runtime/File.cpp"); e != nil || p != filepath.Join(root, "Source", "Runtime", "File.cpp") {
		t.Fatalf("valid path failed: %s %v", p, e)
	}
	if os.Symlink(t.TempDir(), filepath.Join(root, "link")) == nil {
		if _, e := Inside(root, "link/file"); e == nil {
			t.Fatal("accepted symlink escape")
		}
	}
}
func TestPluginName(t *testing.T) {
	for _, n := range []string{"", "../Other", "C:/Engine", "Gorgeous/Core", "Core:stream"} {
		if PluginName(n) {
			t.Errorf("accepted %q", n)
		}
	}
	if !PluginName("GorgeousCore") {
		t.Fatal("valid plugin rejected")
	}
}
