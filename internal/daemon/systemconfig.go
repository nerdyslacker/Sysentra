package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/security"
)

type SystemConfigManager interface {
	Change(security.SystemConfigRequest) error
}

type SystemConfigController struct {
	authorizer Authorizer
	manager    SystemConfigManager
}

func NewSystemConfigController(authorizer Authorizer, manager SystemConfigManager) *SystemConfigController {
	return &SystemConfigController{authorizer: authorizer, manager: manager}
}

func (c *SystemConfigController) Change(ctx context.Context, sender, payload string) error {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return errors.New("invalid request size")
	}
	var request security.SystemConfigRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return errors.New("invalid system configuration request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("system configuration request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := c.authorizer.Authorize(ctx, sender, security.SystemConfigManagementAction, true); err != nil {
		return err
	}
	return c.manager.Change(request)
}
