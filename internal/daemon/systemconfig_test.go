package daemon

import (
	"context"
	"testing"

	"sysentra/internal/security"
)

type systemConfigManagerStub struct {
	request security.SystemConfigRequest
	calls   int
}

func (m *systemConfigManagerStub) Change(request security.SystemConfigRequest) error {
	m.calls++
	m.request = request
	return nil
}

func TestSystemConfigControllerValidatesAndAuthorizesChange(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &systemConfigManagerStub{}
	controller := NewSystemConfigController(authorizer, manager)
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"mirror-use","value":"https://repo.example.org/"}`); err != nil {
		t.Fatal(err)
	}
	if manager.calls != 1 || manager.request.Operation != security.MirrorUse || authorizer.action != security.SystemConfigManagementAction {
		t.Fatalf("manager=%#v action=%q", manager, authorizer.action)
	}
}

func TestSystemConfigControllerRejectsUnknownFieldsBeforeAuthorization(t *testing.T) {
	authorizer := &authorizationStub{}
	controller := NewSystemConfigController(authorizer, &systemConfigManagerStub{})
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"mirror-reset","command":"rm"}`); err == nil || authorizer.sender != "" {
		t.Fatalf("invalid request reached authorizer: sender=%q err=%v", authorizer.sender, err)
	}
}
