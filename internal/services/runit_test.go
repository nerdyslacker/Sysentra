package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"sysentra/internal/runner"
)

type fakeRunner struct{ output string }

func (f fakeRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{Stdout: []byte(f.output)}, nil
}

type stderrRunner struct{ output string }

func (f stderrRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{Stderr: []byte(f.output)}, errors.New("status returned non-zero")
}

func TestListRunitServicesSeparatesAvailableAndEnabledState(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "dbus"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(definitions, "dbus"), filepath.Join(enabled, "dbus")); err != nil {
		t.Fatal(err)
	}

	got, err := ListRunitServices(definitions, enabled)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "dbus" || !got[0].Enabled || got[1].Enabled {
		t.Fatalf("services = %#v", got)
	}
}

func TestInventoryAddsRuntimeStatePIDCommandAndTime(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	proc := filepath.Join(root, "proc")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(definitions, "sshd"), filepath.Join(enabled, "sshd")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(proc, "42"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proc, "42", "cmdline"), []byte("sshd\x00-D\x00"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := NewInventory(fakeRunner{output: "run: " + filepath.Join(enabled, "sshd") + ": (pid 42) 15s\n"}, definitions, enabled, proc).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != "Running" || items[0].PID != 42 || items[0].Command != "sshd" || items[0].Time != "15s" {
		t.Fatalf("service = %#v", items)
	}
}

func TestInventoryReportsDisabledServiceAsStopped(t *testing.T) {
	root := t.TempDir()
	definitions := filepath.Join(root, "sv")
	enabled := filepath.Join(root, "enabled")
	if err := os.MkdirAll(filepath.Join(definitions, "sshd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(enabled, 0o755); err != nil {
		t.Fatal(err)
	}
	items, err := NewInventory(fakeRunner{}, definitions, enabled, filepath.Join(root, "proc")).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != "Stopped" {
		t.Fatalf("services = %#v", items)
	}
}

func TestInventoryParsesStatusReturnedOnStandardError(t *testing.T) {
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
	items, err := NewInventory(stderrRunner{output: "run: " + filepath.Join(enabled, "sshd") + ": (pid 42) 15s\n"}, definitions, enabled, filepath.Join(root, "proc")).List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].State != "Running" || items[0].PID != 42 {
		t.Fatalf("services = %#v", items)
	}
}

func TestParseStoppedServiceTime(t *testing.T) {
	status := parseStatus("down: /etc/sv/cupsd: 27s, normally up\n")["cupsd"]
	if status.State != "Stopped" || status.Time != "27s" {
		t.Fatalf("status = %#v", status)
	}
}
