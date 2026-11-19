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

type grubRunner struct {
	config string
	saved  string
	verify bool
}

func (r *grubRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	switch name {
	case grubMkconfig:
		contents := "menuentry 'Void Linux, with Linux 6.6.72_1' --id 'void-6.6' {\n}\nset default=\"${saved_entry}\"\n"
		return runner.Result{}, os.WriteFile(args[len(args)-1], []byte(contents), 0o644)
	case grubEditenv:
		if args[len(args)-1] == "list" {
			if r.verify {
				return runner.Result{Stdout: []byte("saved_entry=" + r.saved + "\n")}, nil
			}
			return runner.Result{Stdout: []byte("saved_entry=wrong\n")}, nil
		}
		if strings.HasPrefix(args[len(args)-1], "saved_entry=") {
			r.saved = strings.TrimPrefix(args[len(args)-1], "saved_entry=")
		}
		return runner.Result{}, nil
	default:
		return runner.Result{}, errors.New("unexpected command")
	}
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

func TestGRUBEntriesRequireExactNonRecoveryTarget(t *testing.T) {
	config := []byte("menuentry 'Void Linux, with Linux 6.12.10_1' --id 'gnulinux-6.12.10_1-advanced-uuid' {\n}\nmenuentry 'Void Linux, with Linux 6.12.10_1 (recovery mode)' --id 'recovery' {\n}\nmenuentry 'Void Linux, with Linux 6.12.10_10' --id 'other' {\n}\n")
	got := grubEntries(config, "6.12.10_1")
	if len(got) != 1 || got[0] != "gnulinux-6.12.10_1-advanced-uuid" {
		t.Fatalf("entries=%#v", got)
	}
}

func TestManagerSetsAndVerifiesSystemdBootDefault(t *testing.T) {
	root := t.TempDir()
	entries := filepath.Join(root, "boot", "loader", "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(entries, "void-6.6.conf"), []byte("title Void Linux\nlinux /vmlinuz-6.6.72_1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(runner.Exec{}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb"), filepath.Join(root, "fstab"))
	manager.bootRoot = filepath.Join(root, "boot")
	manager.efiMount = filepath.Join(root, "efi")
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.KernelSetDefault, Value: "6.6.72_1"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(root, "boot", "loader", "loader.conf"))
	if err != nil || directive(contents, "default") != "void-6.6" {
		t.Fatalf("loader.conf=%q err=%v", contents, err)
	}
}

func TestManagerRejectsAmbiguousSystemdBootTarget(t *testing.T) {
	root := t.TempDir()
	entries := filepath.Join(root, "boot", "loader", "entries")
	if err := os.MkdirAll(entries, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"first.conf", "second.conf"} {
		if err := os.WriteFile(filepath.Join(entries, name), []byte("linux /vmlinuz-6.6.72_1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manager := NewManager(runner.Exec{}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb"), filepath.Join(root, "fstab"))
	manager.bootRoot = filepath.Join(root, "boot")
	manager.efiMount = filepath.Join(root, "efi")
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.KernelSetDefault, Value: "6.6.72_1"}); err == nil {
		t.Fatal("ambiguous target was accepted")
	}
}

func TestManagerSetsAndVerifiesGRUBDefault(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "boot", "grub", "grub.cfg")
	defaults := filepath.Join(root, "default", "grub")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(defaults), 0o755); err != nil {
		t.Fatal(err)
	}
	initial := "menuentry 'Void Linux, with Linux 6.6.72_1' --id 'void-6.6' {\n}\n"
	if err := os.WriteFile(config, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaults, []byte("GRUB_DEFAULT=0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := &grubRunner{config: config, verify: true}
	manager := NewManager(command, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb"), filepath.Join(root, "fstab"))
	manager.bootRoot, manager.efiMount, manager.grubDefaults = filepath.Join(root, "boot"), filepath.Join(root, "efi"), defaults
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.KernelSetDefault, Value: "6.6.72_1"}); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(defaults)
	if directive(contents, "GRUB_DEFAULT") != "saved" || command.saved != "void-6.6" {
		t.Fatalf("defaults=%q saved=%q", contents, command.saved)
	}
}

func TestManagerRejectsUnverifiedGRUBDefault(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "boot", "grub", "grub.cfg")
	defaults := filepath.Join(root, "default", "grub")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(defaults), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("menuentry 'Void Linux, with Linux 6.6.72_1' --id 'void-6.6' {\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaults, []byte("GRUB_DEFAULT=0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&grubRunner{config: config}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb"), filepath.Join(root, "fstab"))
	manager.bootRoot, manager.efiMount, manager.grubDefaults = filepath.Join(root, "boot"), filepath.Join(root, "efi"), defaults
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.KernelSetDefault, Value: "6.6.72_1"}); err == nil {
		t.Fatal("unverified saved default was accepted")
	}
	contents, _ := os.ReadFile(defaults)
	if directive(contents, "GRUB_DEFAULT") != "0" {
		t.Fatalf("defaults were not rolled back: %q", contents)
	}
}
