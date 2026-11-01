package unreal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleVersionWithoutInstalledEngine(t *testing.T) {
	file := filepath.Join(t.TempDir(), "Game.uproject")
	if err := os.WriteFile(file, []byte(`{"EngineAssociation":"5.7"}`), 0644); err != nil {
		t.Fatal(err)
	}
	version, _, err := GetExampleEngineVersion(file)
	if err != nil || version != "5.7" {
		t.Fatalf("got %q, %v", version, err)
	}
}
