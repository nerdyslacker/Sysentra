package app

import (
	"context"
	"testing"

	"sysentra/internal/core"
	"sysentra/internal/settings"
)

type updateStoreStub struct{ value settings.UpdateChecks }

func (s *updateStoreStub) Read() (settings.UpdateChecks, error) {
	if s.value.IntervalMinutes == 0 {
		return settings.UpdateChecks{Scheduled: true, Notifications: true, IntervalMinutes: 360}, nil
	}
	return s.value, nil
}

func (s *updateStoreStub) Write(value settings.UpdateChecks) error { s.value = value; return nil }

func TestUpdateMonitorChecksAllEnabledSources(t *testing.T) {
	packages := NewPackageService(
		packageReaderStub{updates: []core.Package{{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash"}}},
		packageReaderStub{id: "flatpak", updates: []core.Package{{Ref: core.PackageRef{BackendID: "flatpak", ID: "org.example.App"}, Name: "Example"}}},
	)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service, err := NewUpdateMonitorService(ctx, packages, &updateStoreStub{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.CheckForUpdates()
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 2 || result.XBPS != 1 || result.Flatpak != 1 || result.CheckedAt == "" {
		t.Fatalf("result = %#v", result)
	}
}

func TestUpdateMonitorPersistsPreferences(t *testing.T) {
	store := &updateStoreStub{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service, err := NewUpdateMonitorService(ctx, NewPackageService(packageReaderStub{}), store, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := service.SetUpdatePreferences(false, false, 120)
	if err != nil {
		t.Fatal(err)
	}
	if got.Scheduled || got.Notifications || got.IntervalMinutes != 120 {
		t.Fatalf("preferences = %#v", got)
	}
}

func TestUpdateMonitorPublishesFrontendCount(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got UpdateCheckResultView
	service, err := NewUpdateMonitorService(ctx, NewPackageService(packageReaderStub{}), &updateStoreStub{}, func(result UpdateCheckResultView, _ bool) { got = result })
	if err != nil {
		t.Fatal(err)
	}
	if err := service.PublishUpdateCount(4); err != nil {
		t.Fatal(err)
	}
	if got.Total != 4 {
		t.Fatalf("published count = %d", got.Total)
	}
	if err := service.PublishUpdateCount(-1); err == nil {
		t.Fatal("expected negative count rejection")
	}
}
