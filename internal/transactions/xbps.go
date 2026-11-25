package transactions

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

const maxDiagnosticLines = 500

const (
	xbpsInstall     = "/usr/bin/xbps-install"
	xbpsRemove      = "/usr/bin/xbps-remove"
	xbpsPkgDB       = "/usr/bin/xbps-pkgdb"
	xbpsQuery       = "/usr/bin/xbps-query"
	xbpsReconfigure = "/usr/bin/xbps-reconfigure"
	xbpsRindex      = "/usr/bin/xbps-rindex"
	vkpurge         = "/usr/bin/vkpurge"
	xbpsCache       = "/var/cache/xbps"
)

type XBPSPlanner struct {
	runner      runner.Runner
	fingerprint func() (string, error)
	now         func() time.Time
	cacheDir    string
}

type Result struct {
	RequestID   string             `json:"requestId"`
	Operation   security.Operation `json:"operation"`
	Diagnostics []string           `json:"diagnostics"`
}

type XBPSExecutor struct {
	runner      runner.Runner
	fingerprint func() (string, error)
	now         func() time.Time
	lockPath    string
	cacheDir    string
}

func NewSystemXBPSPlanner(commandRunner runner.Runner) *XBPSPlanner {
	return &XBPSPlanner{runner: commandRunner, fingerprint: systemPackageFingerprint, now: time.Now, cacheDir: xbpsCache}
}

func NewSystemXBPSExecutor(commandRunner runner.Runner) *XBPSExecutor {
	return &XBPSExecutor{runner: commandRunner, fingerprint: systemPackageFingerprint, now: time.Now, lockPath: "/run/lock/sysentra-xbps.lock", cacheDir: xbpsCache}
}

func (e *XBPSExecutor) Commit(ctx context.Context, plan Plan) (Result, error) {
	lock, err := acquireLock(ctx, e.lockPath)
	if err != nil {
		return Result{}, err
	}
	defer func() {
		_ = unix.Flock(int(lock.Fd()), unix.LOCK_UN)
		_ = lock.Close()
	}()

	fingerprint, err := e.fingerprint()
	if err != nil {
		return Result{}, fmt.Errorf("fingerprint package state: %w", err)
	}
	if err := plan.ValidateCommit(e.now(), fingerprint); err != nil {
		return Result{}, err
	}
	if plan.Request.Operation == security.Remove || plan.Request.Operation == security.Purge || plan.Request.Operation == security.ForceRemove {
		if err := validateKernelPackageRemoval(ctx, e.runner, plan.Request.Packages); err != nil {
			return Result{}, err
		}
	}
	if plan.Request.Operation == security.RemoveDriver {
		if err := validateDriverPackageRemoval(ctx, e.runner, plan.Request.Packages, plan.Request.DriverModules); err != nil {
			return Result{}, err
		}
	}
	if plan.Request.Operation == security.Downgrade {
		return e.commitDowngrade(ctx, plan)
	}
	if plan.Request.Operation == security.PurgeKernels {
		if err := validateKernelPurge(ctx, e.runner, plan.Request.Packages); err != nil {
			return Result{}, err
		}
	}
	name, args, err := executionCommand(plan.Request)
	if err != nil {
		return Result{}, err
	}
	commandResult, err := e.runner.Run(ctx, name, args...)
	if err != nil {
		return Result{}, fmt.Errorf("execute %s: %w: %s", plan.Request.Operation, err, strings.TrimSpace(string(commandResult.Stderr)))
	}
	if err := verifyPackageResult(ctx, e.runner, plan); err != nil {
		return Result{}, fmt.Errorf("verify %s: %w", plan.Request.Operation, err)
	}
	return Result{RequestID: plan.Request.RequestID, Operation: plan.Request.Operation, Diagnostics: boundedLines(commandResult.Stdout, commandResult.Stderr)}, nil
}

