package firewall

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type stateReader interface {
	ReadBackend(context.Context, string) (State, error)
}

type serviceChanger interface {
	Change(context.Context, security.ServiceRequest) error
}

type Manager struct {
	runner    runner.Runner
	inventory stateReader
	services  serviceChanger
}

func NewManager(commandRunner runner.Runner, inventory stateReader, services serviceChanger) *Manager {
	return &Manager{runner: commandRunner, inventory: inventory, services: services}
}

func (m *Manager) Change(ctx context.Context, request security.FirewallRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	before, err := m.inventory.ReadBackend(ctx, request.Backend)
	if err != nil || !before.Installed {
		return errors.New("firewall backend is not installed")
	}
	disabledBackends := []string{}
	switch request.Operation {
	case security.FirewallSelect:
		for _, backend := range before.Backends {
			if backend == request.Backend {
				continue
			}
			state, err := m.inventory.ReadBackend(ctx, backend)
			if err != nil {
				return err
			}
			if state.Active {
				if err := m.setActive(ctx, backend, false); err != nil {
					return err
				}
				disabledBackends = append(disabledBackends, backend)
			}
		}
		if err := m.setActive(ctx, request.Backend, true); err != nil {
			_ = m.setActive(ctx, request.Backend, false)
			for _, backend := range disabledBackends {
				_ = m.setActive(ctx, backend, true)
			}
			return err
		}
	case security.FirewallEnable:
		for _, backend := range before.Backends {
			if backend == request.Backend {
				continue
			}
			state, err := m.inventory.ReadBackend(ctx, backend)
			if err != nil {
				return err
			}
			if state.Active {
				return fmt.Errorf("%s is active; select %s explicitly to switch backends", backend, request.Backend)
			}
		}
		if err := m.setActive(ctx, request.Backend, true); err != nil {
			return err
		}
	case security.FirewallDisable:
		if err := m.setActive(ctx, request.Backend, false); err != nil {
			return err
		}
	case security.FirewallReload:
		if !before.Active {
			return errors.New("firewall must be active before reload")
		}
		if request.Backend == "ufw" {
			err = m.run(ctx, "/usr/bin/ufw", "reload")
		} else {
			err = m.run(ctx, "/usr/bin/firewall-cmd", "--reload")
		}
	case security.FirewallPolicySet:
		if request.Direction == "incoming" && request.Policy != "allow" && !hasSSHAccess(before.Rules) {
			return errors.New("refusing a restrictive incoming policy until an SSH allow rule exists")
		}
		err = m.run(ctx, "/usr/bin/ufw", "default", request.Policy, request.Direction)
	case security.FirewallZoneSet:
		if err = m.run(ctx, "/usr/bin/firewall-cmd", "--zone", request.Zone, "--query-service=ssh"); err != nil {
			return errors.New("refusing a default zone that does not allow SSH")
		}
		err = m.run(ctx, "/usr/bin/firewall-cmd", "--set-default-zone="+request.Zone)
	case security.FirewallRuleAdd:
		err = m.addRule(ctx, request.Backend, zone(before, request.Zone), request.Rule)
	case security.FirewallRuleRemove:
		if protectedRule(request.Rule) {
			return errors.New("SSH access rules are protected from removal")
		}
		err = m.removeRule(ctx, request.Backend, zone(before, request.Zone), request.Rule)
	case security.FirewallRuleEdit:
		if protectedRule(request.OldRule) {
			return errors.New("SSH access rules are protected from editing")
		}
		if err = m.removeRule(ctx, request.Backend, zone(before, request.Zone), request.OldRule); err == nil {
			if err = m.addRule(ctx, request.Backend, zone(before, request.Zone), request.Rule); err != nil {
				_ = m.addRule(ctx, request.Backend, zone(before, request.Zone), request.OldRule)
			}
		}
	}
	if err != nil {
		return err
	}
	after, err := m.inventory.ReadBackend(ctx, request.Backend)
	if err != nil {
		return fmt.Errorf("verify firewall change: %w", err)
	}
	if err := verifyChange(request, after); err != nil {
		return err
	}
	if request.Operation == security.FirewallSelect {
		for _, backend := range before.Backends {
			if backend == request.Backend {
				continue
			}
			state, err := m.inventory.ReadBackend(ctx, backend)
			if err != nil || state.Active {
				return errors.New("another firewall backend remained active")
			}
		}
	}
	return nil
}

func (m *Manager) setActive(ctx context.Context, backend string, active bool) error {
	if backend == "ufw" {
		if active {
			return m.run(ctx, "/usr/bin/ufw", "--force", "enable")
		}
		return m.run(ctx, "/usr/bin/ufw", "disable")
	}
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceStop, Name: "firewalld"}
	if active {
		request.Operation = security.ServiceEnable
		if err := m.services.Change(ctx, request); err != nil {
			return err
		}
		request.Operation = security.ServiceStart
		return m.services.Change(ctx, request)
	}
	if err := m.services.Change(ctx, request); err != nil {
		return err
	}
	request.Operation = security.ServiceDisable
	return m.services.Change(ctx, request)
}

