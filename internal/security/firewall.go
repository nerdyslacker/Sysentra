package security

import (
	"errors"
	"net"
	"regexp"
	"strconv"
)

const FirewallManagementAction = "io.github.nerdyslacker.sysentra.manage-firewall"

type FirewallOperation string

const (
	FirewallSelect     FirewallOperation = "select"
	FirewallEnable     FirewallOperation = "enable"
	FirewallDisable    FirewallOperation = "disable"
	FirewallReload     FirewallOperation = "reload"
	FirewallRuleAdd    FirewallOperation = "rule-add"
	FirewallRuleEdit   FirewallOperation = "rule-edit"
	FirewallRuleRemove FirewallOperation = "rule-remove"
	FirewallPolicySet  FirewallOperation = "policy-set"
	FirewallZoneSet    FirewallOperation = "zone-set"
)

type FirewallRule struct {
	Kind      string `json:"kind"`
	Target    string `json:"target"`
	Protocol  string `json:"protocol,omitempty"`
	Direction string `json:"direction,omitempty"`
	Source    string `json:"source,omitempty"`
	Action    string `json:"action,omitempty"`
}

type FirewallRequest struct {
	Version   int               `json:"version"`
	Operation FirewallOperation `json:"operation"`
	Backend   string            `json:"backend"`
	Zone      string            `json:"zone,omitempty"`
	Policy    string            `json:"policy,omitempty"`
	Direction string            `json:"direction,omitempty"`
	Rule      FirewallRule      `json:"rule,omitempty"`
	OldRule   FirewallRule      `json:"oldRule,omitempty"`
}

var firewallName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$`)

func (r FirewallRequest) Validate() error {
	if r.Version != APIVersion || (r.Backend != "ufw" && r.Backend != "firewalld") {
		return errors.New("invalid firewall request")
	}
	switch r.Operation {
	case FirewallSelect, FirewallEnable, FirewallDisable, FirewallReload:
		if r.Zone != "" || r.Policy != "" || r.Direction != "" || !emptyFirewallRule(r.Rule) || !emptyFirewallRule(r.OldRule) {
			return errors.New("firewall lifecycle operation does not accept configuration fields")
		}
	case FirewallRuleAdd, FirewallRuleRemove:
		if err := validateFirewallRule(r.Backend, r.Rule); err != nil || !emptyFirewallRule(r.OldRule) || r.Policy != "" || r.Direction != "" || !validFirewallZoneField(r.Backend, r.Zone) {
			return errors.New("invalid firewall rule")
		}
	case FirewallRuleEdit:
		if validateFirewallRule(r.Backend, r.Rule) != nil || validateFirewallRule(r.Backend, r.OldRule) != nil || r.Rule == r.OldRule || r.Policy != "" || r.Direction != "" || !validFirewallZoneField(r.Backend, r.Zone) {
			return errors.New("distinct valid firewall rules are required")
		}
	case FirewallPolicySet:
		if r.Backend != "ufw" || r.Zone != "" || !emptyFirewallRule(r.Rule) || !emptyFirewallRule(r.OldRule) || (r.Policy != "allow" && r.Policy != "deny" && r.Policy != "reject") || (r.Direction != "incoming" && r.Direction != "outgoing" && r.Direction != "routed") {
			return errors.New("invalid UFW default policy")
		}
	case FirewallZoneSet:
		if r.Backend != "firewalld" || !firewallName.MatchString(r.Zone) || r.Policy != "" || r.Direction != "" || !emptyFirewallRule(r.Rule) || !emptyFirewallRule(r.OldRule) {
			return errors.New("invalid firewalld zone")
		}
	default:
		return errors.New("unsupported firewall operation")
	}
	return nil
}

func validateFirewallRule(backend string, rule FirewallRule) error {
	if rule.Kind != "port" && (backend != "firewalld" || rule.Kind != "service") {
		return errors.New("invalid rule kind")
	}
	if rule.Kind == "port" {
		port, err := strconv.Atoi(rule.Target)
		if err != nil || port < 1 || port > 65535 || (rule.Protocol != "tcp" && rule.Protocol != "udp") {
			return errors.New("invalid port rule")
		}
	} else if !firewallName.MatchString(rule.Target) || rule.Protocol != "" {
		return errors.New("invalid service rule")
	}
	if backend == "ufw" {
		if rule.Direction != "" && rule.Direction != "in" && rule.Direction != "out" {
			return errors.New("invalid UFW direction")
		}
		if rule.Action != "allow" && rule.Action != "deny" && rule.Action != "reject" && rule.Action != "limit" {
			return errors.New("invalid UFW action")
		}
		if rule.Source != "" && rule.Source != "any" && net.ParseIP(rule.Source) == nil {
			if _, _, err := net.ParseCIDR(rule.Source); err != nil {
				return errors.New("invalid rule source")
			}
		}
	} else if rule.Action != "allow" || rule.Direction != "" || (rule.Source != "" && rule.Source != "any") {
		return errors.New("firewalld simple rules only support zone-wide allow")
	}
	return nil
}

func emptyFirewallRule(rule FirewallRule) bool { return rule == (FirewallRule{}) }

func validFirewallZoneField(backend, zone string) bool {
	return backend == "ufw" && zone == "" || backend == "firewalld" && firewallName.MatchString(zone)
}
