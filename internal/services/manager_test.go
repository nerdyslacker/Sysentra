package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type recordingRunner struct {
	name string
	args []string
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.name, r.args = name, args
	return runner.Result{}, nil
}

func TestManagerEnablesAndDisablesValidatedService(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(definitions, "sshd"), filepath.Join(enabled, "sshd")); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{}, definitions, enabled)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceEnable, Name: "sshd"}); err != nil {
		t.Fatal(err)
	}
	if target, err := os.Readlink(filepath.Join(enabled, "sshd")); err != nil || target != filepath.Join(definitions, "sshd") {
		t.Fatalf("target=%q err=%v", target, err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceDisable, Name: "sshd"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(enabled, "sshd")); !os.IsNotExist(err) {
		t.Fatalf("enabled link remains: %v", err)
	}
}

func TestManagerRunsSVWithoutShell(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(definitions, "sshd"), filepath.Join(enabled, "sshd")); err != nil {
		t.Fatal(err)
	}
	commands := &recordingRunner{}
	manager := NewManager(commands, definitions, enabled)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceRestart, Name: "sshd"}); err != nil {
		t.Fatal(err)
	}
	if commands.name != "/usr/bin/sv" || !reflect.DeepEqual(commands.args, []string{"-w", "7", "restart", filepath.Join(enabled, "sshd")}) {
		t.Fatalf("command=%q args=%#v", commands.name, commands.args)
	}
}

func TestManagerSupportsLazysvctlLifecycleOperations(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(definitions, "sshd"), filepath.Join(enabled, "sshd")); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []security.ServiceOperation{security.ServiceReload, security.ServiceOnce, security.ServiceKill, security.ServicePause, security.ServiceContinue} {
		commands := &recordingRunner{}
		manager := NewManager(commands, definitions, enabled)
		if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: operation, Name: "sshd"}); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
		if commands.name != "/usr/bin/sv" || !reflect.DeepEqual(commands.args, []string{"-w", "7", string(operation), filepath.Join(enabled, "sshd")}) {
			t.Fatalf("%s command=%q args=%#v", operation, commands.name, commands.args)
		}
	}
}

func TestManagerRefusesNonSymlinkDisableTarget(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(enabled, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{}, definitions, enabled)
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: 1, Operation: security.ServiceDisable, Name: "sshd"}); err == nil {
		t.Fatal("non-symlink target was removed")
	}
}
