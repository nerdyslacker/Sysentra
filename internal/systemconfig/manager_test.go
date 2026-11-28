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

type successfulRunner struct{}

func (successfulRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return runner.Result{}, nil
}

type recordingRunner struct {
	calls     [][]string
	fail      string
	stderr    string
	svRunning bool
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.calls = append(r.calls, append([]string{name}, args...))
	if name == "/usr/bin/sv" && r.svRunning {
		return runner.Result{Stdout: []byte("run: smbd: (pid 42) 10s\n")}, nil
	}
	if r.fail != "" && strings.Contains(strings.Join(append([]string{name}, args...), " "), r.fail) {
		return runner.Result{Stderr: []byte(r.stderr)}, errors.New("command failed")
	}
	return runner.Result{}, nil
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
	manager := NewManager(successfulRunner{}, config, filepath.Join(root, "vendor"), samba, fstab)
	requests := []security.SystemConfigRequest{
		{Version: security.APIVersion, Operation: security.RepositoryAdd, Value: "https://repo.example.org/current", Path: "games.conf", TrustConfirmed: true},
		{Version: security.APIVersion, Operation: security.SambaAdd, Value: "Media", Path: "/srv/media", Comment: "Shared media", Browseable: true, GuestOK: true, FollowSymlinks: true, ValidUsers: "alice, @media", WriteList: "@editors", ReadList: "bob", ForceUser: "media", ForceGroup: "media", CreateMask: "0770", DirectoryMask: "0775"},
		{Version: security.APIVersion, Operation: security.NFSAdd, Value: "server:/data", Path: filepath.Join(root, "mnt"), Type: "nfs4", Options: "rw,_netdev"},
	}
	for _, request := range requests {
		if err := manager.Change(request); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	checks := map[string]string{
		filepath.Join(config, "games.conf"): "repository=https://repo.example.org/current",
		samba:                               "[Media]",
		fstab:                               "server:/data",
	}
	for path, want := range checks {
		contents, err := os.ReadFile(path)
		if err != nil || !strings.Contains(string(contents), want) {
			t.Fatalf("%s = %q, %v", path, contents, err)
		}
	}
	sambaContents, _ := os.ReadFile(samba)
	for _, want := range []string{"comment = Shared media", "browseable = yes", "follow symlinks = yes", "valid users = alice, @media", "write list = @editors", "read list = bob", "force user = media", "force group = media", "create mask = 0770", "directory mask = 0775"} {
		if !strings.Contains(string(sambaContents), want) {
			t.Fatalf("samba config missing %q: %s", want, sambaContents)
		}
	}
}

func TestManagerShadowsVendorRepositoryBeforeEditing(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "etc")
	vendor := filepath.Join(root, "share")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vendor, 0o700); err != nil {
		t.Fatal(err)
	}
	vendorPath := filepath.Join(vendor, "community.conf")
	original := "repository=https://old.example.org/current\nignorepkg=keep-me\n"
	if err := os.WriteFile(vendorPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(successfulRunner{}, local, vendor, filepath.Join(root, "smb.conf"), filepath.Join(root, "fstab"))
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.RepositoryEdit, Value: "https://new.example.org/current", OldValue: "https://old.example.org/current", Path: "community.conf", TrustConfirmed: true}
	if err := manager.Change(request); err != nil {
		t.Fatal(err)
	}
	localContents, err := os.ReadFile(filepath.Join(local, "community.conf"))
	if err != nil || !strings.Contains(string(localContents), "repository=https://new.example.org/current") || !strings.Contains(string(localContents), "ignorepkg=keep-me") {
		t.Fatalf("local shadow=%q err=%v", localContents, err)
	}
	vendorContents, _ := os.ReadFile(vendorPath)
	if string(vendorContents) != original {
		t.Fatalf("vendor file changed: %q", vendorContents)
	}
}

