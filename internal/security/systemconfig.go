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
	SambaEdit         SystemConfigOperation = "samba-edit"
	SambaRemove       SystemConfigOperation = "samba-remove"
	SambaGlobalSet    SystemConfigOperation = "samba-global-set"
	NFSAdd            SystemConfigOperation = "nfs-add"
	NFSEdit           SystemConfigOperation = "nfs-edit"
	NFSRemove         SystemConfigOperation = "nfs-remove"
	NFSMount          SystemConfigOperation = "nfs-mount"
	NFSUnmount        SystemConfigOperation = "nfs-unmount"
	KernelSetDefault  SystemConfigOperation = "kernel-set-default"
)

var shareNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 _.-]{0,63}$`)
var xbpsConfigFilePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.conf$`)
var sambaPrincipalPattern = regexp.MustCompile(`^[@+&]?[A-Za-z0-9_.-]+\$?$`)
var sambaAccountPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*\$?$`)

type SystemConfigRequest struct {
	Version        int                   `json:"version"`
	Operation      SystemConfigOperation `json:"operation"`
	Value          string                `json:"value,omitempty"`
	OldValue       string                `json:"oldValue,omitempty"`
	Path           string                `json:"path,omitempty"`
	OldPath        string                `json:"oldPath,omitempty"`
	ReadOnly       bool                  `json:"readOnly,omitempty"`
	GuestOK        bool                  `json:"guestOK,omitempty"`
	Comment        string                `json:"comment,omitempty"`
	Browseable     bool                  `json:"browseable,omitempty"`
	FollowSymlinks bool                  `json:"followSymlinks,omitempty"`
	WideLinks      bool                  `json:"wideLinks,omitempty"`
	ValidUsers     string                `json:"validUsers,omitempty"`
	WriteList      string                `json:"writeList,omitempty"`
	ReadList       string                `json:"readList,omitempty"`
	ForceUser      string                `json:"forceUser,omitempty"`
	ForceGroup     string                `json:"forceGroup,omitempty"`
	CreateMask     string                `json:"createMask,omitempty"`
	DirectoryMask  string                `json:"directoryMask,omitempty"`
	Protocol       string                `json:"protocol,omitempty"`
	DefaultMask    string                `json:"defaultMask,omitempty"`
	Printing       bool                  `json:"printing,omitempty"`
	MapGuest       bool                  `json:"mapGuest,omitempty"`
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
		if !validHTTPSURL(r.Value) || !validXBPSConfigFile(r.Path) || !r.TrustConfirmed {
			return errors.New("a valid HTTPS URL, configuration filename, and explicit repository trust confirmation are required")
		}
	case RepositoryRemove, RepositoryEnable, RepositoryDisable, MirrorUse:
		if !validHTTPSURL(r.Value) || (r.Operation != MirrorUse && !validXBPSConfigFile(r.Path)) {
			return errors.New("a valid HTTPS URL and configuration filename are required")
		}
	case RepositoryEdit:
		if !validHTTPSURL(r.Value) || !validHTTPSURL(r.OldValue) || r.Value == r.OldValue || !validXBPSConfigFile(r.Path) || !r.TrustConfirmed {
			return errors.New("distinct valid repository URLs, a configuration filename, and explicit trust confirmation are required")
		}
	case MirrorReset:
		if r.Value != "" {
			return errors.New("mirror reset does not accept a URL")
		}
	case SambaAdd:
		if !validSambaShareName(r.Value) || !validAbsolutePath(r.Path) || !validSambaOptions(r) {
			return errors.New("a valid share name and absolute path are required")
		}
	case SambaEdit:
		if !validSambaShareName(r.Value) || !validSambaShareName(r.OldValue) || !validAbsolutePath(r.Path) || !validSambaOptions(r) {
			return errors.New("valid current and replacement share names and an absolute path are required")
		}
	case SambaRemove:
		if !validSambaShareName(r.Value) {
			return errors.New("a valid non-global share name is required")
		}
	case SambaGlobalSet:
		if (r.Protocol != "SMB2" && r.Protocol != "SMB3") || !validSambaMask(r.DefaultMask) || r.DefaultMask == "" {
			return errors.New("a supported minimum SMB protocol and default permission mask are required")
		}
	case NFSAdd:
		if !validNFSSource(r.Value) || !validAbsolutePath(r.Path) || (r.Type != "nfs" && r.Type != "nfs4") || !validMountOptions(r.Options) {
			return errors.New("a valid NFS source, target, type, and options are required")
		}
	case NFSEdit:
		if !validNFSSource(r.Value) || !validNFSSource(r.OldValue) || !validAbsolutePath(r.Path) || !validAbsolutePath(r.OldPath) || (r.Type != "nfs" && r.Type != "nfs4") || !validMountOptions(r.Options) {
			return errors.New("valid current and replacement NFS entries are required")
		}
	case NFSRemove, NFSMount, NFSUnmount:
		if !validNFSSource(r.Value) || !validAbsolutePath(r.Path) {
			return errors.New("a valid NFS source and mount point are required")
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

func validXBPSConfigFile(name string) bool {
	return xbpsConfigFilePattern.MatchString(name) && !strings.Contains(name, "..") && filepath.Base(name) == name
}

func validSambaOptions(request SystemConfigRequest) bool {
	if len(request.Comment) > 256 || strings.ContainsAny(request.Comment, "\n\r\x00") {
		return false
	}
	if !validSambaPrincipals(request.ValidUsers) || !validSambaPrincipals(request.WriteList) || !validSambaPrincipals(request.ReadList) {
		return false
	}
	if (request.ForceUser != "" && !sambaAccountPattern.MatchString(request.ForceUser)) || (request.ForceGroup != "" && !sambaAccountPattern.MatchString(request.ForceGroup)) {
		return false
	}
	return validSambaMask(request.CreateMask) && validSambaMask(request.DirectoryMask)
}

func validSambaPrincipals(value string) bool {
	if value == "" {
		return true
	}
	for _, principal := range strings.Split(value, ",") {
		if !sambaPrincipalPattern.MatchString(strings.TrimSpace(principal)) {
			return false
		}
	}
	return true
}

func validSambaMask(value string) bool {
	switch value {
	case "", "0744", "0755", "0770", "0775", "0777":
		return true
	default:
		return false
	}
}

func validSambaShareName(value string) bool {
	return shareNamePattern.MatchString(value) && !strings.EqualFold(value, "global")
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
