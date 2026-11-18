package ui

import (
	"context"
	"testing"

	"sysentra/internal/backends"
	"sysentra/internal/core"
)

func TestModelViews(t *testing.T) {
	model := NewModel(backends.Mock{Packages: []core.PackageDetails{
		{Package: core.Package{Ref: core.PackageRef{BackendID: "mock", ID: "one"}, Name: "one", Installed: true}},
		{Package: core.Package{Ref: core.PackageRef{BackendID: "mock", ID: "two"}, Name: "two"}},
	}})

	found, err := model.Search(context.Background(), "two")
	if err != nil || len(found) != 1 {
		t.Fatalf("Search() = %#v, %v", found, err)
	}
	installed, err := model.Installed(context.Background())
	if err != nil || len(installed) != 1 {
		t.Fatalf("Installed() = %#v, %v", installed, err)
	}
}
