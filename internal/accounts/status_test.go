package accounts

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sysentra/internal/runner"
)

type statusRunner struct{ output string }

func (r statusRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{Stdout: []byte(r.output)}, nil
}

func TestManagerStatusReportsPasswordProcessesAndSessions(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	shadow := filepath.Join(root, "shadow")
	proc := filepath.Join(root, "proc")
	_ = os.WriteFile(passwd, []byte("alice:x:1000:1000::/home/alice:/bin/sh\nbob:x:1001:1001::/home/bob:/bin/sh\n"), 0o600)
	_ = os.WriteFile(group, []byte("alice:x:1000:\nbob:x:1001:\n"), 0o600)
	_ = os.WriteFile(shadow, []byte("alice:$6$hash:1::::::\nbob:!:1::::::\n"), 0o600)
	if err := os.MkdirAll(filepath.Join(proc, "42"), 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(proc, "42", "status"), []byte("Name:\ttest\nUid:\t1000\t1000\t1000\t1000\n"), 0o600)
	manager := NewManager(statusRunner{output: "alice tty1 2026-11-20 10:00\n"})
	manager.inventory = &Inventory{passwd: passwd, group: group}
	manager.shadow, manager.proc = shadow, proc
	statuses, err := manager.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0].Password != "Password set" || statuses[0].Processes != 1 || statuses[0].Sessions != 1 || statuses[1].Password != "Locked" {
		t.Fatalf("statuses=%#v", statuses)
	}
}
