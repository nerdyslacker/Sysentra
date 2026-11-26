package flatpakremote

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"sysentra/internal/runner"
)

type runnerStub struct {
	result runner.Result
	name   string
	args   []string
}

func (r *runnerStub) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.name, r.args = name, args
	return r.result, nil
}

func TestManagerListsUserAndSystemRemotes(t *testing.T) {
	command := &runnerStub{result: runner.Result{Stdout: []byte(`[{"name":"flathub","title":"Flathub","url":"https://dl.flathub.org/repo/","options":"system"},{"name":"test","title":"Test","url":"https://example.org/repo/","options":"user,disabled"}]`)}}
	remotes, err := New(command).List(context.Background())
	if err != nil || len(remotes) != 2 || remotes[0].Scope() != "System" || !remotes[0].Enabled() || remotes[1].Scope() != "User" || remotes[1].Enabled() {
		t.Fatalf("remotes=%#v err=%v", remotes, err)
	}
	if reflect.DeepEqual(command.args, []string{"remotes", "--user", "--show-disabled", "--columns=name,title,url,options", "--json"}) {
		t.Fatal("list is still restricted to user remotes")
	}
}

func TestManagerChangesOnlyValidatedUserRemotes(t *testing.T) {
	command := &runnerStub{}
	manager := New(command)
	if err := manager.Change(context.Background(), "add", "flathub", "https://flathub.org/repo/flathub.flatpakrepo"); err != nil {
		t.Fatal(err)
	}
	want := []string{"remote-add", "--user", "--if-not-exists", "flathub", "https://flathub.org/repo/flathub.flatpakrepo"}
	if command.name != "flatpak" || !reflect.DeepEqual(command.args, want) {
		t.Fatalf("command=%q args=%#v", command.name, command.args)
	}
	command.name = ""
	if err := manager.Change(context.Background(), "add", "bad name", "http://example.org/repo"); err == nil || command.name != "" {
		t.Fatal(errors.New("invalid remote reached command runner"))
	}
}
