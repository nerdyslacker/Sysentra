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
	UserAdd          AccountOperation = "user-add"
	UserRemove       AccountOperation = "user-remove"
	GroupAdd         AccountOperation = "group-add"
	GroupRemove      AccountOperation = "group-remove"
	MembershipAdd    AccountOperation = "membership-add"
	MembershipRemove AccountOperation = "membership-remove"
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
}

func (r AccountRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	switch r.Operation {
	case UserAdd, UserRemove:
		if !accountNamePattern.MatchString(r.User) || r.Group != "" || len(r.Groups) != 0 {
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
		if !accountNamePattern.MatchString(r.Group) || r.User != "" || len(r.Groups) != 0 || r.CreateHome || r.Home != "" {
			return errors.New("a valid group name is required")
		}
	case MembershipAdd:
		if !accountNamePattern.MatchString(r.User) || !validAccountGroups(r.Groups) || r.Group != "" || r.CreateHome || r.Home != "" {
			return errors.New("a valid user and one or more groups are required")
		}
	case MembershipRemove:
		if !accountNamePattern.MatchString(r.User) || !accountNamePattern.MatchString(r.Group) || len(r.Groups) != 0 || r.CreateHome || r.Home != "" {
			return errors.New("valid user and group names are required")
		}
	default:
		return errors.New("unsupported account operation")
	}
	if (r.Operation == UserRemove && r.User == "root") || (r.Operation == GroupRemove && r.Group == "root") {
		return errors.New("the root account cannot be removed")
	}
	return nil
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
