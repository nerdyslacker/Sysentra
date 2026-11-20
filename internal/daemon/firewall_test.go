package daemon

import (
	"context"
	"testing"

	"sysentra/internal/security"
)

type firewallManagerStub struct{ request security.FirewallRequest }

func (m *firewallManagerStub) Change(_ context.Context, request security.FirewallRequest) error {
	m.request = request
	return nil
}

func TestFirewallControllerValidatesAndAuthorizes(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &firewallManagerStub{}
	payload := `{"version":1,"operation":"rule-add","backend":"ufw","rule":{"kind":"port","target":"443","protocol":"tcp","action":"allow"}}`
	if err := NewFirewallController(authorizer, manager).Change(context.Background(), ":1.42", payload); err != nil {
		t.Fatal(err)
	}
	if manager.request.Rule.Target != "443" || authorizer.action != security.FirewallManagementAction {
		t.Fatalf("request=%#v action=%q", manager.request, authorizer.action)
	}
}
