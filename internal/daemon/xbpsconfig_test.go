package daemon

import (
	"context"
	"testing"

	"sysentra/internal/security"
)

type ruleManagerStub struct {
	request security.XBPSRuleRequest
	calls   int
}

func (m *ruleManagerStub) Change(request security.XBPSRuleRequest) error {
	m.calls++
	m.request = request
	return nil
}

func TestXBPSConfigControllerValidatesAndAuthorizesChange(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &ruleManagerStub{}
	controller := NewXBPSConfigController(authorizer, manager)
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"add","kind":"ignorepkg","value":"linux6.12"}`); err != nil {
		t.Fatal(err)
	}
	if manager.calls != 1 || manager.request.Value != "linux6.12" || authorizer.action != security.XBPSConfigManagementAction {
		t.Fatalf("manager=%#v action=%q", manager, authorizer.action)
	}
}
