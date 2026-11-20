package security

import (
	"errors"
	"path/filepath"
	"regexp"
	"strings"
)

const AccountManagementAction = "io.github.nerdyslacker.sysentra.manage-accounts"

type AccountOperation string

const (
	UserAdd            AccountOperation = "user-add"
	UserRemove         AccountOperation = "user-remove"
	GroupAdd           AccountOperation = "group-add"
	GroupRemove        AccountOperation = "group-remove"
	MembershipAdd      AccountOperation = "membership-add"
	MembershipRemove   AccountOperation = "membership-remove"
	UserEdit           AccountOperation = "user-edit"
	PrimaryGroupChange AccountOperation = "primary-group-change"
	PasswordLock       AccountOperation = "password-lock"
	PasswordUnlock     AccountOperation = "password-unlock"
)

var accountNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

type AccountRequest struct {
	Version    int              `json:"version"`
	Operation  AccountOperation `json:"operation"`
	User       string           `json:"user,omitempty"`
	Group      string           `json:"group,omitempty"`
	Groups     []string         `json:"groups,omitempty"`
	CreateHome bool             `json:"createHome,omitempty"`
	Home       string           `json:"home,omitempty"`
	Shell      string           `json:"shell,omitempty"`
	MoveHome   bool             `json:"moveHome,omitempty"`
}

func (r AccountRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	switch r.Operation {
	case UserAdd, UserRemove:
		if !accountNamePattern.MatchString(r.User) || r.Group != "" || len(r.Groups) != 0 || r.Shell != "" || r.MoveHome {
			return errors.New("a valid user name is required")
		}
		if r.Operation == UserAdd {
			if r.Home != "" && (!r.CreateHome || !validHomePath(r.Home)) {
				return errors.New("custom home requires creation and a valid absolute path")
			}
		} else if r.CreateHome || r.Home != "" {
			return errors.New("home options are only valid when adding a user")
		}
	case GroupAdd, GroupRemove:
		if !accountNamePattern.MatchString(r.Group) || r.User != "" || len(r.Groups) != 0 || r.CreateHome || r.Home != "" || r.Shell != "" || r.MoveHome {
			return errors.New("a valid group name is required")
		}
	case MembershipAdd:
		if !accountNamePattern.MatchString(r.User) || !validAccountGroups(r.Groups) || r.Group != "" || r.CreateHome || r.Home != "" || r.Shell != "" || r.MoveHome {
			return errors.New("a valid user and one or more groups are required")
		}
	case MembershipRemove:
		if !accountNamePattern.MatchString(r.User) || !accountNamePattern.MatchString(r.Group) || len(r.Groups) != 0 || r.CreateHome || r.Home != "" || r.Shell != "" || r.MoveHome {
			return errors.New("valid user and group names are required")
		}
	case UserEdit:
		if !accountNamePattern.MatchString(r.User) || r.Group != "" || len(r.Groups) != 0 || r.CreateHome || (r.Home == "" && r.Shell == "") || (r.Home != "" && !validHomePath(r.Home)) || (r.Shell != "" && !validShellPath(r.Shell)) || (r.MoveHome && r.Home == "") {
			return errors.New("user editing requires a valid home or shell")
		}
	case PrimaryGroupChange:
		if !accountNamePattern.MatchString(r.User) || !accountNamePattern.MatchString(r.Group) || len(r.Groups) != 0 || r.CreateHome || r.Home != "" || r.Shell != "" || r.MoveHome {
			return errors.New("primary-group change requires valid user and group names")
		}
	case PasswordLock, PasswordUnlock:
		if !accountNamePattern.MatchString(r.User) || r.Group != "" || len(r.Groups) != 0 || r.CreateHome || r.Home != "" || r.Shell != "" || r.MoveHome {
			return errors.New("password lock change requires a valid user name")
		}
	default:
		return errors.New("unsupported account operation")
	}
	if (r.Operation == UserRemove && r.User == "root") || (r.Operation == GroupRemove && r.Group == "root") {
		return errors.New("the root account cannot be removed")
	}
	return nil
}

func validShellPath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value && value != "/" && !strings.ContainsAny(value, ":\n\r\x00")
}

func validHomePath(value string) bool {
	return filepath.IsAbs(value) && filepath.Clean(value) == value && value != "/" && !strings.ContainsAny(value, ":\n\r\x00")
}

func validAccountGroups(groups []string) bool {
	if len(groups) == 0 || len(groups) > 64 {
		return false
	}
	seen := map[string]bool{}
	for _, group := range groups {
		if !accountNamePattern.MatchString(group) || seen[group] || strings.Contains(group, ",") {
			return false
		}
		seen[group] = true
	}
	return true
}
