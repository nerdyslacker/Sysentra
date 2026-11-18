package security

import (
	"errors"
	"strings"
)

const XBPSConfigManagementAction = "io.github.nerdyslacker.sysentra.manage-xbps-config"

type RuleOperation string
type RuleKind string

const (
	RuleAdd    RuleOperation = "add"
	RuleEdit   RuleOperation = "edit"
	RuleRemove RuleOperation = "remove"

	RuleIgnorePackage  RuleKind = "ignorepkg"
	RuleVirtualPackage RuleKind = "virtualpkg"
)

type XBPSRuleRequest struct {
	Version   int           `json:"version"`
	Operation RuleOperation `json:"operation"`
	Kind      RuleKind      `json:"kind"`
	Value     string        `json:"value"`
	OldValue  string        `json:"oldValue,omitempty"`
}

func (r XBPSRuleRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	if r.Operation != RuleAdd && r.Operation != RuleEdit && r.Operation != RuleRemove {
		return errors.New("unsupported XBPS rule operation")
	}
	if !validRuleValue(r.Kind, r.Value) {
		return errors.New("invalid XBPS rule value")
	}
	if r.Operation == RuleEdit {
		if !validRuleValue(r.Kind, r.OldValue) || r.OldValue == r.Value {
			return errors.New("edit requires distinct valid old and new values")
		}
	} else if r.OldValue != "" {
		return errors.New("old value is only valid for edit")
	}
	return nil
}

func validRuleValue(kind RuleKind, value string) bool {
	switch kind {
	case RuleIgnorePackage:
		return packagePattern.MatchString(value)
	case RuleVirtualPackage:
		parts := strings.Split(value, ":")
		return len(parts) == 2 && packagePattern.MatchString(parts[0]) && packagePattern.MatchString(parts[1])
	default:
		return false
	}
}
