package firewall

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type managerRunner struct{ calls [][]string }

func (r *managerRunner) Run(_ context.Context, command string, args ...string) (runner.Result, error) {
	r.calls = append(r.calls, append([]string{command}, args...))
	return runner.Result{}, nil
}

type managerInventory struct{ states []State }

func (i *managerInventory) ReadBackend(context.Context, string) (State, error) {
	if len(i.states) == 0 {
		return State{}, errors.New("unexpected inventory read")
	}
	state := i.states[0]
	i.states = i.states[1:]
	return state, nil
}

type managerServices struct{ operations []security.ServiceOperation }

func (s *managerServices) Change(_ context.Context, request security.ServiceRequest) error {
	s.operations = append(s.operations, request.Operation)
	return nil
}

func TestAddsAndVerifiesUFWRule(t *testing.T) {
	rule := security.FirewallRule{Kind: "port", Target: "443", Protocol: "tcp", Direction: "in", Source: "10.0.0.0/8", Action: "allow"}
	commands := &managerRunner{}
	manager := NewManager(commands, &managerInventory{states: []State{{Backend: "ufw", Installed: true, Active: true}, {Backend: "ufw", Installed: true, Active: true, Rules: []Rule{{Kind: "port", Target: "443", Protocol: "tcp", Direction: "in", Source: "10.0.0.0/8", Action: "allow"}}}}}, &managerServices{})
	err := manager.Change(context.Background(), security.FirewallRequest{Version: security.APIVersion, Operation: security.FirewallRuleAdd, Backend: "ufw", Rule: rule})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"/usr/bin/ufw", "allow", "in", "proto", "tcp", "from", "10.0.0.0/8", "to", "any", "port", "443"}}
	if !reflect.DeepEqual(commands.calls, want) {
		t.Fatalf("calls = %#v", commands.calls)
	}
}

func TestProtectsSSHRuleFromRemoval(t *testing.T) {
	manager := NewManager(&managerRunner{}, &managerInventory{states: []State{{Backend: "ufw", Installed: true, Active: true}}}, &managerServices{})
	err := manager.Change(context.Background(), security.FirewallRequest{Version: security.APIVersion, Operation: security.FirewallRuleRemove, Backend: "ufw", Rule: security.FirewallRule{Kind: "port", Target: "22", Protocol: "tcp", Action: "allow"}})
	if err == nil {
		t.Fatal("expected SSH removal to be refused")
	}
}

func TestSelectsFirewalldAndDisablesUFW(t *testing.T) {
	commands := &managerRunner{}
	services := &managerServices{}
	states := []State{
		{Backend: "firewalld", Installed: true, Backends: []string{"ufw", "firewalld"}},
		{Backend: "ufw", Installed: true, Active: true},
		{Backend: "firewalld", Installed: true, Active: true},
		{Backend: "ufw", Installed: true, Active: false},
	}
	manager := NewManager(commands, &managerInventory{states: states}, services)
	if err := manager.Change(context.Background(), security.FirewallRequest{Version: security.APIVersion, Operation: security.FirewallSelect, Backend: "firewalld"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(commands.calls, [][]string{{"/usr/bin/ufw", "disable"}}) {
		t.Fatalf("calls = %#v", commands.calls)
	}
	if !reflect.DeepEqual(services.operations, []security.ServiceOperation{security.ServiceEnable, security.ServiceStart}) {
		t.Fatalf("operations = %#v", services.operations)
	}
}
