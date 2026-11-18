package daemon

import (
	"context"
	"errors"
	"testing"

	"sysentra/internal/security"
	"sysentra/internal/services"
)

type serviceInventoryStub struct{ items []services.Service }

func (s serviceInventoryStub) List(context.Context) ([]services.Service, error) { return s.items, nil }

type serviceManagerStub struct {
	request security.ServiceRequest
	calls   int
}

func (s *serviceManagerStub) Change(_ context.Context, request security.ServiceRequest) error {
	s.calls++
	s.request = request
	return nil
}

func TestServiceControllerValidatesAndAuthorizesChange(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &serviceManagerStub{}
	controller := NewServiceController(authorizer, serviceInventoryStub{}, manager)
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"restart","name":"sshd"}`); err != nil {
		t.Fatal(err)
	}
	if manager.calls != 1 || manager.request.Name != "sshd" || authorizer.sender != ":1.42" || authorizer.action != security.ServiceManagementAction {
		t.Fatalf("manager=%#v sender=%q action=%q", manager, authorizer.sender, authorizer.action)
	}
}

func TestServiceControllerRejectsInvalidOrDeniedChange(t *testing.T) {
	manager := &serviceManagerStub{}
	authorizer := &authorizationStub{}
	controller := NewServiceController(authorizer, serviceInventoryStub{}, manager)
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"restart","name":"../sshd"}`); err == nil || manager.calls != 0 || authorizer.sender != "" {
		t.Fatalf("invalid request escaped validation: calls=%d sender=%q err=%v", manager.calls, authorizer.sender, err)
	}
	authorizer.err = errors.New("denied")
	if err := controller.Change(context.Background(), ":1.42", `{"version":1,"operation":"restart","name":"sshd"}`); err == nil || manager.calls != 0 {
		t.Fatalf("denied request reached manager: calls=%d err=%v", manager.calls, err)
	}
}
