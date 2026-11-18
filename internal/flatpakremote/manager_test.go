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

func TestManagerListsDisabledUserRemotes(t *testing.T) {
	command := &runnerStub{result: runner.Result{Stdout: []byte(`[{"name":"flathub","title":"Flathub","url":"https://flathub.org/repo/flathub.flatpakrepo","options":"disabled"}]`)}}
	remotes, err := New(command).List(context.Background())
	if err != nil || len(remotes) != 1 || remotes[0].Enabled() {
		t.Fatalf("remotes=%#v err=%v", remotes, err)
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
