package app

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sysentra/internal/accounts"
	"sysentra/internal/drivers"
	"sysentra/internal/firewall"
	"sysentra/internal/firmware"
	"sysentra/internal/kernel"
	"sysentra/internal/repositories"
	"sysentra/internal/security"
	"sysentra/internal/services"
	"sysentra/internal/shares"
	"sysentra/internal/transactions"
)

type kernelReaderStub struct {
	kernels []kernel.Kernel
	status  kernel.Status
}

func (s kernelReaderStub) List(context.Context) ([]kernel.Kernel, error) { return s.kernels, nil }
func (s kernelReaderStub) Status(context.Context) (kernel.Status, error) { return s.status, nil }

type systemServiceClientStub struct {
	request         security.PackageRequest
	systemRequest   security.SystemConfigRequest
	serviceRequest  security.ServiceRequest
	firewallRequest security.FirewallRequest
	commits         int
	listedServices  []services.Service
}

func (s *systemServiceClientStub) AccountStatus(context.Context) ([]accounts.Status, error) {
	return []accounts.Status{{Name: "alice", Password: "Locked", Processes: 3, Sessions: 1}}, nil
}

func (s *systemServiceClientStub) ListServices(context.Context) ([]services.Service, error) {
	return s.listedServices, nil
}
func (s *systemServiceClientStub) ServiceDetails(context.Context, string) (services.Details, error) {
	return services.Details{}, nil
}
func (s *systemServiceClientStub) ChangeService(_ context.Context, request security.ServiceRequest) error {
	s.serviceRequest = request
	return nil
}
func (s *systemServiceClientStub) ChangeXBPSRule(context.Context, security.XBPSRuleRequest) error {
	return nil
}
func (s *systemServiceClientStub) ChangeSystemConfig(_ context.Context, request security.SystemConfigRequest) error {
	s.systemRequest = request
	return nil
}
func (s *systemServiceClientStub) ChangeAccount(context.Context, security.AccountRequest) error {
	return nil
}
func (s *systemServiceClientStub) ChangeFirewall(_ context.Context, request security.FirewallRequest) error {
	s.firewallRequest = request
	return nil
}
func (s *systemServiceClientStub) ChangeFirmware(_ context.Context, request security.FirmwareRequest) (firmware.Result, error) {
	return firmware.Result{Diagnostics: []string{string(request.Operation)}}, nil
}

type accountReaderStub struct{}

type firewallReaderStub struct{ state firewall.State }

type firmwareReaderStub struct{ catalog firmware.Catalog }

type driverReaderStub struct{ catalog drivers.Catalog }

func (s firmwareReaderStub) List(context.Context) (firmware.Catalog, error) { return s.catalog, nil }
func (s driverReaderStub) List(context.Context) (drivers.Catalog, error)    { return s.catalog, nil }

func TestDriverCatalogPreservesVerifiedPackageActions(t *testing.T) {
	device := drivers.Device{Bus: "USB", ID: "0BDA:C811", Candidates: []drivers.Candidate{{Module: "8821cu", Package: "rtl8821cu-dkms", Available: true, CanInstall: true, Evidence: "Exact USB ID"}}}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, nil, driverReaderStub{catalog: drivers.Catalog{Devices: []drivers.Device{device}}}, nil, nil, nil, nil, nil, nil, nil)
	view, err := service.ListDrivers()
	if err != nil || len(view.Devices) != 1 || len(view.Devices[0].Candidates) != 1 || !view.Devices[0].Candidates[0].CanInstall {
		t.Fatalf("view=%#v err=%v", view, err)
	}
}

type sharesReaderStub struct{}

func (sharesReaderStub) Samba() (shares.SambaServer, error) { return shares.SambaServer{}, nil }
func (sharesReaderStub) NFS() ([]shares.NFSMount, error)    { return nil, nil }

func (s firewallReaderStub) ReadBackend(context.Context, string) (firewall.State, error) {
	return s.state, nil
}

