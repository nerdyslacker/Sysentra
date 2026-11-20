package firmware

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"sysentra/internal/runner"
)

type reply struct {
	result runner.Result
	err    error
}
type stubRunner struct {
	replies []reply
	calls   [][]string
}

func (s *stubRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	s.calls = append(s.calls, append([]string{name}, args...))
	reply := s.replies[len(s.calls)-1]
	return reply.result, reply.err
}

func TestListReturnsUnavailableWithoutRunningFwupd(t *testing.T) {
	command := &stubRunner{}
	inventory := NewInventory(command)
	inventory.lookPath = func(string) (string, error) { return "", errors.New("missing") }

	catalog, err := inventory.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Available || len(command.calls) != 0 {
		t.Fatalf("catalog = %#v, calls = %#v", catalog, command.calls)
	}
}

func TestListCombinesDevicesAndAvailableUpdates(t *testing.T) {
	command := &stubRunner{replies: []reply{
		{result: runner.Result{Stdout: []byte(`{"Devices":[{"Name":"System Firmware","Vendor":"Example","Version":"1.0","DeviceId":"dev-1","Summary":"Main firmware","Flags":["updatable"]}]}`)}},
		{result: runner.Result{Stdout: []byte(`{"Devices":[{"DeviceId":"dev-1","Flags":["needs-reboot","require-ac"],"Releases":[{"Version":"1.1","Flags":["is-upgrade"],"RemoteId":"lvfs","Summary":"Security update","UpdateMessage":"Keep AC power connected","Urgency":"high","Size":4096}]}]}`)}},
	}}
	inventory := NewInventory(command)
	inventory.lookPath = func(string) (string, error) { return "/usr/bin/fwupdmgr", nil }

	catalog, err := inventory.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []Device{{Name: "System Firmware", Vendor: "Example", Version: "1.0", ID: "dev-1", Summary: "Main firmware", Updatable: true, NewVersion: "1.1", RebootRequired: true, ACRequired: true, UpdateRemote: "lvfs", UpdateSummary: "Security update", UpdateMessage: "Keep AC power connected", UpdateUrgency: "high", UpdateSize: 4096}}
	if !catalog.Available || !reflect.DeepEqual(catalog.Devices, want) {
		t.Fatalf("catalog = %#v", catalog)
	}
	if !reflect.DeepEqual(command.calls, [][]string{{"fwupdmgr", "get-devices", "--json"}, {"fwupdmgr", "get-updates", "--json"}}) {
		t.Fatalf("calls = %#v", command.calls)
	}
}

func TestListRejectsMalformedDeviceJSON(t *testing.T) {
	command := &stubRunner{replies: []reply{{result: runner.Result{Stdout: []byte(`{"Devices":`)}}}}
	inventory := NewInventory(command)
	inventory.lookPath = func(string) (string, error) { return "fwupdmgr", nil }
	if _, err := inventory.List(context.Background()); err == nil {
		t.Fatal("expected malformed JSON error")
	}
}

func TestListTreatsEmptyFailedUpdateQueryAsNoUpdates(t *testing.T) {
	command := &stubRunner{replies: []reply{
		{result: runner.Result{Stdout: []byte(`{"Devices":[]}`)}},
		{err: errors.New("nothing to do")},
	}}
	inventory := NewInventory(command)
	inventory.lookPath = func(string) (string, error) { return "fwupdmgr", nil }
	catalog, err := inventory.List(context.Background())
	if err != nil || !catalog.Available {
		t.Fatalf("catalog = %#v, error = %v", catalog, err)
	}
}

func TestListReportsUnavailableDaemonWithoutFailingPage(t *testing.T) {
	command := &stubRunner{replies: []reply{{err: errors.New("daemon unavailable")}}}
	inventory := NewInventory(command)
	inventory.lookPath = func(string) (string, error) { return "fwupdmgr", nil }
	catalog, err := inventory.List(context.Background())
	if err != nil || !catalog.Available || catalog.Warning == "" {
		t.Fatalf("catalog=%#v err=%v", catalog, err)
	}
}