func (m *Manager) addRule(ctx context.Context, backend, zone string, rule security.FirewallRule) error {
	if backend == "ufw" {
		return m.run(ctx, "/usr/bin/ufw", ufwRuleArgs(rule, false)...)
	}
	if err := m.run(ctx, "/usr/bin/firewall-cmd", firewalldRuleArgs(zone, rule, true)...); err != nil {
		return err
	}
	if err := m.run(ctx, "/usr/bin/firewall-cmd", "--reload"); err != nil {
		_ = m.run(ctx, "/usr/bin/firewall-cmd", firewalldRuleArgs(zone, rule, false)...)
		_ = m.run(ctx, "/usr/bin/firewall-cmd", "--reload")
		return err
	}
	return nil
}

func (m *Manager) removeRule(ctx context.Context, backend, zone string, rule security.FirewallRule) error {
	if backend == "ufw" {
		return m.run(ctx, "/usr/bin/ufw", ufwRuleArgs(rule, true)...)
	}
	if err := m.run(ctx, "/usr/bin/firewall-cmd", firewalldRuleArgs(zone, rule, false)...); err != nil {
		return err
	}
	if err := m.run(ctx, "/usr/bin/firewall-cmd", "--reload"); err != nil {
		_ = m.run(ctx, "/usr/bin/firewall-cmd", firewalldRuleArgs(zone, rule, true)...)
		_ = m.run(ctx, "/usr/bin/firewall-cmd", "--reload")
		return err
	}
	return nil
}

func ufwRuleArgs(rule security.FirewallRule, remove bool) []string {
	args := []string{}
	if remove {
		args = append(args, "--force", "delete")
	}
	args = append(args, rule.Action)
	if rule.Direction != "" {
		args = append(args, rule.Direction)
	}
	args = append(args, "proto", rule.Protocol)
	if rule.Source != "" && rule.Source != "any" {
		args = append(args, "from", rule.Source)
	}
	args = append(args, "to", "any", "port", rule.Target)
	return args
}

func firewalldRuleArgs(zone string, rule security.FirewallRule, add bool) []string {
	operation := "--remove-"
	if add {
		operation = "--add-"
	}
	value := rule.Target
	if rule.Kind == "port" {
		operation += "port="
		value += "/" + rule.Protocol
	} else {
		operation += "service="
	}
	return []string{"--permanent", "--zone", zone, operation + value}
}

func (m *Manager) run(ctx context.Context, command string, args ...string) error {
	result, err := m.runner.Run(ctx, command, args...)
	if err != nil {
		return fmt.Errorf("run firewall command: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func zone(state State, requested string) string {
	if requested != "" {
		return requested
	}
	return state.Zone
}

func protectedRule(rule security.FirewallRule) bool {
	return rule.Kind == "service" && rule.Target == "ssh" || rule.Kind == "port" && rule.Target == "22" && rule.Protocol == "tcp"
}

func hasSSHAccess(rules []Rule) bool {
	for _, rule := range rules {
		if protectedRule(security.FirewallRule{Kind: rule.Kind, Target: rule.Target, Protocol: rule.Protocol}) && strings.EqualFold(rule.Action, "allow") {
			return true
		}
	}
	return false
}

func verifyChange(request security.FirewallRequest, state State) error {
	switch request.Operation {
	case security.FirewallSelect, security.FirewallEnable:
		if !state.Active {
			return errors.New("firewall did not become active")
		}
	case security.FirewallDisable:
		if state.Active {
			return errors.New("firewall remained active")
		}
	case security.FirewallPolicySet:
		if !strings.Contains(strings.ToLower(state.Policy), request.Policy+" ("+request.Direction+")") {
			return errors.New("firewall policy verification failed")
		}
	case security.FirewallZoneSet:
		if state.Zone != request.Zone {
			return errors.New("firewall zone verification failed")
		}
	case security.FirewallRuleAdd, security.FirewallRuleEdit:
		if !containsRule(state.Rules, request.Rule) {
			return errors.New("firewall rule was not found after the change")
		}
	case security.FirewallRuleRemove:
		if containsRule(state.Rules, request.Rule) {
			return errors.New("firewall rule remained after removal")
		}
	}
	return nil
}

func containsRule(rules []Rule, wanted security.FirewallRule) bool {
	for _, rule := range rules {
		if rule.Kind == wanted.Kind && rule.Target == wanted.Target && rule.Protocol == wanted.Protocol && rule.Direction == wanted.Direction && strings.EqualFold(rule.Action, wanted.Action) && (wanted.Source == "" || wanted.Source == "any" || strings.EqualFold(rule.Source, wanted.Source)) {
			return true
		}
	}
	return false
}