func (accountReaderStub) List() ([]accounts.User, []accounts.Group, error) {
	return []accounts.User{{Name: "alice", UID: 1000, GID: 1000, Home: "/home/alice", Shell: "/bin/sh", Groups: []string{"alice"}}}, []accounts.Group{{Name: "alice", GID: 1000}}, nil
}

func TestAccountsIncludePrivilegedStatusAndHistory(t *testing.T) {
	client := &systemServiceClientStub{}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, accountReaderStub{})
	service.accountHistoryPath = filepath.Join(t.TempDir(), "accounts.json")
	view, err := service.ListAccounts()
	if err != nil || len(view.Users) != 1 || view.Users[0].Password != "Locked" || view.Users[0].Processes != 3 || view.Users[0].Sessions != 1 || view.Users[0].PrimaryGroup != "alice" {
		t.Fatalf("view=%#v err=%v", view, err)
	}
	if _, err := service.EditUser("alice", "/srv/alice", "/bin/zsh", true); err != nil {
		t.Fatal(err)
	}
	history, err := service.ListAccountHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 || history[0].Operation != string(security.UserEdit) || history[0].Target != "alice" {
		t.Fatalf("history=%#v", history)
	}
}

func TestFirewallUsesExplicitBackendAndTypedChange(t *testing.T) {
	client := &systemServiceClientStub{}
	inventory := firewallReaderStub{state: firewall.State{Backend: "firewalld", Installed: true, Active: true, Zone: "public", Backends: []string{"ufw", "firewalld"}, Rules: []firewall.Rule{{Kind: "service", Target: "https", Action: "allow", Source: "Any"}}}}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, inventory, nil, client, nil, nil, nil, nil, nil, nil)
	view, err := service.GetFirewall("firewalld")
	if err != nil || view.Zone != "public" || len(view.Backends) != 2 || !view.Rules[0].Mutable {
		t.Fatalf("view=%#v err=%v", view, err)
	}
	input := FirewallChangeInput{Operation: "rule-add", Backend: "firewalld", Zone: "public", Rule: FirewallRuleView{Kind: "port", Target: "443", Protocol: "tcp", Action: "allow", Source: "any"}}
	if _, err := service.ChangeFirewall(input); err != nil {
		t.Fatal(err)
	}
	if client.firewallRequest.Rule.Target != "443" || client.firewallRequest.Zone != "public" {
		t.Fatalf("request=%#v", client.firewallRequest)
	}
}

func TestShareChangesPreserveOldIdentifiers(t *testing.T) {
	client := &systemServiceClientStub{listedServices: []services.Service{{Name: "smbd", Enabled: true, State: "Running", PID: 123}, {Name: "nmbd", State: "Stopped"}, {Name: "sshd", Enabled: true, State: "Running"}}}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, sharesReaderStub{}, nil)
	input := SambaShareInput{Operation: "edit", Name: "Videos", OldName: "Media", Path: "/srv/videos", Comment: "Videos", Browseable: true, GuestOK: true, FollowSymlinks: true, ValidUsers: "alice, @media", WriteList: "@editors", ReadList: "bob", ForceUser: "media", ForceGroup: "media", CreateMask: "0770", DirectoryMask: "0775"}
	view, err := service.ChangeSambaShare(input)
	if err != nil {
		t.Fatal(err)
	}
	if client.systemRequest.Operation != security.SambaEdit || client.systemRequest.OldValue != "Media" || client.systemRequest.Comment != "Videos" || client.systemRequest.WriteList != "@editors" {
		t.Fatalf("request=%#v", client.systemRequest)
	}
	if len(view.Services) != 2 || view.Services[0].Name != "smbd" || view.Services[1].Name != "nmbd" {
		t.Fatalf("services=%#v", view.Services)
	}
	if _, err := service.ChangeNFSMount("edit", "server:/new", "server:/old", "/mnt/new", "/mnt/old", "nfs4", "rw,_netdev"); err != nil {
		t.Fatal(err)
	}
	if client.systemRequest.Operation != security.NFSEdit || client.systemRequest.OldPath != "/mnt/old" {
		t.Fatalf("request=%#v", client.systemRequest)
	}
}

