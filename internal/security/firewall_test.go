package security

import "testing"

func TestFirewallRequestValidation(t *testing.T) {
	valid := []FirewallRequest{
		{Version: APIVersion, Operation: FirewallSelect, Backend: "ufw"},
		{Version: APIVersion, Operation: FirewallRuleAdd, Backend: "ufw", Rule: FirewallRule{Kind: "port", Target: "443", Protocol: "tcp", Source: "10.0.0.0/8", Action: "allow"}},
		{Version: APIVersion, Operation: FirewallRuleAdd, Backend: "firewalld", Zone: "public", Rule: FirewallRule{Kind: "service", Target: "https", Action: "allow"}},
		{Version: APIVersion, Operation: FirewallPolicySet, Backend: "ufw", Policy: "deny", Direction: "incoming"},
		{Version: APIVersion, Operation: FirewallZoneSet, Backend: "firewalld", Zone: "public"},
	}
	for _, request := range valid {
		if err := request.Validate(); err != nil {
			t.Errorf("%#v: %v", request, err)
		}
	}
	invalid := []FirewallRequest{
		{Version: APIVersion, Operation: FirewallRuleAdd, Backend: "ufw", Rule: FirewallRule{Kind: "port", Target: "70000", Protocol: "tcp", Action: "allow"}},
		{Version: APIVersion, Operation: FirewallRuleAdd, Backend: "firewalld", Rule: FirewallRule{Kind: "port", Target: "22", Protocol: "tcp", Action: "deny"}},
		{Version: APIVersion, Operation: FirewallZoneSet, Backend: "ufw", Zone: "public"},
		{Version: APIVersion, Operation: FirewallRuleAdd, Backend: "ufw", Zone: "public", Rule: FirewallRule{Kind: "port", Target: "443", Protocol: "tcp", Action: "allow"}},
		{Version: APIVersion, Operation: FirewallPolicySet, Backend: "ufw", Policy: "deny", Direction: "incoming", Rule: FirewallRule{Kind: "port", Target: "443", Protocol: "tcp", Action: "allow"}},
		{Version: APIVersion, Operation: FirewallSelect, Backend: "nftables"},
	}
	for _, request := range invalid {
		if err := request.Validate(); err == nil {
			t.Errorf("expected %#v to fail", request)
		}
	}
}