func TestManagerEditsAndRemovesSambaSharesAfterValidation(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	if err := os.WriteFile(config, []byte("[global]\nworkgroup = WORKGROUP\n\n[Media]\npath = /srv/old\nread only = yes\n\n[Keep]\npath = /srv/keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := &recordingRunner{svRunning: true}
	manager := NewManager(commands, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), config, filepath.Join(root, "fstab"))
	edit := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaEdit, OldValue: "Media", Value: "Videos", Path: "/srv/videos", GuestOK: true}
	if err := manager.Change(edit); err != nil {
		t.Fatal(err)
	}
	remove := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaRemove, Value: "Videos"}
	if err := manager.Change(remove); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(config)
	if strings.Contains(string(contents), "[Media]") || strings.Contains(string(contents), "[Videos]") || !strings.Contains(string(contents), "[Keep]") {
		t.Fatalf("config=%q", contents)
	}
	if len(commands.calls) != 6 || commands.calls[0][0] != "/usr/bin/testparm" || commands.calls[1][0] != "/usr/bin/sv" || commands.calls[2][0] != "/usr/bin/smbcontrol" {
		t.Fatalf("calls=%#v", commands.calls)
	}
}

func TestManagerKeepsSambaConfigWhenTestparmFails(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	original := []byte("[global]\nworkgroup = WORKGROUP\n")
	if err := os.WriteFile(config, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(&recordingRunner{fail: "testparm"}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), config, filepath.Join(root, "fstab"))
	err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaAdd, Value: "Media", Path: "/srv/media"})
	contents, _ := os.ReadFile(config)
	if err == nil || string(contents) != string(original) {
		t.Fatalf("err=%v config=%q", err, contents)
	}
}

func TestManagerPreservesUnknownSambaShareOptionsOnEdit(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	original := []byte("[Media]\npath = /srv/old\nvfs objects = recycle\nrecycle:keeptree = yes\n")
	if err := os.WriteFile(config, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(successfulRunner{}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), config, filepath.Join(root, "fstab"))
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaEdit, OldValue: "Media", Value: "Media", Path: "/srv/new", Browseable: true, FollowSymlinks: true, CreateMask: "0744", DirectoryMask: "0755"}
	if err := manager.Change(request); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(config)
	if !strings.Contains(string(contents), "vfs objects = recycle") || !strings.Contains(string(contents), "recycle:keeptree = yes") {
		t.Fatalf("unknown options were lost: %s", contents)
	}
}

func TestManagerUpdatesSambaGlobalConfiguration(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	original := []byte("[global]\nworkgroup = WORKGROUP\nserver string = Void\n\n[Media]\npath = /srv/media\n")
	if err := os.WriteFile(config, original, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(successfulRunner{}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), config, filepath.Join(root, "fstab"))
	request := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.SambaGlobalSet, Protocol: "SMB3", DefaultMask: "0775", Printing: true, MapGuest: true}
	if err := manager.Change(request); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(config)
	for _, want := range []string{"server string = Void", "server min protocol = SMB3", "client min protocol = SMB3", "create mask = 0775", "directory mask = 0775", "map to guest = Bad User", "# BEGIN SYSENTRA PRINTING", "[printers]", "[print$]", "[Media]"} {
		if !strings.Contains(string(contents), want) {
			t.Fatalf("global config missing %q: %s", want, contents)
		}
	}
}

func TestManagerPreservesUnclosedSambaPrintingMarker(t *testing.T) {
	lines := []string{"[global]", "# BEGIN SYSENTRA PRINTING", "server string = keep", "[Media]", "path = /srv/media"}
	got := strings.Join(removeManagedSambaPrinting(lines), "\n")
	if !strings.Contains(got, "server string = keep") || !strings.Contains(got, "[Media]") {
		t.Fatalf("unclosed marker discarded config: %q", got)
	}
}