func TestSambaGlobalConfigurationUsesTypedConfigAndActiveFirewall(t *testing.T) {
	client := &systemServiceClientStub{}
	inventory := firewallReaderStub{state: firewall.State{Backend: "firewalld", Installed: true, Active: true, Zone: "public"}}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, inventory, nil, client, nil, nil, nil, nil, sharesReaderStub{}, nil)
	_, err := service.ChangeSambaGlobal(SambaGlobalInput{Protocol: "SMB3", DefaultMask: "0775", Printing: true, MapGuest: true, FirewallAllowed: true})
	if err != nil {
		t.Fatal(err)
	}
	if client.systemRequest.Operation != security.SambaGlobalSet || client.systemRequest.Protocol != "SMB3" || !client.systemRequest.Printing || client.firewallRequest.Operation != security.FirewallRuleAdd || client.firewallRequest.Rule.Target != "samba" || client.firewallRequest.Zone != "public" {
		t.Fatalf("system=%#v firewall=%#v", client.systemRequest, client.firewallRequest)
	}
}

func TestSambaFirewallDistinguishesPartialAndCompleteUFWAccess(t *testing.T) {
	state := firewall.State{Backend: "ufw", Rules: []firewall.Rule{{Kind: "port", Target: "445", Protocol: "tcp", Action: "allow"}}}
	if sambaFirewallAllowed(state) || !sambaFirewallConfigured(state) {
		t.Fatalf("partial Samba access was classified incorrectly: %#v", state)
	}
	state.Rules = append(state.Rules,
		firewall.Rule{Kind: "port", Target: "137", Protocol: "udp", Action: "allow"},
		firewall.Rule{Kind: "port", Target: "138", Protocol: "udp", Action: "allow"},
		firewall.Rule{Kind: "port", Target: "139", Protocol: "tcp", Action: "allow"},
	)
	if !sambaFirewallAllowed(state) {
		t.Fatalf("complete Samba access was not detected: %#v", state)
	}
}

func TestFirmwareUpdateReturnsDiagnosticsAndRefreshedCatalog(t *testing.T) {
	client := &systemServiceClientStub{}
	inventory := firmwareReaderStub{catalog: firmware.Catalog{Available: true, Devices: []firmware.Device{{ID: "device-1", Name: "System Firmware", Version: "1.2.3"}}}}
	service := NewSystemService(nil, nil, nil, nil, nil, inventory, nil, nil, client, nil, nil, nil, nil, nil, nil)
	result, err := service.ChangeFirmware("update", "device-1", "1.2.3")
	if err != nil || len(result.Diagnostics) != 1 || len(result.Catalog.Devices) != 1 || result.Catalog.Devices[0].Version != "1.2.3" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}
func (s *systemServiceClientStub) Plan(_ context.Context, request security.PackageRequest) (transactions.Plan, error) {
	s.request = request
	return transactions.Plan{Request: request}, nil
}
func (s *systemServiceClientStub) Commit(_ context.Context, _ string) (transactions.Result, error) {
	s.commits++
	return transactions.Result{Diagnostics: []string{"regenerating initramfs", "done"}}, nil
}

func TestCurrentMirrorBaseUsesFirstEnabledCurrentRepository(t *testing.T) {
	items := []repositories.Repository{
		{URL: "https://disabled.example/current", Enabled: false},
		{URL: "https://mirror.example/void/current/nonfree", Enabled: true},
	}
	if got := currentMirrorBase(items); got != "https://mirror.example/void" {
		t.Fatalf("current mirror = %q", got)
	}
}

