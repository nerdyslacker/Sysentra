package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/security"
)

type XBPSRuleManager interface {
	Change(security.XBPSRuleRequest) error
}

type XBPSConfigController struct {
	authorizer Authorizer
	manager    XBPSRuleManager
}

func NewXBPSConfigController(authorizer Authorizer, manager XBPSRuleManager) *XBPSConfigController {
	return &XBPSConfigController{authorizer: authorizer, manager: manager}
}

func (c *XBPSConfigController) Change(ctx context.Context, sender, payload string) error {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return errors.New("invalid request size")
	}
	var request security.XBPSRuleRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return errors.New("invalid XBPS configuration request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("XBPS configuration request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := c.authorizer.Authorize(ctx, sender, security.XBPSConfigManagementAction, true); err != nil {
		return err
	}
	return c.manager.Change(request)
}
