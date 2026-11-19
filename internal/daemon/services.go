package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/security"
	"sysentra/internal/services"
)

type ServiceInventory interface {
	List(context.Context) ([]services.Service, error)
	Details(string) (services.Details, error)
}

type ServiceManager interface {
	Change(context.Context, security.ServiceRequest) error
}

type ServiceController struct {
	authorizer Authorizer
	inventory  ServiceInventory
	manager    ServiceManager
}

func NewServiceController(authorizer Authorizer, inventory ServiceInventory, manager ServiceManager) *ServiceController {
	return &ServiceController{authorizer: authorizer, inventory: inventory, manager: manager}
}

func (s *ServiceController) List(ctx context.Context) ([]services.Service, error) {
	return s.inventory.List(ctx)
}

func (s *ServiceController) Details(ctx context.Context, sender, name string) (services.Details, error) {
	request := security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceStart, Name: name}
	if err := request.Validate(); err != nil {
		return services.Details{}, err
	}
	if err := s.authorizer.Authorize(ctx, sender, security.ServiceManagementAction, true); err != nil {
		return services.Details{}, err
	}
	return s.inventory.Details(name)
}

func (s *ServiceController) Change(ctx context.Context, sender, payload string) error {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return errors.New("invalid request size")
	}
	var request security.ServiceRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return errors.New("invalid service request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("service request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := s.authorizer.Authorize(ctx, sender, security.ServiceManagementAction, true); err != nil {
		return err
	}
	return s.manager.Change(ctx, request)
}