func TestManagerEditsRemovesAndMountsNFS(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	if err := os.WriteFile(fstab, []byte("server:/old /mnt/old nfs4 rw,_netdev 0 0 # sysentra\nkeep:/data /mnt/keep nfs rw 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := &recordingRunner{}
	manager := NewManager(commands, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb.conf"), fstab)
	manager.mounts = filepath.Join(root, "mounts")
	if err := os.WriteFile(manager.mounts, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	edit := security.SystemConfigRequest{Version: security.APIVersion, Operation: security.NFSEdit, OldValue: "server:/old", OldPath: "/mnt/old", Value: "server:/new", Path: filepath.Join(root, "mnt-new"), Type: "nfs4", Options: "rw,_netdev"}
	if err := manager.Change(edit); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.NFSMount, Value: "server:/new", Path: edit.Path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.mounts, []byte("server:/new "+escapeFstab(edit.Path)+" nfs4 rw 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.NFSUnmount, Value: "server:/new", Path: edit.Path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manager.mounts, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.NFSRemove, Value: "server:/new", Path: edit.Path}); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(fstab)
	if strings.Contains(string(contents), "server:/") || !strings.Contains(string(contents), "keep:/data") {
		t.Fatalf("fstab=%q", contents)
	}
	if len(commands.calls) != 2 || commands.calls[0][0] != "/usr/bin/mount" || commands.calls[1][0] != "/usr/bin/umount" {
		t.Fatalf("calls=%#v", commands.calls)
	}
}

func TestManagerRollsBackPartialNFSFailure(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	if err := os.WriteFile(fstab, []byte("server:/data /mnt/data nfs4 rw,_netdev 0 0 # sysentra\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := &recordingRunner{fail: "/usr/bin/mount", stderr: "mount.nfs4: No route to host"}
	manager := NewManager(commands, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb.conf"), fstab)
	err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.NFSMount, Value: "server:/data", Path: "/mnt/data"})
	if err == nil || !strings.Contains(err.Error(), "No route to host") || len(commands.calls) != 2 || commands.calls[1][0] != "/usr/bin/umount" {
		t.Fatalf("err=%v calls=%#v", err, commands.calls)
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

func TestMirrorChangeBacksUpAndReplacesExistingOverride(t *testing.T) {
	root := t.TempDir()
	vendor := filepath.Join(root, "vendor")
	config := filepath.Join(root, "xbps.d")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatal(err)
	}
	name := "00-repository-main.conf"
	if err := os.WriteFile(filepath.Join(vendor, name), []byte("repository=https://repo-default.voidlinux.org/current\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := []byte("repository=https://old.example/current\n")
	if err := os.WriteFile(filepath.Join(config, name), previous, 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(successfulRunner{}, config, vendor, filepath.Join(root, "smb.conf"), filepath.Join(root, "fstab"))
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.MirrorUse, Value: "https://new.example/"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(config, name))
	if err != nil || !strings.Contains(string(contents), managedMarker+"\nrepository=https://new.example/current") {
		t.Fatalf("override=%q err=%v", contents, err)
	}
	backup, err := os.ReadFile(filepath.Join(config, name+".bak"))
	if err != nil || string(backup) != string(previous) {
		t.Fatalf("backup=%q err=%v", backup, err)
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

func TestChangesGlobalEnvironmentWithoutDroppingComments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "environment")
	if err := os.WriteFile(path, []byte("# locale\nLANG=en_US.UTF-8\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(successfulRunner{}, filepath.Join(root, "xbps"), filepath.Join(root, "vendor"), filepath.Join(root, "smb"), filepath.Join(root, "fstab"))
	manager.environment = path
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.EnvironmentGlobalSet, Value: "EDITOR", Path: "helix --tutor"}); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(path)
	if !strings.Contains(string(contents), "# locale\n") || !strings.Contains(string(contents), `EDITOR="helix --tutor"`) {
		t.Fatalf("contents = %q", contents)
	}
	if err := manager.Change(security.SystemConfigRequest{Version: security.APIVersion, Operation: security.EnvironmentGlobalRemove, Value: "EDITOR"}); err != nil {
		t.Fatal(err)
	}
}
