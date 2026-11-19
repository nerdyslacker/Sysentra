package transactions

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type plannerRunner struct {
	name string
	args []string
	err  error
}

func (r *plannerRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	if name != xbpsQuery {
		r.name, r.args = name, args
	}
	if name == xbpsQuery {
		if len(args) == 1 && args[0] == "--list-pkgs" {
			return runner.Result{Stdout: []byte("ii firefox-129.0_1 Web browser\n")}, r.err
		}
		return runner.Result{Stdout: []byte("firefox-130.0_1\n")}, r.err
	}
	return runner.Result{Stdout: []byte("firefox-130.0_1 update x86_64 https://repo/current 80000000 90000000\n")}, r.err
}

type exitError int

func (e exitError) Error() string { return "command failed" }
func (e exitError) ExitCode() int { return int(e) }

type verificationRunner struct {
	result runner.Result
	err    error
}

type downgradeRunner struct {
	queryVersion string
	calls        []string
}

type kernelPurgeRunner struct{}

func (kernelPurgeRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	switch name {
	case vkpurge:
		return runner.Result{Stdout: []byte("6.6.70_1\n6.6.71_1\n")}, nil
	case "/usr/bin/uname":
		return runner.Result{Stdout: []byte("6.6.72_1\n")}, nil
	default:
		return runner.Result{}, errors.New("unexpected command")
	}
}

func (r *downgradeRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	switch name {
	case xbpsRindex:
		return runner.Result{Stdout: []byte("index: 1 package registered\n")}, nil
	case xbpsInstall:
		return runner.Result{Stdout: []byte("firefox-129.0_1 reinstall x86_64 /tmp/repository 1000 500\n")}, nil
	case xbpsQuery:
		return runner.Result{Stdout: []byte("firefox-" + r.queryVersion + "\n")}, nil
	default:
		return runner.Result{}, errors.New("unexpected command")
	}
}

func (r verificationRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	return r.result, r.err
}

func TestXBPSExecutorRejectsStalePlanBeforeRunningCommand(t *testing.T) {
	command := &plannerRunner{}
	now := time.Unix(100, 0)
	executor := &XBPSExecutor{
		runner:      command,
		fingerprint: func() (string, error) { return "state-b", nil },
		now:         func() time.Time { return now },
		lockPath:    t.TempDir() + "/xbps.lock",
	}
	plan := Plan{
		Request:          security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Install, Packages: []string{"firefox"}},
		StateFingerprint: "state-a",
		ExpiresAt:        now.Add(time.Minute),
	}
	if _, err := executor.Commit(context.Background(), plan); err == nil || command.name != "" {
		t.Fatalf("stale plan reached command runner: command=%q err=%v", command.name, err)
	}
}

func TestXBPSExecutorRunsAllowlistedCommand(t *testing.T) {
	command := &plannerRunner{}
	now := time.Unix(100, 0)
	executor := &XBPSExecutor{
		runner:      command,
		fingerprint: func() (string, error) { return "state-a", nil },
		now:         func() time.Time { return now },
		lockPath:    t.TempDir() + "/xbps.lock",
	}
	plan := Plan{
		Request:          security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Reinstall, Packages: []string{"firefox"}},
		StateFingerprint: "state-a",
		ExpiresAt:        now.Add(time.Minute),
	}
	result, err := executor.Commit(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if command.name != "/usr/bin/xbps-install" || !reflect.DeepEqual(command.args, []string{"--yes", "--force", "firefox"}) {
		t.Fatalf("command = %q %#v", command.name, command.args)
	}
	if result.RequestID != "request-1234" || result.Operation != security.Reinstall {
		t.Fatalf("result = %#v", result)
	}
}

