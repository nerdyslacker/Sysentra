package services

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTurnstileReadyAddsAndRemovesValidatedServices(t *testing.T) {
	root := t.TempDir()
	manager := NewTurnstileManager(&recordingRunner{}, root)
	ready, err := manager.Ready("add", []string{"pipewire", "dbus"})
	if err != nil || !reflect.DeepEqual(ready, []string{"dbus", "pipewire"}) {
		t.Fatalf("ready=%#v err=%v", ready, err)
	}
	ready, err = manager.Ready("remove", []string{"dbus"})
	if err != nil || !reflect.DeepEqual(ready, []string{"pipewire"}) {
		t.Fatalf("ready=%#v err=%v", ready, err)
	}
	contents, _ := os.ReadFile(filepath.Join(root, "turnstile-ready", "conf"))
	if string(contents) != "core_services=\"pipewire\"\n" {
		t.Fatalf("conf=%q", contents)
	}
	if _, err := manager.Ready("add", nil); err == nil {
		t.Fatal("empty ready-service update accepted")
	}
}

func TestTurnstileEnvironmentUsesFiniteExecutableAndArguments(t *testing.T) {
	commands := &recordingRunner{}
	manager := NewTurnstileManager(commands, t.TempDir())
	if err := manager.Environment(context.Background(), []string{"DISPLAY", "XDG_SESSION_TYPE=wayland"}); err != nil {
		t.Fatal(err)
	}
	if commands.name != "/usr/bin/turnstile-update-runit-env" || !reflect.DeepEqual(commands.args, []string{"DISPLAY", "XDG_SESSION_TYPE=wayland"}) {
		t.Fatalf("command=%q args=%#v", commands.name, commands.args)
	}
	if err := manager.Environment(context.Background(), []string{"BAD-NAME=value"}); err == nil {
		t.Fatal("invalid environment name accepted")
	}
}
