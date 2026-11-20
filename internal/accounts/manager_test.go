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

func TestManagerEditsUserPrimaryGroupAndPasswordLock(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	_ = os.WriteFile(passwd, []byte("alice:x:1000:1000::/home/alice:/bin/sh\n"), 0o600)
	_ = os.WriteFile(group, []byte("alice:x:1000:\nusers:x:1001:\n"), 0o600)
	tests := []struct {
		request security.AccountRequest
		args    []string
	}{
		{security.AccountRequest{Version: security.APIVersion, Operation: security.UserEdit, User: "alice", Home: "/srv/alice", Shell: "/bin/zsh", MoveHome: true}, []string{"--home", "/srv/alice", "--move-home", "--shell", "/bin/zsh", "alice"}},
		{security.AccountRequest{Version: security.APIVersion, Operation: security.PrimaryGroupChange, User: "alice", Group: "users"}, []string{"--gid", "users", "alice"}},
		{security.AccountRequest{Version: security.APIVersion, Operation: security.PasswordLock, User: "alice"}, []string{"--lock", "alice"}},
		{security.AccountRequest{Version: security.APIVersion, Operation: security.PasswordUnlock, User: "alice"}, []string{"--unlock", "alice"}},
	}
	for _, test := range tests {
		command := &recordingRunner{}
		manager := NewManager(command)
		manager.inventory = &Inventory{passwd: passwd, group: group}
		if err := manager.Change(context.Background(), test.request); err != nil {
			t.Fatalf("%s: %v", test.request.Operation, err)
		}
		if command.name != "/usr/bin/usermod" || !reflect.DeepEqual(command.args, test.args) {
			t.Fatalf("%s command=%q args=%#v", test.request.Operation, command.name, command.args)
		}
	}
}

func TestManagerProtectsSystemUsersFromAccountMutation(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	_ = os.WriteFile(passwd, []byte("root:x:0:0::/root:/bin/sh\n"), 0o600)
	_ = os.WriteFile(group, []byte("root:x:0:\nusers:x:1001:\n"), 0o600)
	for _, request := range []security.AccountRequest{
		{Version: security.APIVersion, Operation: security.UserEdit, User: "root", Shell: "/bin/zsh"},
		{Version: security.APIVersion, Operation: security.PrimaryGroupChange, User: "root", Group: "users"},
		{Version: security.APIVersion, Operation: security.PasswordLock, User: "root"},
	} {
		command := &recordingRunner{}
		manager := NewManager(command)
		manager.inventory = &Inventory{passwd: passwd, group: group}
		if err := manager.Change(context.Background(), request); err == nil || command.name != "" {
			t.Fatalf("%s reached command: %q", request.Operation, command.name)
		}
	}
}
