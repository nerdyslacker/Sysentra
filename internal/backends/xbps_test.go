package backends

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"sysentra/internal/core"
	"sysentra/internal/runner"
)

func TestXBPSCachedVersionsMatchExactPackageAndExcludeInstalled(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"firefox-130.0_1.x86_64.xbps", "firefox-131.0_1.x86_64.xbps", "firefox-esr-128.0_1.x86_64.xbps"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	backend := NewXBPS(&stubRunner{})
	backend.cacheDir = dir
	got := backend.cachedVersions("firefox", "131.0_1")
	if !reflect.DeepEqual(got, []string{"130.0_1"}) {
		t.Fatalf("cached versions = %#v", got)
	}
}

func TestXBPSListInstalledIncludesRecordedRepository(t *testing.T) {
	path := t.TempDir() + "/pkgdb.plist"
	fixture := `<?xml version="1.0"?><plist><dict><key>bash</key><dict><key>repository</key><string>https://repo.example/current</string></dict></dict></plist>`
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	backend := NewXBPSWithPackageDB(&stubRunner{result: runner.Result{Stdout: []byte("ii bash-5.3_2 Shell\n")}}, path)

	packages, err := backend.ListInstalled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if packages[0].Origin != "https://repo.example/current" {
		t.Fatalf("origin = %q", packages[0].Origin)
	}
}

type stubRunner struct {
	result runner.Result
	err    error
	name   string
	args   []string
}

type runnerReply struct {
	result runner.Result
	err    error
}

type exitError int

func (e exitError) Error() string { return "command failed" }
func (e exitError) ExitCode() int { return int(e) }

type sequenceRunner struct {
	replies []runnerReply
	calls   int
}

type repositoryRunner struct{}

func (repositoryRunner) Run(_ context.Context, _ string, args ...string) (runner.Result, error) {
	if reflect.DeepEqual(args, []string{"--list-repos"}) {
		return runner.Result{Stdout: []byte("14841 https://repo.example/current (RSA signed)\n67 https://repo.example/current/nonfree (RSA signed)\n")}, nil
	}
	if len(args) > 2 && args[2] == "https://repo.example/current" {
		return runner.Result{Stdout: []byte("[-] 0ad-0.28.0_1 Strategy game\n[*] bash-5.3_2 Shell\n")}, nil
	}
	return runner.Result{Stdout: []byte("[-] UEFITool-A76_1 Firmware image tool\n")}, nil
}

func (r *sequenceRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	reply := r.replies[r.calls]
	r.calls++
	return reply.result, reply.err
}

func (r *stubRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.name = name
	r.args = args
	return r.result, r.err
}

func TestXBPSListInstalled(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte("ii bash-5.3_2 GNU Bourne Again Shell\nii MesaLib-devel-26.2.7_1 Open source graphics stack\n")}}
	backend := NewXBPS(command)

	packages, err := backend.ListInstalled(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	want := []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", Version: "5.3_2", Installed: true, Description: "GNU Bourne Again Shell"},
		{Ref: core.PackageRef{BackendID: "xbps", ID: "MesaLib-devel"}, Name: "MesaLib-devel", Version: "26.2.7_1", Installed: true, Description: "Open source graphics stack"},
	}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %#v, want %#v", packages, want)
	}
	if command.name != "xbps-query" || !reflect.DeepEqual(command.args, []string{"--list-pkgs"}) {
		t.Fatalf("command = %q %q", command.name, command.args)
	}
}

func TestXBPSListIgnoresIndentedDescriptionContinuations(t *testing.T) {
	output := []byte("[-] UMU-Launcher-1.2.3_1 Game launcher\n UMU-Launcher supports multiple stores\n")
	packages, err := parseXBPSList(output, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 1 || packages[0].Name != "UMU-Launcher" {
		t.Fatalf("packages = %#v", packages)
	}
}

func TestXBPSListInstalledIncludesNativePackageStates(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte("ii bash-5.3_2 Shell\nii curl-8.0_1 Client\n")}},
		{result: runner.Result{Stdout: []byte("bash-5.3_2\n")}},
		{result: runner.Result{Stdout: []byte("curl-8.0_1\n")}},
		{result: runner.Result{Stdout: []byte("bash-5.3_2\n")}},
	}}
	backend := NewXBPS(command)
	backend.packageStates = true

	packages, err := backend.ListInstalled(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !packages[0].Held || !packages[0].RepoLocked || packages[0].Orphaned {
		t.Fatalf("bash = %#v", packages[0])
	}
	if !packages[1].Orphaned || packages[1].Held || packages[1].RepoLocked {
		t.Fatalf("curl = %#v", packages[1])
	}
}