func TestXBPSPlansAndExecutesVerifiedCachedDowngrade(t *testing.T) {
	cache := t.TempDir()
	archive := filepath.Join(cache, "firefox-129.0_1.x86_64.xbps")
	if err := os.WriteFile(archive, []byte("package archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive+".sig2", []byte("signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	plannerRunner := &downgradeRunner{queryVersion: "130.0_1"}
	planner := &XBPSPlanner{runner: plannerRunner, fingerprint: func() (string, error) { return "state-a", nil }, now: func() time.Time { return now }, cacheDir: cache}
	request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Downgrade, Packages: []string{"firefox"}, TargetVersion: "129.0_1"}
	plan, err := planner.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes) != 1 || plan.Changes[0].Action != "downgrade" || plan.Changes[0].FromVersion != "130.0_1" || plan.Changes[0].ToVersion != "129.0_1" || plan.Changes[0].ArtifactSHA256 == "" {
		t.Fatalf("plan = %#v", plan)
	}
	if plan.DownloadSize != 0 {
		t.Fatalf("download size = %d, want cached archive excluded", plan.DownloadSize)
	}

	command := &downgradeRunner{queryVersion: "129.0_1"}
	executor := &XBPSExecutor{
		runner:      command,
		fingerprint: func() (string, error) { return "state-a", nil },
		now:         func() time.Time { return now },
		lockPath:    t.TempDir() + "/xbps.lock",
		cacheDir:    cache,
	}
	result, err := executor.Commit(context.Background(), plan)
	if err != nil || result.Operation != security.Downgrade {
		t.Fatalf("result=%#v err=%v calls=%#v", result, err, command.calls)
	}
}

func TestXBPSDowngradeRejectsArchiveChangedAfterPreview(t *testing.T) {
	cache := t.TempDir()
	archive := filepath.Join(cache, "firefox-129.0_1.x86_64.xbps")
	if err := os.WriteFile(archive, []byte("reviewed archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive+".sig2", []byte("signature"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	planner := &XBPSPlanner{runner: &downgradeRunner{queryVersion: "130.0_1"}, fingerprint: func() (string, error) { return "state-a", nil }, now: func() time.Time { return now }, cacheDir: cache}
	plan, err := planner.Plan(context.Background(), security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Downgrade, Packages: []string{"firefox"}, TargetVersion: "129.0_1"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, []byte("replaced archive"), 0o600); err != nil {
		t.Fatal(err)
	}
	command := &downgradeRunner{queryVersion: "129.0_1"}
	executor := &XBPSExecutor{runner: command, fingerprint: func() (string, error) { return "state-a", nil }, now: func() time.Time { return now }, lockPath: t.TempDir() + "/xbps.lock", cacheDir: cache}
	if _, err := executor.Commit(context.Background(), plan); err == nil || len(command.calls) != 0 {
		t.Fatalf("changed archive reached runner: calls=%#v err=%v", command.calls, err)
	}
}

func TestVerifyPackageResultRejectsWrongInstalledVersion(t *testing.T) {
	plan := Plan{
		Request: security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Install, Packages: []string{"firefox"}},
		Changes: []Change{{Package: "firefox", Action: "install", ToVersion: "131.0_1"}},
	}
	err := verifyPackageResult(context.Background(), verificationRunner{result: runner.Result{Stdout: []byte("firefox-130.0_1\n")}}, plan)
	if err == nil {
		t.Fatal("expected installed version mismatch")
	}
}

func TestVerifyPackageResultAcceptsRemovedPackage(t *testing.T) {
	plan := Plan{Request: security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Remove, Packages: []string{"firefox"}}}
	if err := verifyPackageResult(context.Background(), verificationRunner{err: exitError(2)}, plan); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPackageResultChecksHoldState(t *testing.T) {
	plan := Plan{Request: security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Hold, Packages: []string{"firefox"}}}
	if err := verifyPackageResult(context.Background(), verificationRunner{result: runner.Result{Stdout: []byte("firefox-130.0_1\n")}}, plan); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyPackageResultChecksWholeSystemUpgradeFromOneInventory(t *testing.T) {
	plan := Plan{
		Request: security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.SystemUpgrade},
		Changes: []Change{{Package: "firefox", Action: "update", ToVersion: "131.0_1"}, {Package: "obsolete", Action: "remove"}},
	}
	result := runner.Result{Stdout: []byte("ii firefox-131.0_1 Web browser\nii bash-5.3_2 Shell\n")}
	if err := verifyPackageResult(context.Background(), verificationRunner{result: result}, plan); err != nil {
		t.Fatal(err)
	}
}

