package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sysentra/internal/security"
)

func TestUserInventoryListsEnabledAndDisabledServices(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "syncthing"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".disabled", "backup"), 0o755); err != nil {
		t.Fatal(err)
	}
	items, err := NewUserInventory(fakeRunner{output: "run: " + filepath.Join(root, "syncthing") + ": (pid 42) 15s\n"}, root, filepath.Join(root, "proc")).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Name != "backup" || items[0].Enabled || items[0].State != "Stopped" || items[1].Name != "syncthing" || !items[1].Enabled || items[1].State != "Running" {
		t.Fatalf("services = %#v", items)
	}
}

func TestUserManagerMovesServicesThroughDisabledDirectory(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "syncthing"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewUserManager(&recordingRunner{}, root)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceDisable, Name: "syncthing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".disabled", "syncthing")); err != nil {
		t.Fatalf("disabled service: %v", err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceEnable, Name: "syncthing"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "syncthing")); err != nil {
		t.Fatalf("enabled service: %v", err)
	}
}

func TestUserManagerRunsLifecycleCommandWithoutPrivilege(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "syncthing"), 0o755); err != nil {
		t.Fatal(err)
	}
	commands := &recordingRunner{}
	manager := NewUserManager(commands, root)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServicePause, Name: "syncthing"}); err != nil {
		t.Fatal(err)
	}
	if commands.name != "/usr/bin/sv" || !reflect.DeepEqual(commands.args, []string{"-w", "7", "pause", filepath.Join(root, "syncthing")}) {
		t.Fatalf("command=%q args=%#v", commands.name, commands.args)
	}
}

func TestUserManagerRejectsSymlinkService(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	manager := NewUserManager(&recordingRunner{}, root)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceDisable, Name: "escape"}); err == nil {
		t.Fatal("symlink service was accepted")
	}
}
