package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/security"
)

type FirewallManager interface {
	Change(context.Context, security.FirewallRequest) error
}

type FirewallController struct {
	authorizer Authorizer
	manager    FirewallManager
}

func NewFirewallController(authorizer Authorizer, manager FirewallManager) *FirewallController {
	return &FirewallController{authorizer: authorizer, manager: manager}
}

func (c *FirewallController) Change(ctx context.Context, sender, payload string) error {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return errors.New("invalid request size")
	}
	var request security.FirewallRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return errors.New("invalid firewall request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("firewall request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := c.authorizer.Authorize(ctx, sender, security.FirewallManagementAction, true); err != nil {
		return err
	}
	return c.manager.Change(ctx, request)
}