func TestXBPSListInstalledRejectsMalformedOutput(t *testing.T) {
	backend := NewXBPS(&stubRunner{result: runner.Result{Stdout: []byte("ii malformed\n")}})

	_, err := backend.ListInstalled(context.Background())
	if !errors.Is(err, ErrInvalidBackendOutput) {
		t.Fatalf("error = %v, want ErrInvalidBackendOutput", err)
	}
}

func TestXBPSListInstalledPreservesRunnerError(t *testing.T) {
	want := errors.New("query failed")
	backend := NewXBPS(&stubRunner{err: want})

	_, err := backend.ListInstalled(context.Background())
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped %v", err, want)
	}
}

func TestXBPSSearch(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte("[*] bash-5.3_2 GNU Bourne Again Shell\n[-] bash-devel-5.3_2 Development files\n")}}
	backend := NewXBPS(command)

	packages, err := backend.Search(context.Background(), "bash")
	if err != nil {
		t.Fatal(err)
	}

	want := []core.Package{
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", Version: "5.3_2", Installed: true, Description: "GNU Bourne Again Shell"},
		{Ref: core.PackageRef{BackendID: "xbps", ID: "bash-devel"}, Name: "bash-devel", Version: "5.3_2", Description: "Development files"},
	}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %#v, want %#v", packages, want)
	}
	if command.name != "xbps-query" || !reflect.DeepEqual(command.args, []string{"--repository", "--search", "bash"}) {
		t.Fatalf("command = %q %q", command.name, command.args)
	}
}

func TestXBPSListAvailable(t *testing.T) {
	backend := NewXBPS(repositoryRunner{})

	packages, err := backend.ListAvailable(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(packages) != 3 || packages[0].Name != "0ad" || packages[1].Name != "bash" || !packages[1].Installed || packages[2].Name != "UEFITool" || packages[2].Version != "A76_1" {
		t.Fatalf("unexpected packages: %#v", packages)
	}
	if packages[0].Origin != "https://repo.example/current" || packages[2].Origin != "https://repo.example/current/nonfree" {
		t.Fatalf("package origins = %#v", packages)
	}
}

func TestXBPSListUpdates(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte("bash-5.4_1 update x86_64 https://repo.example/current 1 1\nnew-dependency-1.0_1 install x86_64 https://repo.example/current 1 1\n")}}
	backend := NewXBPS(command)

	packages, err := backend.ListUpdates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []core.Package{{Ref: core.PackageRef{BackendID: "xbps", ID: "bash"}, Name: "bash", NewVersion: "5.4_1", Origin: "https://repo.example/current", Installed: true}}
	if !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages = %#v, want %#v", packages, want)
	}
	if command.name != "xbps-install" || !reflect.DeepEqual(command.args, []string{"-Mun"}) {
		t.Fatalf("command = %q %q", command.name, command.args)
	}
}

func TestXBPSFindOwnersUsesNativeOwnedByQuery(t *testing.T) {
	command := &stubRunner{result: runner.Result{Stdout: []byte("bash-5.3_2: /usr/bin/bash (regular file)\n")}}
	backend := NewXBPS(command)

	matches, err := backend.FindOwners(context.Background(), "/usr/bin/bash")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(matches, []string{"bash-5.3_2: /usr/bin/bash (regular file)"}) {
		t.Fatalf("matches = %#v", matches)
	}
	if command.name != "xbps-query" || !reflect.DeepEqual(command.args, []string{"--ownedby", "/usr/bin/bash"}) {
		t.Fatalf("command = %q %q", command.name, command.args)
	}
}

func TestXBPSGetInstalledPackageDetails(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte("architecture: x86_64\nfilename-size: 11MB\nhomepage: https://www.gnu.org/software/bash/bash.html\ninstall-date: 2026-08-07 00:35 +04\ninstalled_size: 36MB\nlicense: GPL-3.0-or-later\nmaintainer: Bash Team <bash@example.test>\npkgname: bash\npkgver: bash-5.3_2\nprovides:\n\tcmd:bash-5.3_2\nrepository: https://repo-default.voidlinux.org/current\nshlib-provides:\n\tlibbash.so.1\nshlib-requires:\n\tlibc.so.6\nshort_desc: GNU Bourne Again Shell\nsourcepkg: bash\nstate: installed\n")}},
		{result: runner.Result{Stdout: []byte("glibc>=2.41_1\n")}},
		{result: runner.Result{Stdout: []byte("bash-completion-2.16.0_1\n")}},
		{result: runner.Result{Stdout: []byte("/usr/bin/bash\n/usr/share/man/man1/bash.1\n")}},
	}}
	backend := NewXBPS(command)
	backend.iconRoots = nil

	details, err := backend.Get(context.Background(), core.PackageRef{BackendID: "xbps", ID: "bash"})
	if err != nil {
		t.Fatal(err)
	}

	want := core.PackageDetails{
		Package: core.Package{
			Ref:          core.PackageRef{BackendID: "xbps", ID: "bash"},
			Name:         "bash",
			Version:      "5.3_2",
			Architecture: "x86_64",
			Origin:       "https://repo-default.voidlinux.org/current",
			Installed:    true,
		},
		Description:         "GNU Bourne Again Shell",
		Homepage:            "https://www.gnu.org/software/bash/bash.html",
		License:             "GPL-3.0-or-later",
		DownloadSize:        "11MB",
		InstalledSize:       "36MB",
		InstallDate:         "2026-08-07 00:35 +04",
		Maintainer:          "Bash Team <bash@example.test>",
		SourcePackage:       "bash",
		Provides:            []string{"cmd:bash-5.3_2"},
		Requires:            []string{"libc.so.6"},
		Exports:             []string{"libbash.so.1"},
		Dependencies:        []string{"glibc>=2.41_1"},
		ReverseDependencies: []string{"bash-completion-2.16.0_1"},
		Files:               []string{"/usr/bin/bash", "/usr/share/man/man1/bash.1"},
	}
	if !reflect.DeepEqual(details, want) {
		t.Fatalf("details = %#v, want %#v", details, want)
	}
	if command.calls != 4 {
		t.Fatalf("calls = %d, want 4", command.calls)
	}
}