func (e *XBPSExecutor) commitDowngrade(ctx context.Context, plan Plan) (Result, error) {
	request := plan.Request
	var target Change
	for _, change := range plan.Changes {
		if change.Package == request.Packages[0] {
			target = change
			break
		}
	}
	archive, digest, err := cachedArchive(e.cacheDir, request.Packages[0], request.TargetVersion)
	if err != nil {
		return Result{}, err
	}
	if target.ArtifactPath != archive || target.ArtifactSHA256 != digest {
		return Result{}, errors.New("cached package archive changed; create a new plan")
	}
	repository, stagedArchive, cleanup, err := stageCachedArchive(archive)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()
	indexResult, err := e.runner.Run(ctx, xbpsRindex, "--add", stagedArchive)
	if err != nil {
		return Result{}, fmt.Errorf("index cached package: %w: %s", err, strings.TrimSpace(string(indexResult.Stderr)))
	}
	result, err := e.runner.Run(ctx, xbpsInstall, "--yes", "--repository", repository, "--force", request.Packages[0]+"-"+request.TargetVersion)
	if err != nil {
		return Result{}, fmt.Errorf("execute downgrade: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	if err := verifyInstalledPackage(ctx, e.runner, request.Packages[0], request.TargetVersion); err != nil {
		return Result{}, fmt.Errorf("verify downgrade: %w", err)
	}
	return Result{RequestID: request.RequestID, Operation: request.Operation, Diagnostics: boundedLines(indexResult.Stdout, indexResult.Stderr, result.Stdout, result.Stderr)}, nil
}

func (p *XBPSPlanner) planDowngrade(ctx context.Context, plan Plan) (Plan, error) {
	request := plan.Request
	archive, digest, err := cachedArchive(p.cacheDir, request.Packages[0], request.TargetVersion)
	if err != nil {
		return Plan{}, err
	}
	repository, stagedArchive, cleanup, err := stageCachedArchive(archive)
	if err != nil {
		return Plan{}, err
	}
	defer cleanup()
	indexResult, err := p.runner.Run(ctx, xbpsRindex, "--add", stagedArchive)
	if err != nil {
		return Plan{}, fmt.Errorf("index cached package: %w: %s", err, strings.TrimSpace(string(indexResult.Stderr)))
	}
	result, err := p.runner.Run(ctx, xbpsInstall, "--dry-run", "--yes", "--repository", repository, "--force", request.Packages[0]+"-"+request.TargetVersion)
	if err != nil {
		return Plan{}, fmt.Errorf("preview downgrade: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	plan.Diagnostics = boundedLines(result.Stdout, result.Stderr)
	plan.Changes, plan.DownloadSize, plan.InstalledSize = parseNativeChanges(plan.Diagnostics)
	installed, err := p.runner.Run(ctx, xbpsQuery, "--property", "pkgver", request.Packages[0])
	if err != nil {
		return Plan{}, fmt.Errorf("query installed package version: %w", err)
	}
	_, currentVersion, ok := splitPackageVersion(strings.TrimSpace(string(installed.Stdout)))
	if !ok {
		return Plan{}, errors.New("installed package returned an invalid version")
	}
	found := false
	for index := range plan.Changes {
		if plan.Changes[index].Package == request.Packages[0] {
			plan.DownloadSize -= plan.Changes[index].DownloadSize
			if plan.DownloadSize < 0 {
				plan.DownloadSize = 0
			}
			plan.Changes[index].Action = string(security.Downgrade)
			plan.Changes[index].FromVersion = currentVersion
			plan.Changes[index].ToVersion = request.TargetVersion
			plan.Changes[index].DownloadSize = 0
			plan.Changes[index].ArtifactPath = archive
			plan.Changes[index].ArtifactSHA256 = digest
			found = true
		}
	}
	if !found {
		return Plan{}, errors.New("cached package was not present in the native downgrade preview")
	}
	plan.AvailableSpace = availableSpace("/")
	return plan, nil
}

func cachedArchive(cacheDir, packageName, version string) (string, string, error) {
	paths, err := filepath.Glob(filepath.Join(cacheDir, packageName+"-"+version+".*.xbps"))
	if err != nil || len(paths) != 1 {
		return "", "", errors.New("exactly one cached package archive is required")
	}
	archive := paths[0]
	info, err := os.Lstat(archive)
	if err != nil || !info.Mode().IsRegular() {
		return "", "", errors.New("cached package archive is not a regular file")
	}
	signature := archive + ".sig2"
	if info, err = os.Lstat(signature); err != nil || !info.Mode().IsRegular() {
		return "", "", errors.New("cached package signature is missing")
	}
	file, err := os.Open(archive)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", "", err
	}
	return archive, hex.EncodeToString(hash.Sum(nil)), nil
}

func stageCachedArchive(archive string) (string, string, func(), error) {
	repository, err := os.MkdirTemp("", "sysentra-downgrade-")
	if err != nil {
		return "", "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(repository) }
	stagedArchive := filepath.Join(repository, filepath.Base(archive))
	if err := os.Symlink(archive, stagedArchive); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	if err := os.Symlink(archive+".sig2", stagedArchive+".sig2"); err != nil {
		cleanup()
		return "", "", func() {}, err
	}
	return repository, stagedArchive, cleanup, nil
}

func verifyPackageResult(ctx context.Context, commandRunner runner.Runner, plan Plan) error {
	request := plan.Request
	switch request.Operation {
	case security.SystemUpgrade:
		return verifyInstalledChanges(ctx, commandRunner, plan.Changes)
	case security.SyncDatabase:
		if _, err := commandRunner.Run(ctx, xbpsQuery, "--list-repos"); err != nil {
			return fmt.Errorf("read synchronized repositories: %w", err)
		}
	case security.Install, security.Update, security.Reinstall, security.Reconfigure:
		expected := make(map[string]string, len(plan.Changes))
		for _, change := range plan.Changes {
			if change.ToVersion != "" {
				expected[change.Package] = change.ToVersion
			}
		}
		for _, name := range request.Packages {
			if err := verifyInstalledPackage(ctx, commandRunner, name, expected[name]); err != nil {
				return err
			}
		}
	case security.Remove, security.Purge, security.RemoveDriver:
		for _, name := range request.Packages {
			if err := verifyRemovedPackage(ctx, commandRunner, name); err != nil {
				return err
			}
		}
	case security.ForceRemove:
		for _, name := range request.Packages {
			if err := verifyRemovedPackage(ctx, commandRunner, name); err != nil {
				return err
			}
		}
	case security.RemoveOrphans:
		return verifyInstalledChanges(ctx, commandRunner, plan.Changes)
	case security.PurgeKernels:
		result, err := commandRunner.Run(ctx, vkpurge, "list")
		if err != nil {
			return fmt.Errorf("list old kernels after purge: %w", err)
		}
		remaining := listedValues(result.Stdout)
		for _, version := range request.Packages {
			if remaining[version] {
				return fmt.Errorf("kernel %s remains after vkpurge", version)
			}
		}
	case security.Hold, security.Unhold, security.RepoLock, security.RepoUnlock:
		flag := "--list-hold-pkgs"
		want := request.Operation == security.Hold
		if request.Operation == security.RepoLock || request.Operation == security.RepoUnlock {
			flag, want = "--list-repolock-pkgs", request.Operation == security.RepoLock
		}
		result, err := commandRunner.Run(ctx, xbpsQuery, flag)
		if err != nil {
			return fmt.Errorf("query package mode: %w", err)
		}
		listed := listedPackages(result.Stdout)
		for _, name := range request.Packages {
			if listed[name] != want {
				return fmt.Errorf("package %s did not reach requested mode", name)
			}
		}
	}
	return nil
}

func verifyRemovedPackage(ctx context.Context, commandRunner runner.Runner, name string) error {
	_, err := commandRunner.Run(ctx, xbpsQuery, "--property", "pkgver", name)
	if err == nil {
		return fmt.Errorf("package %s is still installed", name)
	}
	var exitError interface{ ExitCode() int }
	if !errors.As(err, &exitError) || exitError.ExitCode() != 2 {
		return fmt.Errorf("query removed package %s: %w", name, err)
	}
	return nil
}

func verifyInstalledChanges(ctx context.Context, commandRunner runner.Runner, changes []Change) error {
	result, err := commandRunner.Run(ctx, xbpsQuery, "--list-pkgs")
	if err != nil {
		return fmt.Errorf("list installed packages: %w", err)
	}
	installed := installedPackageVersions(result.Stdout)
	for _, change := range changes {
		if change.Action == "remove" {
			if _, found := installed[change.Package]; found {
				return fmt.Errorf("package %s is still installed", change.Package)
			}
			continue
		}
		version, found := installed[change.Package]
		if !found {
			return fmt.Errorf("package %s is not installed", change.Package)
		}
		if change.ToVersion != "" && version != change.ToVersion {
			return fmt.Errorf("package %s has version %s, expected %s", change.Package, version, change.ToVersion)
		}
	}
	return nil
}

func installedPackageVersions(output []byte) map[string]string {
	installed := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ii" {
			continue
		}
		if name, version, ok := splitPackageVersion(fields[1]); ok {
			installed[name] = version
		}
	}
	return installed
}

func verifyInstalledPackage(ctx context.Context, commandRunner runner.Runner, name, expectedVersion string) error {
	result, err := commandRunner.Run(ctx, xbpsQuery, "--property", "pkgver", name)
	if err != nil {
		return fmt.Errorf("query installed package %s: %w", name, err)
	}
	installedName, version, ok := splitPackageVersion(strings.TrimSpace(string(result.Stdout)))
	if !ok || installedName != name {
		return fmt.Errorf("package %s returned invalid installed version", name)
	}
	if expectedVersion != "" && version != expectedVersion {
		return fmt.Errorf("package %s has version %s, expected %s", name, version, expectedVersion)
	}
	return nil
}

func listedPackages(output []byte) map[string]bool {
	listed := make(map[string]bool)
	for _, line := range strings.Split(string(output), "\n") {
		if name, _, ok := splitPackageVersion(strings.TrimSpace(line)); ok {
			listed[name] = true
		}
	}
	return listed
}

func acquireLock(ctx context.Context, path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open XBPS lock: %w", err)
	}
	for {
		if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return file, nil
		} else if !errors.Is(err, unix.EWOULDBLOCK) {
			_ = file.Close()
			return nil, fmt.Errorf("lock XBPS: %w", err)
		}
		select {
		case <-ctx.Done():
			_ = file.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func executionCommand(request security.PackageRequest) (string, []string, error) {
	switch request.Operation {
	case security.Install:
		return xbpsInstall, append([]string{"--yes"}, request.Packages...), nil
	case security.Reinstall:
		return xbpsInstall, append([]string{"--yes", "--force"}, request.Packages...), nil
	case security.Reconfigure:
		return xbpsReconfigure, append([]string{"-f"}, request.Packages...), nil
	case security.Update:
		return xbpsInstall, append([]string{"--yes", "--update"}, request.Packages...), nil
	case security.SystemUpgrade:
		return xbpsInstall, []string{"--memory-sync", "--yes", "--update"}, nil
	case security.SyncDatabase:
		return xbpsInstall, []string{"--sync"}, nil
	case security.Remove, security.RemoveDriver:
		return xbpsRemove, append([]string{"--yes"}, request.Packages...), nil
	case security.Purge:
		return xbpsRemove, append([]string{"--yes", "--recursive"}, request.Packages...), nil
	case security.RemoveOrphans:
		return xbpsRemove, []string{"--yes", "--remove-orphans"}, nil
	case security.ForceRemove:
		return xbpsRemove, append([]string{"--yes", "--force-revdeps"}, request.Packages...), nil
	case security.CleanCache:
		return xbpsRemove, []string{"--yes", "--clean-cache"}, nil
	case security.VerifyDatabase:
		return xbpsPkgDB, []string{"--all"}, nil
	case security.ReconfigureAll:
		return xbpsReconfigure, []string{"--force", "--all"}, nil
	case security.PurgeKernels:
		return vkpurge, append([]string{"rm"}, request.Packages...), nil
	case security.Hold, security.Unhold, security.RepoLock, security.RepoUnlock:
		return xbpsPkgDB, append([]string{"--mode", string(request.Operation)}, request.Packages...), nil
	default:
		return "", nil, fmt.Errorf("execution is not supported for %s", request.Operation)
	}
}

func (p *XBPSPlanner) Plan(ctx context.Context, request security.PackageRequest) (Plan, error) {
	if err := request.Validate(); err != nil {
		return Plan{}, err
	}
	if request.Operation == security.Remove || request.Operation == security.Purge || request.Operation == security.ForceRemove {
		if err := validateKernelPackageRemoval(ctx, p.runner, request.Packages); err != nil {
			return Plan{}, err
		}
	}
	if request.Operation == security.RemoveDriver {
		if err := validateDriverPackageRemoval(ctx, p.runner, request.Packages, request.DriverModules); err != nil {
			return Plan{}, err
		}
	}
	fingerprint, err := p.fingerprint()
	if err != nil {
		return Plan{}, fmt.Errorf("fingerprint package state: %w", err)
	}
	plan := Plan{Request: request, StateFingerprint: fingerprint, ExpiresAt: p.now().Add(2 * time.Minute)}
	if request.Operation == security.Downgrade {
		return p.planDowngrade(ctx, plan)
	}
	if request.Operation == security.PurgeKernels {
		if err := validateKernelPurge(ctx, p.runner, request.Packages); err != nil {
			return Plan{}, err
		}
		plan.Changes = make([]Change, len(request.Packages))
		for index, version := range request.Packages {
			plan.Changes[index] = Change{Package: "kernel " + version, Action: "remove", FromVersion: version}
			plan.InstalledSize += bootArtifactSize(version)
		}
		plan.AvailableSpace = availableSpace("/boot")
		plan.Diagnostics = []string{"vkpurge will run Void kernel pre/post-remove hooks and update the boot menu."}
		return plan, nil
	}
	name, args, native := previewCommand(request)
	if !native {
		switch request.Operation {
		case security.SyncDatabase:
			plan.Diagnostics = []string{"Refresh XBPS repository indexes."}
		case security.CleanCache:
			plan.Diagnostics = []string{"Remove obsolete package archives from the XBPS cache. XBPS does not provide a non-mutating cache cleanup preview."}
		case security.VerifyDatabase:
			plan.Diagnostics = []string{"Check files, dependencies, alternatives, and package database records for every installed package."}
		case security.ReconfigureAll:
			plan.Diagnostics = []string{"Force post-installation configuration for every installed package."}
		default:
			plan.Changes = make([]Change, len(request.Packages))
			for index, packageName := range request.Packages {
				plan.Changes[index] = Change{Package: packageName, Action: string(request.Operation)}
			}
		}
		return plan, nil
	}
	result, err := p.runner.Run(ctx, name, args...)
	if err != nil {
		return Plan{}, fmt.Errorf("preview %s: %w: %s", request.Operation, err, strings.TrimSpace(string(result.Stderr)))
	}
	plan.Diagnostics = boundedLines(result.Stdout, result.Stderr)
	plan.Changes, plan.DownloadSize, plan.InstalledSize = parseNativeChanges(plan.Diagnostics)
	for _, change := range plan.Changes {
		if change.Action != "update" {
			continue
		}
		result, err := p.runner.Run(ctx, xbpsQuery, "--list-pkgs")
		if err != nil {
			return Plan{}, fmt.Errorf("list installed package versions: %w", err)
		}
		installed := installedPackageVersions(result.Stdout)
		for index := range plan.Changes {
			if plan.Changes[index].Action == "update" {
				plan.Changes[index].FromVersion = installed[plan.Changes[index].Package]
			}
		}
		break
	}
	plan.AvailableSpace = availableSpace("/")
	return plan, nil
}

func parseNativeChanges(lines []string) ([]Change, int64, int64) {
	changes := make([]Change, 0, len(lines))
	var downloadTotal, installedTotal int64
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) != 6 {
			continue
		}
		name, version, ok := splitPackageVersion(fields[0])
		installedSize, installedErr := strconv.ParseInt(fields[4], 10, 64)
		downloadSize, downloadErr := strconv.ParseInt(fields[5], 10, 64)
		if !ok || installedErr != nil || downloadErr != nil || installedSize < 0 || downloadSize < 0 {
			continue
		}
		change := Change{Package: name, Action: fields[1], InstalledSize: installedSize, DownloadSize: downloadSize}
		switch fields[1] {
		case "install":
			change.ToVersion = version
		case "remove":
			change.FromVersion = version
		case "reinstall":
			change.FromVersion, change.ToVersion = version, version
		default:
			change.ToVersion = version
		}
		changes = append(changes, change)
		downloadTotal += downloadSize
		installedTotal += installedSize
	}
	return changes, downloadTotal, installedTotal
}

