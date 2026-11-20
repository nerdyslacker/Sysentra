package daemon

import (
	"context"
	"testing"

	"sysentra/internal/accounts"
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

func (m *accountManagerStub) Status(context.Context) ([]accounts.Status, error) {
	return []accounts.Status{{Name: "alice", Password: "Locked", Processes: 2}}, nil
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

func TestAccountControllerAuthorizesStatus(t *testing.T) {
	authorizer := &authorizationStub{}
	items, err := NewAccountController(authorizer, &accountManagerStub{}).Status(context.Background(), ":1.42")
	if err != nil || len(items) != 1 || items[0].Password != "Locked" || authorizer.action != security.AccountManagementAction {
		t.Fatalf("items=%#v action=%q err=%v", items, authorizer.action, err)
	}
}
