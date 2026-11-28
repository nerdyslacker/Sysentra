package app

import (
	"context"
	"errors"
	"sort"
	"strings"

	"sysentra/internal/environment"
	"sysentra/internal/security"
)

type environmentSystemClient interface {
	ChangeSystemConfig(context.Context, security.SystemConfigRequest) error
}

type EnvironmentVariableView struct {
	Name  string `json:"name"`
	Value string `json:"value"`
	Scope string `json:"scope"`
	File  string `json:"file"`
}

type EnvironmentService struct {
	manager    *environment.Manager
	system     environmentSystemClient
	globalPath string
}

func NewEnvironmentService(manager *environment.Manager, system environmentSystemClient) *EnvironmentService {
	return &EnvironmentService{manager: manager, system: system, globalPath: "/etc/environment"}
}

func (s *EnvironmentService) ListEnvironmentVariables() ([]EnvironmentVariableView, error) {
	global, err := environment.ListGlobal(s.globalPath)
	if err != nil {
		return nil, err
	}
	shell, err := s.manager.List()
	if err != nil {
		return nil, err
	}
	variables := append(global, shell...)
	views := make([]EnvironmentVariableView, len(variables))
	for index, variable := range variables {
		views[index] = EnvironmentVariableView(variable)
	}
	sort.Slice(views, func(i, j int) bool {
		if views[i].Scope == views[j].Scope {
			return views[i].Name < views[j].Name
		}
		return views[i].Scope < views[j].Scope
	})
	return views, nil
}

func (s *EnvironmentService) ChangeEnvironmentVariable(scope, operation, name, oldName, value, file string) ([]EnvironmentVariableView, error) {
	scope = strings.ToLower(strings.TrimSpace(scope))
	operation = strings.ToLower(strings.TrimSpace(operation))
	if scope == "global" {
		if s.system == nil {
			return nil, errors.New("privileged system configuration service is unavailable")
		}
		op := security.EnvironmentGlobalSet
		if operation == "remove" {
			op = security.EnvironmentGlobalRemove
		} else if operation != "set" {
			return nil, errors.New("unsupported environment variable operation")
		}
		request := security.SystemConfigRequest{Version: security.APIVersion, Operation: op, Value: name, OldValue: oldName, Path: value}
		if err := request.Validate(); err != nil {
			return nil, err
		}
		if err := s.system.ChangeSystemConfig(context.Background(), request); err != nil {
			return nil, err
		}
	} else if err := s.manager.Change(scope, operation, name, oldName, value, file); err != nil {
		return nil, err
	}
	return s.ListEnvironmentVariables()
}