func splitPackageVersion(pkgver string) (string, string, bool) {
	separator := strings.LastIndexByte(pkgver, '-')
	if separator < 1 || separator == len(pkgver)-1 {
		return "", "", false
	}
	version := pkgver[separator+1:]
	revision := strings.LastIndexByte(version, '_')
	if revision < 1 || revision == len(version)-1 {
		return "", "", false
	}
	if _, err := strconv.ParseUint(version[revision+1:], 10, 32); err != nil {
		return "", "", false
	}
	return pkgver[:separator], version, true
}

func validateDriverPackageRemoval(ctx context.Context, commandRunner runner.Runner, packages, modules []string) error {
	result, err := commandRunner.Run(ctx, "lsmod")
	if err != nil {
		return fmt.Errorf("list loaded modules before driver removal: %w", err)
	}
	loaded := map[string]bool{}
	for index, line := range strings.Split(string(result.Stdout), "\n") {
		fields := strings.Fields(line)
		if index > 0 && len(fields) > 0 {
			loaded[fields[0]] = true
		}
	}
	for index, packageName := range packages {
		module := strings.ReplaceAll(modules[index], "-", "_")
		if loaded[module] {
			return fmt.Errorf("refusing to remove %s while module %s is loaded", packageName, modules[index])
		}
		files, err := commandRunner.Run(ctx, xbpsQuery, "-f", packageName)
		if err != nil {
			return fmt.Errorf("verify driver package %s: %w", packageName, err)
		}
		normalized := strings.ReplaceAll(strings.ToLower(string(files.Stdout)), "-", "_")
		if !strings.Contains(normalized, strings.ToLower(module)) {
			return fmt.Errorf("package %s does not contain evidence for module %s", packageName, modules[index])
		}
	}
	return nil
}

