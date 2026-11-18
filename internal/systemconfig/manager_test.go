package systemconfig

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type failingRunner struct{}

func (failingRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{}, errors.New("sync failed")
}

func TestManagerAddsRepositorySambaAndNFS(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "xbps.d")
	samba := filepath.Join(root, "samba", "smb.conf")
	fstab := filepath.Join(root, "fstab")
	manager := NewManager(runner.Exec{}, config, filepath.Join(root, "vendor"), samba, fstab)
	requests := []security.SystemConfigRequest{
		{Version: security.APIVersion, Operation: security.RepositoryAdd, Value: "https://repo.example.org/current", TrustConfirmed: true},
		{Version: security.APIVersion, Operation: security.SambaAdd, Value: "Media", Path: "/srv/media", GuestOK: true},
		{Version: security.APIVersion, Operation: security.NFSAdd, Value: "server:/data", Path: filepath.Join(root, "mnt"), Type: "nfs4", Options: "rw,_netdev"},
	}
	for _, request := range requests {
		if err := manager.Change(request); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	checks := map[string]string{
		filepath.Join(config, managedRepositories): "repository=https://repo.example.org/current",
		samba: "[Media]",
		fstab: "server:/data",
	}
	for path, want := range checks {
		contents, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(contents), want) {
			t.Fatalf("%s = %q, %v", path, contents, err)
		}
	}
}

func TestMirrorChangeRollsBackWhenIndexRefreshFails(t *testing.T) {
	root := t.TempDir()
	vendor := filepath.Join(root, "vendor")
	config := filepath.Join(root, "xbps.d")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendor, "00-repository-main.conf"), []byte("repository=https://repo-default.voidlinux.org/current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(failingRunner{}, config, vendor, filepath.Join(root, "smb.conf"), filepath.Join(root, "fstab"))
	err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.MirrorUse, Value: "https://repo.example.org/"})
	if err == nil {
		t.Fatal("expected refresh failure")
	}
	if _, statErr := os.Stat(filepath.Join(config, "00-repository-main.conf")); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("new mirror override was not rolled back: %v", statErr)
	}
}