func TestXBPSGetTreatsMissingRelationsAndFilesAsEmpty(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{result: runner.Result{Stdout: []byte("pkgname: clang\npkgver: clang-21_3\nstate: installed\n")}},
		{err: exitError(2)},
		{err: exitError(2)},
		{err: exitError(2)},
	}}

	details, err := NewXBPS(command).Get(context.Background(), core.PackageRef{BackendID: "xbps", ID: "clang"})
	if err != nil {
		t.Fatal(err)
	}
	if len(details.Dependencies) != 0 || len(details.ReverseDependencies) != 0 || len(details.Files) != 0 {
		t.Fatalf("unexpected related details: %#v", details)
	}
}

func TestXBPSResolvesAllowlistedPackageIcon(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Papirus", "48x48", "apps")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "firefox.png"), []byte("png-data"), 0o644); err != nil {
		t.Fatal(err)
	}
	backend := NewXBPS(&stubRunner{})
	backend.iconRoots = []string{root}

	icon := backend.packageIcon("firefox", nil)
	if icon != "data:image/png;base64,cG5nLWRhdGE=" {
		t.Fatalf("icon = %q", icon)
	}
	if icon := backend.packageIcon("../../etc/passwd", nil); icon != "" {
		t.Fatalf("unsafe icon = %q", icon)
	}
}

func TestXBPSGetFallsBackToRepository(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{
		{err: exitError(2)},
		{result: runner.Result{Stdout: []byte("architecture: x86_64\npkgname: ripgrep\npkgver: ripgrep-15.1.0_1\nrepository: https://repo-default.voidlinux.org/current\nshort_desc: Fast search tool\n")}},
		{result: runner.Result{Stdout: []byte("pcre2>=10.45_1\n")}},
		{result: runner.Result{Stdout: []byte("vscode-1.0_1\n")}},
	}}
	backend := NewXBPS(command)

	details, err := backend.Get(context.Background(), core.PackageRef{BackendID: "xbps", ID: "ripgrep"})
	if err != nil {
		t.Fatal(err)
	}
	if details.Name != "ripgrep" || details.Version != "15.1.0_1" || details.Installed {
		t.Fatalf("unexpected details: %#v", details)
	}
	if !reflect.DeepEqual(details.Dependencies, []string{"pcre2>=10.45_1"}) || !reflect.DeepEqual(details.ReverseDependencies, []string{"vscode-1.0_1"}) || details.Files != nil {
		t.Fatalf("unexpected related details: %#v", details)
	}
	if command.calls != 4 {
		t.Fatalf("calls = %d, want 4", command.calls)
	}
}

func TestXBPSGetDoesNotHideCommandFailure(t *testing.T) {
	command := &sequenceRunner{replies: []runnerReply{{err: exitError(1)}}}
	backend := NewXBPS(command)

	_, err := backend.Get(context.Background(), core.PackageRef{BackendID: "xbps", ID: "bash"})
	if err == nil || command.calls != 1 {
		t.Fatalf("error = %v, calls = %d; want error after one call", err, command.calls)
	}
}

func TestXBPSGetRejectsMismatchedPackageDetails(t *testing.T) {
	backend := NewXBPS(&stubRunner{result: runner.Result{Stdout: []byte("pkgname: different\npkgver: different-1.0_1\n")}})

	_, err := backend.Get(context.Background(), core.PackageRef{BackendID: "xbps", ID: "requested"})
	if !errors.Is(err, ErrInvalidBackendOutput) {
		t.Fatalf("error = %v, want ErrInvalidBackendOutput", err)
	}
}
