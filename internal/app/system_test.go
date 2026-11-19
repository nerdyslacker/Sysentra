package app

import (
	"context"
	"testing"

	"sysentra/internal/kernel"
	"sysentra/internal/repositories"
	"sysentra/internal/security"
	"sysentra/internal/services"
	"sysentra/internal/transactions"
)

type kernelReaderStub struct {
	kernels []kernel.Kernel
	status  kernel.Status
}

func (s kernelReaderStub) List(context.Context) ([]kernel.Kernel, error) { return s.kernels, nil }
func (s kernelReaderStub) Status(context.Context) (kernel.Status, error) { return s.status, nil }

type systemServiceClientStub struct {
	request security.PackageRequest
	commits int
}

func (s *systemServiceClientStub) ListServices(context.Context) ([]services.Service, error) {
	return nil, nil
}
func (s *systemServiceClientStub) ChangeService(context.Context, security.ServiceRequest) error {
	return nil
}
func (s *systemServiceClientStub) ChangeXBPSRule(context.Context, security.XBPSRuleRequest) error {
	return nil
}
func (s *systemServiceClientStub) ChangeSystemConfig(context.Context, security.SystemConfigRequest) error {
	return nil
}
func (s *systemServiceClientStub) ChangeAccount(context.Context, security.AccountRequest) error {
	return nil
}
func (s *systemServiceClientStub) Plan(_ context.Context, request security.PackageRequest) (transactions.Plan, error) {
	s.request = request
	return transactions.Plan{Request: request}, nil
}
func (s *systemServiceClientStub) Commit(_ context.Context, _ string) (transactions.Result, error) {
	s.commits++
	return transactions.Result{}, nil
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

func TestReconfigureRunningKernelRejectsOtherPackages(t *testing.T) {
	client := &systemServiceClientStub{}
	service := NewSystemService(kernelReaderStub{kernels: []kernel.Kernel{{Package: "linux6.12", Running: true}, {Package: "linux6.6", Installed: true}}}, nil, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil)
	if _, err := service.ReconfigureRunningKernel("linux6.6"); err == nil || client.commits != 0 {
		t.Fatalf("non-running kernel reached commit: commits=%d err=%v", client.commits, err)
	}
	kernels, err := service.ReconfigureRunningKernel("linux6.12")
	if err != nil || client.request.Operation != security.Reconfigure || len(client.request.Packages) != 1 || client.request.Packages[0] != "linux6.12" || client.commits != 1 || len(kernels) != 2 {
		t.Fatalf("request=%#v commits=%d kernels=%#v err=%v", client.request, client.commits, kernels, err)
	}
}

func TestGetKernelStatusMapsBootAndDiagnostics(t *testing.T) {
	service := NewSystemService(kernelReaderStub{status: kernel.Status{
		BootMode: "UEFI", Bootloader: "GRUB", Boot: kernel.Mount{Target: "/boot", Source: "/dev/vda2", FileSystem: "ext4", Size: 1000, Available: 400, Mounted: true},
		Images: []kernel.BootImage{{Version: "6.12.10_1", Status: "Running", Size: 42}}, DKMS: []kernel.DKMSModule{{Module: "zfs", Version: "2.2.3", Kernel: "6.12.10_1", Status: "installed"}},
	}}, nil, nil, nil, nil, nil, nil, nil, &systemServiceClientStub{}, nil, nil, nil, nil)
	got, err := service.GetKernelStatus()
	if err != nil || got.BootMode != "UEFI" || got.Bootloader != "GRUB" || len(got.Images) != 1 || got.Images[0].Status != "Running" || len(got.DKMS) != 1 {
		t.Fatalf("status=%#v err=%v", got, err)
	}
}
