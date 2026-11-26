package security

import "testing"

func TestXBPSRuleRequestValidatesFiniteOperationsAndValues(t *testing.T) {
	valid := []XBPSRuleRequest{
		{Version: 1, Operation: RuleAdd, Kind: RuleIgnorePackage, Value: "linux6.12"},
		{Version: 1, Operation: RuleAdd, Kind: RuleVirtualPackage, Value: "cron-daemon:dcron"},
		{Version: 1, Operation: RuleEdit, Kind: RuleIgnorePackage, OldValue: "linux6.6", Value: "linux6.12", File: "policy.conf"},
		{Version: 1, Operation: RuleRemove, Kind: RuleIgnorePackage, Value: "linux6.6", File: "policy.conf"},
	}
	for _, request := range valid {
		if err := request.Validate(); err != nil {
			t.Fatalf("%#v: %v", request, err)
		}
	}
}

func TestXBPSRuleRequestRejectsInjectionAndMissingOldValue(t *testing.T) {
	invalid := []XBPSRuleRequest{
		{Version: 1, Operation: RuleAdd, Kind: RuleIgnorePackage, Value: "foo\nrepository=https://evil"},
		{Version: 1, Operation: RuleAdd, Kind: RuleVirtualPackage, Value: "cron-daemon"},
		{Version: 1, Operation: RuleEdit, Kind: RuleIgnorePackage, Value: "linux6.12"},
		{Version: 1, Operation: RuleRemove, Kind: RuleIgnorePackage, Value: "linux", File: "../policy.conf"},
		{Version: 1, Operation: RuleAdd, Kind: RuleIgnorePackage, Value: "linux", File: "policy.conf"},
	}
	for _, request := range invalid {
		if err := request.Validate(); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
}
