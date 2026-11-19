package security

import (
	"errors"
	"fmt"
	"regexp"
)

const (
	APIVersion       = 1
	HelperAPIVersion = 13
)

type Operation string

const (
	Install        Operation = "install"
	Remove         Operation = "remove"
	Purge          Operation = "purge"
	Reinstall      Operation = "reinstall"
	Reconfigure    Operation = "reconfigure"
	Update         Operation = "update"
	Downgrade      Operation = "downgrade"
	SystemUpgrade  Operation = "system-upgrade"
	SyncDatabase   Operation = "sync-database"
	Hold           Operation = "hold"
	Unhold         Operation = "unhold"
	RepoLock       Operation = "repo-lock"
	RepoUnlock     Operation = "repo-unlock"
	RemoveOrphans  Operation = "remove-orphans"
	ForceRemove    Operation = "force-remove"
	CleanCache     Operation = "clean-cache"
	VerifyDatabase Operation = "verify-database"
	ReconfigureAll Operation = "reconfigure-all"
	PurgeKernels   Operation = "purge-kernels"
)

var (
	requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{7,63}$`)
	packagePattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.-]{0,127}$`)
)

type PackageRequest struct {
	Version       int       `json:"version"`
	RequestID     string    `json:"requestId"`
	Operation     Operation `json:"operation"`
	Packages      []string  `json:"packages"`
	TargetVersion string    `json:"targetVersion,omitempty"`
}

func (r PackageRequest) Validate() error {
	if r.Version != APIVersion {
		return fmt.Errorf("unsupported API version %d", r.Version)
	}
	if !requestIDPattern.MatchString(r.RequestID) {
		return errors.New("invalid request ID")
	}
	if !validOperation(r.Operation) {
		return errors.New("unsupported package operation")
	}
	if r.Operation == SystemUpgrade || r.Operation == SyncDatabase || r.Operation == RemoveOrphans || r.Operation == CleanCache || r.Operation == VerifyDatabase || r.Operation == ReconfigureAll {
		if len(r.Packages) != 0 {
			return fmt.Errorf("%s does not accept package names", r.Operation)
		}
		return nil
	}
	if r.Operation == Downgrade && (len(r.Packages) != 1 || !packagePattern.MatchString(r.TargetVersion)) {
		return errors.New("downgrade requires one package and a valid target version")
	}
	if r.Operation != Downgrade && r.TargetVersion != "" {
		return errors.New("target version is only valid for downgrade")
	}
	if len(r.Packages) == 0 || len(r.Packages) > 128 {
		return errors.New("package operation requires 1 to 128 packages")
	}
	seen := make(map[string]bool, len(r.Packages))
	for _, name := range r.Packages {
		if !packagePattern.MatchString(name) || seen[name] {
			return fmt.Errorf("invalid or duplicate package %q", name)
		}
		seen[name] = true
	}
	return nil
}

func validOperation(operation Operation) bool {
	switch operation {
	case Install, Remove, Purge, Reinstall, Reconfigure, Update, Downgrade, SystemUpgrade, SyncDatabase, Hold, Unhold, RepoLock, RepoUnlock, RemoveOrphans, ForceRemove, CleanCache, VerifyDatabase, ReconfigureAll, PurgeKernels:
		return true
	default:
		return false
	}
}