func availableSpace(path string) int64 {
	var state unix.Statfs_t
	if err := unix.Statfs(path, &state); err != nil {
		return 0
	}
	return int64(state.Bavail) * int64(state.Bsize)
}

func validateKernelPurge(ctx context.Context, commandRunner runner.Runner, requested []string) error {
	result, err := commandRunner.Run(ctx, vkpurge, "list")
	if err != nil {
		return fmt.Errorf("list old kernels: %w", err)
	}
	old := listedValues(result.Stdout)
	runningResult, err := commandRunner.Run(ctx, "/usr/bin/uname", "-r")
	if err != nil {
		return fmt.Errorf("detect running kernel before purge: %w", err)
	}
	running := strings.TrimSpace(string(runningResult.Stdout))
	for _, version := range requested {
		if !old[version] {
			return fmt.Errorf("kernel %s is not reported as old by vkpurge", version)
		}
		if version == running {
			return fmt.Errorf("refusing to purge running kernel %s", version)
		}
	}
	return nil
}

func validateKernelPackageRemoval(ctx context.Context, commandRunner runner.Runner, requested []string) error {
	requestedKernels := map[string]bool{}
	for _, name := range requested {
		result, err := commandRunner.Run(ctx, xbpsQuery, "--files", name)
		if err != nil {
			return fmt.Errorf("inspect package files before removal: %w", err)
		}
		if containsKernelBootImage(result.Stdout) {
			requestedKernels[name] = true
		}
	}
	if len(requestedKernels) == 0 {
		return nil
	}
	owners, err := commandRunner.Run(ctx, xbpsQuery, "--ownedby", "/boot/vmlinuz-*")
	if err != nil {
		return fmt.Errorf("identify installed kernels before removal: %w", err)
	}
	runningResult, err := commandRunner.Run(ctx, "/usr/bin/uname", "-r")
	if err != nil {
		return fmt.Errorf("detect running kernel before removal: %w", err)
	}
	return validateKernelRemovalState(requested, parseKernelOwners(owners.Stdout), strings.TrimSpace(string(runningResult.Stdout)))
}

