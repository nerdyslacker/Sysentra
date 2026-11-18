package accounts

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

func TestManagerUsesAllowlistedMembershipCommand(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	if err := os.WriteFile(passwd, []byte("alice:x:1000:1000::/home/alice:/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(group, []byte("alice:x:1000:\nwheel:x:10:\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := &recordingRunner{}
	manager := NewManager(command)
	manager.inventory = &Inventory{passwd: passwd, group: group}
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.MembershipAdd, User: "alice", Groups: []string{"wheel"}}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if command.name != "/usr/bin/usermod" || !reflect.DeepEqual(command.args, []string{"--append", "--groups", "wheel", "alice"}) {
		t.Fatalf("command=%q args=%#v", command.name, command.args)
	}
}

func TestManagerProtectsSystemUser(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	_ = os.WriteFile(passwd, []byte("daemon:x:1:1::/:/sbin/nologin\n"), 0o600)
	_ = os.WriteFile(group, []byte("daemon:x:1:\n"), 0o600)
	command := &recordingRunner{}
	manager := NewManager(command)
	manager.inventory = &Inventory{passwd: passwd, group: group}
	err := manager.Change(context.Background(), security.AccountRequest{Version: security.APIVersion, Operation: security.UserRemove, User: "daemon"})
	if err == nil || command.name != "" {
		t.Fatalf("system user reached command: %q %v", command.name, err)
	}
}

func TestManagerCreatesUserWithCustomHome(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	_ = os.WriteFile(passwd, nil, 0o600)
	_ = os.WriteFile(group, nil, 0o600)
	command := &recordingRunner{}
	manager := NewManager(command)
	manager.inventory = &Inventory{passwd: passwd, group: group}
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.UserAdd, User: "alice", CreateHome: true, Home: "/srv/homes/alice"}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	want := []string{"--user-group", "--create-home", "--home-dir", "/srv/homes/alice", "alice"}
	if command.name != "/usr/bin/useradd" || !reflect.DeepEqual(command.args, want) {
		t.Fatalf("command=%q args=%#v", command.name, command.args)
	}
}

func TestManagerCreatesUserWithoutHome(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	_ = os.WriteFile(passwd, nil, 0o600)
	_ = os.WriteFile(group, nil, 0o600)
	command := &recordingRunner{}
	manager := NewManager(command)
	manager.inventory = &Inventory{passwd: passwd, group: group}
	request := security.AccountRequest{Version: security.APIVersion, Operation: security.UserAdd, User: "alice"}
	if err := manager.Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	want := []string{"--user-group", "--no-create-home", "alice"}
	if command.name != "/usr/bin/useradd" || !reflect.DeepEqual(command.args, want) {
		t.Fatalf("command=%q args=%#v", command.name, command.args)
	}
}
