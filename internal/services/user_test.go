package services

import (
	"context"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
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

func TestUserManagerCreatesDisabledService(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "worker")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewUserManager(&recordingRunner{}, root)
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	group, err := user.LookupGroupId(current.Gid)
	if err != nil {
		t.Fatal(err)
	}
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceAdd, Name: "backup", Command: executable, Arguments: []string{"--once"}, User: current.Username, Group: group.Name}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, ".disabled", "backup", "run"))
	if err != nil {
		t.Fatalf("disabled service was not created: %v", err)
	}
	want := "#!/bin/sh\n# Created with Sysentra\nexec chpst -u '" + current.Username + ":" + group.Name + "' '" + executable + "' '--once' 2>&1\n"
	if string(contents) != want {
		t.Fatalf("run=%q want=%q", contents, want)
	}
}

func TestUserManagerOmitsUnknownGroupFromChpstIdentity(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "worker")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	manager := NewUserManager(&recordingRunner{}, root)
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceAdd, Name: "backup", Command: executable, User: current.Username, Group: "sysentramissinggroup"}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, ".disabled", "backup", "run"))
	if err != nil || !strings.Contains(string(contents), "chpst -u '"+current.Username+"' ") || strings.Contains(string(contents), current.Username+":") {
		t.Fatalf("run=%q err=%v", contents, err)
	}
}

func TestUserManagerDeletesOnlyDisabledSysentraService(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "worker")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewUserManager(&recordingRunner{}, root)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceAdd, Name: "backup", Command: executable}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceEnable, Name: "backup"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceDelete, Name: "backup"}); err == nil {
		t.Fatal("enabled managed service was deleted")
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceDisable, Name: "backup"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceDelete, Name: "backup"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".disabled", "backup")); !os.IsNotExist(err) {
		t.Fatalf("managed service remains: %v", err)
	}
}
