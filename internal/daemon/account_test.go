package daemon

import (
	"context"
	"testing"

	"sysentra/internal/security"
)

type accountManagerStub struct {
	calls   int
	request security.AccountRequest
}

func (m *accountManagerStub) Change(_ context.Context, request security.AccountRequest) error {
	m.calls++
	m.request = request
	return nil
}

func TestAccountControllerValidatesAndAuthorizesChange(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &accountManagerStub{}
	controller := NewAccountController(authorizer, manager)
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"membership-add","user":"alice","groups":["wheel","audio"]}`); err != nil {
		t.Fatal(err)
	}
	if manager.calls != 1 || manager.request.User != "alice" || authorizer.action != security.AccountManagementAction {
		t.Fatalf("manager=%#v action=%q", manager, authorizer.action)
	}
}