func validateKernelRemovalState(requested []string, kernels map[string][]string, running string) error {
	hasKernel := false
	for _, name := range requested {
		if len(kernels[name]) > 0 {
			hasKernel = true
			break
		}
	}
	if !hasKernel {
		return nil
	}
	if len(kernels) == 0 {
		return errors.New("kernel package state is unknown; removal is blocked")
	}
	matches := []string{}
	for name, versions := range kernels {
		for _, version := range versions {
			if version == running {
				matches = append(matches, name)
				break
			}
		}
	}
	if len(matches) != 1 {
		return errors.New("running kernel package could not be proven; removal is blocked")
	}
	runningPackage := matches[0]
	remove := map[string]bool{}
	for _, name := range requested {
		if len(kernels[name]) > 0 {
			remove[name] = true
		}
	}
	if remove[runningPackage] {
		return fmt.Errorf("refusing to remove running kernel package %s", runningPackage)
	}
	remaining := 0
	for name := range kernels {
		if !remove[name] {
			remaining++
		}
	}
	if remaining < 2 {
		return errors.New("kernel removal must preserve the running kernel and one installed fallback series")
	}
	return nil
}

func containsKernelBootImage(output []byte) bool {
	for _, path := range strings.Fields(string(output)) {
		if strings.HasPrefix(path, "/boot/vmlinuz-") {
			return true
		}
	}
	return false
}

