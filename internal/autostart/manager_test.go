package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManagerEnablesBackgroundAutostartAndDisablesIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "autostart", "sysentra.desktop")
	manager := &Manager{path: path, executable: "/opt/Sysentra App/sysentra"}
	if err := manager.Set(true); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `Exec="/opt/Sysentra App/sysentra" --background`) || !strings.Contains(string(contents), "X-Sysentra-Managed=true") {
		t.Fatalf("desktop entry = %q", contents)
	}
	enabled, err := manager.Enabled()
	if err != nil || !enabled {
		t.Fatalf("enabled = %v, err = %v", enabled, err)
	}
	if err := manager.Set(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("autostart entry still exists: %v", err)
	}
}

func TestManagerRefusesToReplaceUnownedEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sysentra.desktop")
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nExec=other\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := &Manager{path: path, executable: "/usr/bin/sysentra"}
	if err := manager.Set(true); err == nil {
		t.Fatal("expected unowned entry rejection")
	}
}
