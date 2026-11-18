package backends

import (
	"context"
	"errors"
	"testing"

	"sysentra/internal/core"
)

func testBackend() Mock {
	return Mock{Packages: []core.PackageDetails{
		{Package: core.Package{Ref: core.PackageRef{BackendID: "mock", ID: "nano"}, Name: "nano", Installed: true}, Description: "text editor"},
		{Package: core.Package{Ref: core.PackageRef{BackendID: "mock", ID: "ripgrep"}, Name: "ripgrep"}, Description: "fast search tool"},
	}}
}

func TestMockSearchAndInstalled(t *testing.T) {
	backend := testBackend()

	found, err := backend.Search(context.Background(), "SEARCH")
	if err != nil || len(found) != 1 || found[0].Name != "ripgrep" {
		t.Fatalf("Search() = %#v, %v", found, err)
	}

	installed, err := backend.ListInstalled(context.Background())
	if err != nil || len(installed) != 1 || installed[0].Name != "nano" {
		t.Fatalf("ListInstalled() = %#v, %v", installed, err)
	}
}

func TestMockGetMissing(t *testing.T) {
	_, err := testBackend().Get(context.Background(), core.PackageRef{BackendID: "mock", ID: "missing"})
	if !errors.Is(err, ErrPackageNotFound) {
		t.Fatalf("Get() error = %v", err)
	}
}