func parseKernelOwners(output []byte) map[string][]string {
	kernels := map[string][]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		owner, path, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		name, _, ok := splitPackageVersion(owner)
		if !ok {
			continue
		}
		fields := strings.Fields(path)
		if len(fields) == 0 {
			continue
		}
		path = fields[0]
		if version := strings.TrimPrefix(path, "/boot/vmlinuz-"); version != path && version != "" {
			kernels[name] = append(kernels[name], version)
		}
	}
	return kernels
}

func listedValues(output []byte) map[string]bool {
	values := map[string]bool{}
	for _, value := range strings.Fields(string(output)) {
		values[value] = true
	}
	return values
}

func bootArtifactSize(version string) int64 {
	var total int64
	for _, prefix := range []string{"config-", "initramfs-", "System.map-", "vmlinuz-"} {
		info, err := os.Lstat(filepath.Join("/boot", prefix+version))
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
	}
	return total
}

func previewCommand(request security.PackageRequest) (string, []string, bool) {
	var args []string
	switch request.Operation {
	case security.Install:
		args = []string{"--dry-run", "--yes"}
	case security.Reinstall:
		args = []string{"--dry-run", "--yes", "--force"}
	case security.Update:
		args = []string{"--dry-run", "--yes", "--update"}
	case security.SystemUpgrade:
		return xbpsInstall, []string{"--memory-sync", "--dry-run", "--yes", "--update"}, true
	case security.Remove, security.RemoveDriver:
		return xbpsRemove, append([]string{"--dry-run", "--yes"}, request.Packages...), true
	case security.Purge:
		return xbpsRemove, append([]string{"--dry-run", "--yes", "--recursive"}, request.Packages...), true
	case security.RemoveOrphans:
		return xbpsRemove, []string{"--dry-run", "--yes", "--remove-orphans"}, true
	case security.ForceRemove:
		return xbpsRemove, append([]string{"--dry-run", "--yes", "--force-revdeps"}, request.Packages...), true
	default:
		return "", nil, false
	}
	return xbpsInstall, append(args, request.Packages...), true
}

func boundedLines(outputs ...[]byte) []string {
	var lines []string
	for _, output := range outputs {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() && len(lines) < maxDiagnosticLines {
			if line := strings.TrimSpace(scanner.Text()); line != "" {
				lines = append(lines, line)
			}
		}
	}
	return lines
}

func systemPackageFingerprint() (string, error) {
	paths, err := filepath.Glob("/var/db/xbps/pkgdb-*.plist")
	if err != nil || len(paths) == 0 {
		return "", errors.New("XBPS package database not found")
	}
	contents, err := os.ReadFile(paths[len(paths)-1])
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:]), nil
}
