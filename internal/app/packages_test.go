package app

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"sysentra/internal/core"
)

type packageReaderStub struct {
	id        string
	packages  []core.Package
	available []core.Package
	updates   []core.Package
	err       error
}

type packageOwnerStub struct{ packageReaderStub }

func (packageOwnerStub) FindOwners(context.Context, string) ([]string, error) {
	return []string{"bash-5.3_2: /usr/bin/bash (regular file)"}, nil
}

type packageDetailsStub struct{ packageReaderStub }

func (packageDetailsStub) Get(context.Context, core.PackageRef) (core.PackageDetails, error) {
	return core.PackageDetails{
		Package:      core.Package{Name: "bash", Version: "5.3_2", Origin: "current", Installed: true},
		Description:  "GNU Bourne Again Shell",
		Homepage:     "https://www.gnu.org/software/bash/",
		License:      "GPL-3.0-or-later",
		Dependencies: []string{"glibc>=2.41_1"},
	}, nil
}

func (s packageReaderStub) ID() string {
	if s.id == "" {
		return "xbps"
	}
	return s.id
}

func (s packageReaderStub) ListInstalled(context.Context) ([]core.Package, error) {
	return s.packages, s.err
}

func (s packageReaderStub) ListAvailable(context.Context) ([]core.Package, error) {
	return s.available, s.err
}

func (s packageReaderStub) ListUpdates(context.Context) ([]core.Package, error) {
	return s.updates, s.err
}

func TestPackageServiceListsInstalledPackages(t *testing.T) {
	service := NewPackageService(packageReaderStub{packages: []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", Description: "GNU Bourne Again Shell", Version: "5.3_2", Origin: "current", Installed: true},
	}})

	catalog, err := service.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	want := []PackageView{{BackendID: "xbps", ID: "bash", Name: "bash", Version: "5.3_2", Repository: "current", Status: "Installed", Installed: true, Description: "GNU Bourne Again Shell"}}
	if !reflect.DeepEqual(catalog.Packages, want) {
		t.Fatalf("packages = %#v, want %#v", catalog.Packages, want)
	}
}

func TestPackageServicePreservesBackendError(t *testing.T) {
	want := errors.New("inventory unavailable")
	service := NewPackageService(packageReaderStub{err: want})

	_, err := service.ListInstalled()
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
}

func TestPackageServiceDiscoversAvailablePackages(t *testing.T) {
	service := NewPackageService(packageReaderStub{available: []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "ripgrep"}, Name: "ripgrep", Version: "15.1.0_1"},
	}})

	catalog, err := service.Discover()
	if err != nil {
		t.Fatal(err)
	}
	want := []PackageView{{BackendID: "xbps", ID: "ripgrep", Name: "ripgrep", Version: "15.1.0_1", Status: "Available"}}
	if !reflect.DeepEqual(catalog.Packages, want) {
		t.Fatalf("packages = %#v, want %#v", catalog.Packages, want)
	}
}

func TestPackageServiceListsUpdates(t *testing.T) {
	service := NewPackageService(packageReaderStub{packages: []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", Version: "5.3_2", Origin: "https://repo.example/current", Installed: true},
	}, updates: []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", NewVersion: "5.4_1", Installed: true},
	}})

	catalog, err := service.ListUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packages) != 1 || catalog.Packages[0].NewVersion != "5.4_1" || catalog.Packages[0].Repository != "https://repo.example/current" || catalog.Packages[0].Status != "Update available" {
		t.Fatalf("unexpected updates: %#v", catalog)
	}
}

func TestPackageServiceIncludesOptionalFlatpakVersions(t *testing.T) {
	service := NewPackageService(
		packageReaderStub{},
		packageReaderStub{id: "flatpak", packages: []core.Package{{Ref: core.PackageRef{BackendID: "flatpak", ID: "org.example.App"}, Name: "Example", Version: "2.4", Scope: "user", Installed: true}}},
	)

	catalog, err := service.ListInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packages) != 1 || catalog.Packages[0].BackendID != "flatpak" || catalog.Packages[0].Version != "2.4" || catalog.Packages[0].Scope != "user" {
		t.Fatalf("unexpected catalog: %#v", catalog)
	}
}

func TestPackageServiceReportsOptionalBackendFailure(t *testing.T) {
	service := NewPackageService(packageReaderStub{}, packageReaderStub{id: "flatpak", err: errors.New("offline")})

	catalog, err := service.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Warnings) != 1 {
		t.Fatalf("warnings = %#v, want one", catalog.Warnings)
	}
}

func TestPackageServiceDoesNotQueryDisabledOptionalSource(t *testing.T) {
	service := NewPackageService(packageReaderStub{}, packageReaderStub{id: "flatpak", err: errors.New("must not be queried")})
	if _, err := service.SetAdditionalSourceEnabled("flatpak", false); err != nil {
		t.Fatal(err)
	}
	catalog, err := service.Discover()
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Packages) != 0 || len(catalog.Warnings) != 0 {
		t.Fatalf("disabled source affected catalog: %#v", catalog)
	}
}

func TestPackageServiceRejectsUnknownAdditionalSource(t *testing.T) {
	service := NewPackageService(packageReaderStub{}, packageReaderStub{id: "flatpak"})
	if _, err := service.SetAdditionalSourceEnabled("unknown", false); err == nil {
		t.Fatal("expected unknown source error")
	}
}

func TestPackageServiceFindsOwningPackage(t *testing.T) {
	service := NewPackageService(packageOwnerStub{})
	result, err := service.FindOwningPackage(" /usr/bin/bash ")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Matches) != 1 || result.Matches[0] != "bash-5.3_2: /usr/bin/bash (regular file)" {
		t.Fatalf("result = %#v", result)
	}
}

func TestPackageServiceRejectsEmptyOwnerQuery(t *testing.T) {
	service := NewPackageService(packageOwnerStub{})
	if _, err := service.FindOwningPackage("  "); err == nil {
		t.Fatal("expected empty query error")
	}
}

func TestPackageServiceReturnsPackageDetails(t *testing.T) {
	service := NewPackageService(packageDetailsStub{})
	details, err := service.GetPackageDetails("xbps", "bash")
	if err != nil {
		t.Fatal(err)
	}
	if details.Name != "bash" || details.Description != "GNU Bourne Again Shell" || len(details.Dependencies) != 1 || !details.Installed {
		t.Fatalf("details = %#v", details)
	}
	if details.Provides == nil || details.Requires == nil || details.Exports == nil || details.ReverseDependencies == nil || details.Files == nil {
		t.Fatalf("detail lists must be empty arrays, got %#v", details)
	}
}
