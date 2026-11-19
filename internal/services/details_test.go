package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestServiceDetailsReturnsRunScriptAndBoundedLogs(t *testing.T) {
	root := t.TempDir()
	definition := filepath.Join(root, "backup")
	if err := os.MkdirAll(filepath.Join(definition, "log", "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(definition, "run"), []byte("#!/bin/sh\nexec backup\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lines := make([]byte, 0, 4096)
	for index := 0; index < 220; index++ {
		lines = append(lines, []byte("backup complete\n")...)
	}
	if err := os.WriteFile(filepath.Join(definition, "log", "main", "current"), lines, 0o600); err != nil {
		t.Fatal(err)
	}
	details, err := serviceDetails(root, "backup", false)
	if err != nil || details.RunScript != "#!/bin/sh\nexec backup" || len(details.Logs) != 200 || details.LogPath == "" {
		t.Fatalf("details=%#v err=%v", details, err)
	}
	if _, err := serviceDetails(root, "../backup", false); err == nil {
		t.Fatal("escaping service name accepted")
	}
}
