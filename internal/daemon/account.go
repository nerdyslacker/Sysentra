package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/accounts"
	"sysentra/internal/security"
)

type AccountManager interface {
	Change(context.Context, security.AccountRequest) error
	Status(context.Context) ([]accounts.Status, error)
}
type AccountController struct {
	authorizer Authorizer
	manager    AccountManager
}

func NewAccountController(authorizer Authorizer, manager AccountManager) *AccountController {
	return &AccountController{authorizer: authorizer, manager: manager}
}

func (c *AccountController) Status(ctx context.Context, sender string) ([]accounts.Status, error) {
	if err := c.authorizer.Authorize(ctx, sender, security.AccountManagementAction, true); err != nil {
		return nil, err
	}
	return c.manager.Status(ctx)
}

func (c *AccountController) Change(ctx context.Context, sender, payload string) error {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return errors.New("invalid request size")
	}
	var request security.AccountRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return errors.New("invalid account request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("account request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if err := c.authorizer.Authorize(ctx, sender, security.AccountManagementAction, true); err != nil {
		return err
	}
	return c.manager.Change(ctx, request)
}
