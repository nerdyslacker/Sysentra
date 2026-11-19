package security

import (
	"errors"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

const SystemConfigManagementAction = "io.github.nerdyslacker.sysentra.manage-system-config"

type SystemConfigOperation string

const (
	RepositoryAdd     SystemConfigOperation = "repository-add"
	RepositoryEdit    SystemConfigOperation = "repository-edit"
	RepositoryRemove  SystemConfigOperation = "repository-remove"
	RepositoryEnable  SystemConfigOperation = "repository-enable"
	RepositoryDisable SystemConfigOperation = "repository-disable"
	MirrorUse         SystemConfigOperation = "mirror-use"
	MirrorReset       SystemConfigOperation = "mirror-reset"
	SambaAdd          SystemConfigOperation = "samba-add"
	NFSAdd            SystemConfigOperation = "nfs-add"
	KernelSetDefault  SystemConfigOperation = "kernel-set-default"
)

var shareNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)

type SystemConfigRequest struct {
	Version        int                   `json:"version"`
	Operation      SystemConfigOperation `json:"operation"`
	Value          string                `json:"value,omitempty"`
	OldValue       string                `json:"oldValue,omitempty"`
	Path           string                `json:"path,omitempty"`
	ReadOnly       bool                  `json:"readOnly,omitempty"`
	GuestOK        bool                  `json:"guestOK,omitempty"`
	Type           string                `json:"type,omitempty"`
	Options        string                `json:"options,omitempty"`
	TrustConfirmed bool                  `json:"trustConfirmed,omitempty"`
}

func (r SystemConfigRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	switch r.Operation {
	case RepositoryAdd:
		if !validHTTPSURL(r.Value) || !r.TrustConfirmed {
			return errors.New("a valid HTTPS URL and explicit repository trust confirmation are required")
		}
	case RepositoryRemove, RepositoryEnable, RepositoryDisable, MirrorUse:
		if !validHTTPSURL(r.Value) {
			return errors.New("a valid HTTPS URL is required")
		}
	case RepositoryEdit:
		if !validHTTPSURL(r.Value) || !validHTTPSURL(r.OldValue) || r.Value == r.OldValue || !r.TrustConfirmed {
			return errors.New("distinct valid repository URLs and explicit trust confirmation are required")
		}
	case MirrorReset:
		if r.Value != "" {
			return errors.New("mirror reset does not accept a URL")
		}
	case SambaAdd:
		if !shareNamePattern.MatchString(r.Value) || !validAbsolutePath(r.Path) {
			return errors.New("a valid share name and absolute path are required")
		}
	case NFSAdd:
		if !validNFSSource(r.Value) || !validAbsolutePath(r.Path) || (r.Type != "nfs" && r.Type != "nfs4") || !validMountOptions(r.Options) {
			return errors.New("a valid NFS source, target, type, and options are required")
		}
	case KernelSetDefault:
		if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$`).MatchString(r.Value) {
			return errors.New("a valid installed kernel version is required")
		}
	default:
		return errors.New("unsupported system configuration operation")
	}
	return nil
}

func validHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}

func validAbsolutePath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value && !strings.ContainsAny(value, "\n\r\x00")
}

func validNFSSource(value string) bool {
	host, path, ok := strings.Cut(value, ":")
	return ok && host != "" && !strings.ContainsAny(host, " /\\\t\n\r") && validAbsolutePath(path)
}

func validMountOptions(value string) bool {
	if value == "" {
		return true
	}
	for _, option := range strings.Split(value, ",") {
		if option == "" || strings.ContainsAny(option, " \t\n\r#") {
			return false
		}
	}
	return true
}
