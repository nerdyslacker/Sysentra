package services

import (
	"context"
	"os"
	"os/user"
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

func TestManagerInitializesUserRunsvdirService(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{}, definitions, enabled)
	manager.lookupUser = func(string) (*user.User, error) {
		return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: home}, nil
	}
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceInitUser, Name: "alice"}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	run := filepath.Join(definitions, "runsvdir-alice", "run")
	contents, err := os.ReadFile(run)
	if err != nil || string(contents) != "#!/bin/sh\nexec chpst -u 'alice' runsvdir '"+filepath.Join(home, "service")+"'\n" {
		t.Fatalf("run=%q err=%v", contents, err)
	}
	if target, err := os.Readlink(filepath.Join(enabled, "runsvdir-alice")); err != nil || target != filepath.Join(definitions, "runsvdir-alice") {
		t.Fatalf("target=%q err=%v", target, err)
	}
}

func TestManagerCreatesDisabledSystemServiceWithQuotedArguments(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	executable := filepath.Join(root, "worker")
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{}, definitions, enabled)
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceAdd, Name: "backup", Command: executable, Arguments: []string{"two words", "it's-safe"}}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(definitions, "backup", "run"))
	if err != nil || string(contents) != "#!/bin/sh\n# Created with Sysentra\nexec '"+executable+"' 'two words' 'it'\"'\"'s-safe' 2>&1\n" {
		t.Fatalf("run=%q err=%v", contents, err)
	}
	if _, err := os.Lstat(filepath.Join(enabled, "backup")); !os.IsNotExist(err) {
		t.Fatalf("new service was unexpectedly enabled: %v", err)
	}
}

func TestManagerDeletesOnlyDisabledSysentraService(t *testing.T) {
	root := t.TempDir()
	definitions, enabled := filepath.Join(root, "sv"), filepath.Join(root, "enabled")
	executable := filepath.Join(root, "worker")
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{}, definitions, enabled)
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
	if _, err := os.Stat(filepath.Join(definitions, "backup")); !os.IsNotExist(err) {
		t.Fatalf("managed service remains: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(context.Background(), security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceDelete, Name: "sshd"}); err == nil {
		t.Fatal("unmanaged service was deleted")
	}
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