func TestXBPSPlannerUsesNativeDryRunAndBindsState(t *testing.T) {
	command := &plannerRunner{}
	planner := &XBPSPlanner{runner: command, fingerprint: func() (string, error) { return "state-a", nil }, now: func() time.Time { return time.Unix(100, 0) }}
	request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Update, Packages: []string{"firefox"}}
	plan, err := planner.Plan(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if command.name != "/usr/bin/xbps-install" || !reflect.DeepEqual(command.args, []string{"--dry-run", "--yes", "--update", "firefox"}) {
		t.Fatalf("command = %q %#v", command.name, command.args)
	}
	if plan.StateFingerprint != "state-a" || len(plan.Diagnostics) != 1 || !plan.ExpiresAt.Equal(time.Unix(220, 0)) || plan.Changes[0].FromVersion != "129.0_1" {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestXBPSPlannerDoesNotExecuteForPackageModeChange(t *testing.T) {
	command := &plannerRunner{}
	planner := &XBPSPlanner{runner: command, fingerprint: func() (string, error) { return "state-a", nil }, now: time.Now}
	request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Hold, Packages: []string{"firefox"}}
	plan, err := planner.Plan(context.Background(), request)
	if err != nil || command.name != "" || len(plan.Changes) != 1 || plan.Changes[0].Action != "hold" {
		t.Fatalf("plan = %#v, command = %q, err = %v", plan, command.name, err)
	}
}

func TestXBPSPurgeUsesRecursiveRemoveForPreviewAndExecution(t *testing.T) {
	request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Purge, Packages: []string{"firefox"}}
	name, args, native := previewCommand(request)
	if !native || name != "/usr/bin/xbps-remove" || !reflect.DeepEqual(args, []string{"--dry-run", "--yes", "--recursive", "firefox"}) {
		t.Fatalf("preview = %q %#v native=%v", name, args, native)
	}
	name, args, err := executionCommand(request)
	if err != nil || name != "/usr/bin/xbps-remove" || !reflect.DeepEqual(args, []string{"--yes", "--recursive", "firefox"}) {
		t.Fatalf("execution = %q %#v err=%v", name, args, err)
	}
}

func TestXBPSDatabaseSyncUsesAllowlistedExecutable(t *testing.T) {
	request := security.PackageRequest{Version: 1, RequestID: "sync-db-1234", Operation: security.SyncDatabase}
	name, args, native := previewCommand(request)
	if native || name != "" || args != nil {
		t.Fatalf("preview = %q %#v native=%v", name, args, native)
	}
	name, args, err := executionCommand(request)
	if err != nil || name != "/usr/bin/xbps-install" || !reflect.DeepEqual(args, []string{"--sync"}) {
		t.Fatalf("execution = %q %#v err=%v", name, args, err)
	}
}

func TestXBPSSystemUpgradeUsesCurrentRepositoryMetadata(t *testing.T) {
	request := security.PackageRequest{Version: 1, RequestID: "upgrade-1234", Operation: security.SystemUpgrade}
	name, args, native := previewCommand(request)
	if !native || name != "/usr/bin/xbps-install" || !reflect.DeepEqual(args, []string{"--memory-sync", "--dry-run", "--yes", "--update"}) {
		t.Fatalf("preview = %q %#v native=%v", name, args, native)
	}
	name, args, err := executionCommand(request)
	if err != nil || name != "/usr/bin/xbps-install" || !reflect.DeepEqual(args, []string{"--memory-sync", "--yes", "--update"}) {
		t.Fatalf("execution = %q %#v err=%v", name, args, err)
	}
}

func TestXBPSMaintenanceUsesFiniteCommands(t *testing.T) {
	tests := []struct {
		operation security.Operation
		packages  []string
		preview   []string
		execute   []string
		native    bool
		command   string
	}{
		{security.RemoveOrphans, nil, []string{"--dry-run", "--yes", "--remove-orphans"}, []string{"--yes", "--remove-orphans"}, true, xbpsRemove},
		{security.ForceRemove, []string{"firefox"}, []string{"--dry-run", "--yes", "--force-revdeps", "firefox"}, []string{"--yes", "--force-revdeps", "firefox"}, true, xbpsRemove},
		{security.CleanCache, nil, nil, []string{"--yes", "--clean-cache"}, false, xbpsRemove},
		{security.VerifyDatabase, nil, nil, []string{"--all"}, false, xbpsPkgDB},
		{security.ReconfigureAll, nil, nil, []string{"--force", "--all"}, false, xbpsReconfigure},
		{security.PurgeKernels, []string{"6.12.10_1"}, nil, []string{"rm", "6.12.10_1"}, false, vkpurge},
	}
	for _, test := range tests {
		request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: test.operation, Packages: test.packages}
		name, args, native := previewCommand(request)
		if native != test.native || !reflect.DeepEqual(args, test.preview) || (native && name != test.command) {
			t.Fatalf("%s preview = %q %#v native=%v", test.operation, name, args, native)
		}
		name, args, err := executionCommand(request)
		if err != nil || name != test.command || !reflect.DeepEqual(args, test.execute) {
			t.Fatalf("%s execution = %q %#v err=%v", test.operation, name, args, err)
		}
	}
}