func TestAddSystemServiceUsesValidatedPrivilegedRequest(t *testing.T) {
	client := &systemServiceClientStub{}
	service := NewSystemService(nil, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil)
	if _, err := service.AddService("system", "backup", "sh --archive /home/", "root", "wheel"); err != nil {
		t.Fatal(err)
	}
	if client.serviceRequest.Operation != security.ServiceAdd || client.serviceRequest.Name != "backup" || !strings.HasSuffix(client.serviceRequest.Command, "/sh") || !reflect.DeepEqual(client.serviceRequest.Arguments, []string{"--archive", "/home/"}) || client.serviceRequest.User != "root" || client.serviceRequest.Group != "wheel" {
		t.Fatalf("request=%#v", client.serviceRequest)
	}
	if _, err := service.AddService("turnstile", "backup", "sh", "", ""); err == nil {
		t.Fatal("unsupported service scope accepted")
	}
}

func TestReconfigureKernelAcceptsEveryInstalledPackage(t *testing.T) {
	client := &systemServiceClientStub{}
	service := NewSystemService(kernelReaderStub{kernels: []kernel.Kernel{{Package: "linux6.12", Installed: true, Running: true}, {Package: "linux6.6", Installed: true}, {Package: "linux6.1"}}}, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil)
	if _, err := service.ReconfigureKernel("linux6.6"); err != nil || client.commits != 1 {
		t.Fatalf("installed kernel reconfiguration failed: commits=%d err=%v", client.commits, err)
	}
	result, err := service.ReconfigureKernel("linux6.12")
	if err != nil || client.request.Operation != security.Reconfigure || len(client.request.Packages) != 1 || client.request.Packages[0] != "linux6.12" || client.commits != 2 || len(result.Diagnostics) != 2 {
		t.Fatalf("request=%#v commits=%d result=%#v err=%v", client.request, client.commits, result, err)
	}
	if _, err := service.ReconfigureKernel("linux6.1"); err == nil || client.commits != 2 {
		t.Fatalf("available kernel reached commit: commits=%d err=%v", client.commits, err)
	}
}

func TestSetDefaultKernelRequiresInstalledNonRunningKernel(t *testing.T) {
	client := &systemServiceClientStub{}
	service := NewSystemService(kernelReaderStub{kernels: []kernel.Kernel{{Package: "linux6.12", Version: "6.12.10_1", Installed: true, Running: true}, {Package: "linux6.6", Version: "6.6.72_1", Installed: true}}}, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil)
	if _, err := service.SetDefaultKernel("linux6.6"); err != nil || client.systemRequest.Operation != security.KernelSetDefault || client.systemRequest.Value != "6.6.72_1" {
		t.Fatalf("request=%#v err=%v", client.systemRequest, err)
	}
	if _, err := service.SetDefaultKernel("linux6.12"); err == nil {
		t.Fatal("running kernel accepted as a default-kernel change")
	}
}

func TestGetKernelStatusMapsBootAndDiagnostics(t *testing.T) {
	service := NewSystemService(kernelReaderStub{status: kernel.Status{
		BootMode: "UEFI", Bootloader: "GRUB", Boot: kernel.Mount{Target: "/boot", Source: "/dev/vda2", FileSystem: "ext4", Size: 1000, Available: 400, Mounted: true},
		Images: []kernel.BootImage{{Version: "6.12.10_1", Status: "Running", Size: 42}}, DKMS: []kernel.DKMSModule{{Module: "zfs", Version: "2.2.3", Kernel: "6.12.10_1", Status: "installed"}},
	}}, nil, nil, nil, nil, nil, nil, nil, &systemServiceClientStub{}, nil, nil, nil, nil, nil, nil)
	got, err := service.GetKernelStatus()
	if err != nil || got.BootMode != "UEFI" || got.Bootloader != "GRUB" || len(got.Images) != 1 || got.Images[0].Status != "Running" || len(got.DKMS) != 1 {
		t.Fatalf("status=%#v err=%v", got, err)
	}
}
