package backends

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"sysentra/internal/core"
	"sysentra/internal/runner"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestFlatpakListInstalledIncludesVersionAndScope(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","description":"Example application","version":"2.4","branch":"stable","origin":"flathub","installation":"user"}]`)}}
	backend := NewFlatpak(command)

	packages, err := backend.ListInstalled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{
		Ref: core.PackageRef{BackendID: "flatpak", ID: "org.example.App"}, Name: "Example", Description: "Example application", Version: "2.4", Architecture: "", Origin: "flathub", Scope: "user", Installed: true,
	}}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %#v, want %#v", packages, want)
	}
	if command.name != "flatpak" || !reflect.DeepEqual(command.args, []string{"list", "--app", "--columns=name,description,application,version,branch,arch,origin,installation", "-j"}) {
		t.Fatalf("command = %q %q", command.name, command.args)
	}
}

func TestFlatpakListAvailable(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","version":"2.5","branch":"stable","origin":"flathub"}]`)}}
	backend := NewFlatpak(command)

	packages, err := backend.ListAvailable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Version != "2.5" || packages[0].Installed || packages[0].Scope != "system" {
		t.Fatalf("unexpected packages: %#v", packages)
	}
	if !reflect.DeepEqual(command.args, []string{"remote-ls", "--app", "--columns=name,description,application,version,branch,arch,origin", "-j"}) {
		t.Fatalf("args = %q", command.args)
	}
}

func TestFlatpakListUpdatesUsesRemoteVersion(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","version":"2.5","origin":"flathub","installation":"system"}]`)}}
	backend := NewFlatpak(command)

	packages, err := backend.ListUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].NewVersion != "2.5" || !packages[0].Installed {
		t.Fatalf("unexpected updates: %#v", packages)
	}
	if !reflect.DeepEqual(command.args, []string{"remote-ls", "--updates", "--app", "--columns=name,description,application,version,branch,arch,origin", "-j"}) {
		t.Fatalf("args = %q", command.args)
	}
}

func TestFlatpakGetInstalledDetails(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","description":"Example application","version":"2.4","arch":"x86_64","origin":"flathub","installation":"system"}]`)}},
		{result: runner.Result{Stdout: []byte("1048576\n")}},
		{result: runner.Result{Stdout: []byte("org.freedesktop.Platform/x86_64/25.08\n")}},
	}}
	backend := NewFlatpak(command)
	backend.iconRoots = nil

	details, err := backend.Get(context.Background(), core.PackageRef{BackendID: "flatpak", ID: "org.example.App"})
	if err != nil {
		t.Fatal(err)
	}
	if details.Name != "Example" || !details.Installed || details.InstalledSize != "1.0 MB" || !reflect.DeepEqual(details.Requires, []string{"org.freedesktop.Platform/x86_64/25.08"}) {
		t.Fatalf("details = %#v", details)
	}
}

func TestFlatpakGetAvailableDetails(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte(`[]`)}},
		{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","description":"Example application","version":"2.5","arch":"x86_64","origin":"flathub"}]`)}},
	}}
	backend := NewFlatpak(command)
	backend.iconRoots = nil

	details, err := backend.Get(context.Background(), core.PackageRef{BackendID: "flatpak", ID: "org.example.App"})
	if err != nil {
		t.Fatal(err)
	}
	if details.Name != "Example" || details.Installed || details.Version != "2.5" {
		t.Fatalf("details = %#v", details)
	}
}

func TestFlatpakGetEnrichesFlathubDetails(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"description":"<p>A <em>complete</em> description.</p>","developer_name":"Example Org","project_license":"GPL-3.0-or-later","urls":{"homepage":"https://example.org"},"bundle":{"runtime":"org.freedesktop.Platform/x86_64/25.08"}}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte(`[]`)}},
		{result: runner.Result{Stdout: []byte(`[{"application_id":"org.example.App","name":"Example","origin":"flathub"}]`)}},
	}}
	backend := NewFlatpak(command, WithFlathubMetadata("https://flathub.example/api/v2/appstream", client))
	backend.iconRoots = nil
	details, err := backend.Get(context.Background(), core.PackageRef{BackendID: "flatpak", ID: "org.example.App"})
	if err != nil {
		t.Fatal(err)
	}
	if details.Description != "A complete description." || details.Homepage != "https://example.org" || details.License != "GPL-3.0-or-later" || details.Maintainer != "Example Org" || !reflect.DeepEqual(details.Requires, []string{"org.freedesktop.Platform/x86_64/25.08"}) {
		t.Fatalf("details = %#v", details)
	}
}