func TestKernelPurgeAcceptsEveryVersionReportedAsOld(t *testing.T) {
	if err := validateKernelPurge(context.Background(), kernelPurgeRunner{}, []string{"6.6.70_1"}); err != nil {
		t.Fatal(err)
	}
	if err := validateKernelPurge(context.Background(), kernelPurgeRunner{}, []string{"6.6.70_1", "6.6.71_1"}); err != nil {
		t.Fatalf("purge rejected old kernels: %v", err)
	}
	if err := validateKernelPurge(context.Background(), kernelPurgeRunner{}, []string{"6.6.69_1"}); err == nil {
		t.Fatal("purge accepted a kernel not reported by vkpurge")
	}
}

func TestKernelPackageRemovalProtectsRunningKernelAndFallback(t *testing.T) {
	installed := map[string][]string{"linux6.12": {"6.12.10_1"}, "linux6.6": {"6.6.72_1"}, "aurora-kernel": {"6.1.90-custom"}}
	if err := validateKernelRemovalState([]string{"linux6.12"}, installed, "6.12.10_1"); err == nil {
		t.Fatal("running kernel package removal was accepted")
	}
	if err := validateKernelRemovalState([]string{"aurora-kernel"}, installed, "6.12.10_1"); err != nil {
		t.Fatalf("safe old series removal rejected: %v", err)
	}
	if err := validateKernelRemovalState([]string{"linux6.6"}, map[string][]string{"linux6.12": {"6.12.10_1"}, "linux6.6": {"6.6.72_1"}}, "6.12.10_1"); err == nil {
		t.Fatal("last fallback removal was accepted")
	}
	if err := validateKernelRemovalState([]string{"firefox"}, map[string][]string{"linux6.12": {"6.12.10_1"}}, "6.12.10_1"); err != nil {
		t.Fatalf("non-kernel removal rejected: %v", err)
	}
}

func TestParseKernelOwnersUsesBootArtifactsInsteadOfPackageNames(t *testing.T) {
	output := []byte("malformed\naurora-kernel-6.14.2_3: /boot/vmlinuz-6.14.2-aurora (regular file)\nlinux6.12-6.12.10_1: /boot/vmlinuz-6.12.10_1 (regular file)\n")
	got := parseKernelOwners(output)
	if len(got) != 2 || got["aurora-kernel"][0] != "6.14.2-aurora" {
		t.Fatalf("owners=%#v", got)
	}
}

func TestXBPSReconfigureUsesAllowlistedExecutable(t *testing.T) {
	request := security.PackageRequest{Version: 1, RequestID: "request-1234", Operation: security.Reconfigure, Packages: []string{"linux7.3"}}
	name, args, native := previewCommand(request)
	if native || name != "" || args != nil {
		t.Fatalf("preview = %q %#v native=%v", name, args, native)
	}
	name, args, err := executionCommand(request)
	if err != nil || name != "/usr/bin/xbps-reconfigure" || !reflect.DeepEqual(args, []string{"-f", "linux7.3"}) {
		t.Fatalf("execution = %q %#v err=%v", name, args, err)
	}
}

func TestParseNativeChangesBuildsRowsAndTotals(t *testing.T) {
	lines := []string{
		"0ad-data-0.28.0_1 install x86_64 https://repo/current 3602349060 1712633917",
		"libenet-1.3.18_1 install x86_64 https://repo/current 52276 27015",
		"warning that is not a package row",
	}
	changes, download, installed := parseNativeChanges(lines)
	if len(changes) != 2 || changes[0].Package != "0ad-data" || changes[0].ToVersion != "0.28.0_1" || changes[0].FromVersion != "" {
		t.Fatalf("changes = %#v", changes)
	}
	if download != 1712660932 || installed != 3602401336 {
		t.Fatalf("totals = download %d installed %d", download, installed)
	}
}

func TestParseNativeChangesUsesInstalledVersionForRemoval(t *testing.T) {
	changes, _, _ := parseNativeChanges([]string{"SPIRV-LLVM-Translator21-21.1.12_1 remove x86_64 https://repo/current 6828273 1743403"})
	if len(changes) != 1 || changes[0].Package != "SPIRV-LLVM-Translator21" || changes[0].FromVersion != "21.1.12_1" || changes[0].ToVersion != "" {
		t.Fatalf("changes = %#v", changes)
	}
}
