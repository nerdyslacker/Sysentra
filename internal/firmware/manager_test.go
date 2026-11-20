package firmware

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type managerRunner struct {
	result runner.Result
	err    error
	calls  [][]string
}

func (r *managerRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	return r.result, r.err
}

func TestManagerRefreshesWithoutDisablingFwupdSafety(t *testing.T) {
	commands := &managerRunner{result: runner.Result{Stdout: []byte("Metadata refreshed\n")}}
	result, err := NewManager(commands).Change(context.Background(), security.FirmwareRequest{Version: security.APIVersion, Operation: security.FirmwareRefresh})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"/usr/bin/fwupdmgr", "refresh", "--assume-yes"}}
	if !reflect.DeepEqual(commands.calls, want) || len(result.Diagnostics) != 1 {
		t.Fatalf("calls=%#v result=%#v", commands.calls, result)
	}
}

func TestManagerUpdatesOnlyReviewedDevice(t *testing.T) {
	commands := &managerRunner{result: runner.Result{Stdout: []byte("Successfully installed firmware\n")}}
	request := security.FirmwareRequest{Version: security.APIVersion, Operation: security.FirmwareUpdate, DeviceID: "device-1", VersionName: "1.2.3"}
	if _, err := NewManager(commands).Change(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"/usr/bin/fwupdmgr", "install", "device-1", "1.2.3", "--assume-yes"}}
	if !reflect.DeepEqual(commands.calls, want) {
		t.Fatalf("calls=%#v", commands.calls)
	}
}

func TestManagerReturnsNativeFailure(t *testing.T) {
	commands := &managerRunner{result: runner.Result{Stderr: []byte("Blocked by low battery\n")}, err: errors.New("exit status 1")}
	_, err := NewManager(commands).Change(context.Background(), security.FirmwareRequest{Version: security.APIVersion, Operation: security.FirmwareUpdate, DeviceID: "device-1", VersionName: "1.2.3"})
	if err == nil || !strings.Contains(err.Error(), "low battery") {
		t.Fatalf("error=%v", err)
	}
}
