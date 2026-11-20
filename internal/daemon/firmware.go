package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"sysentra/internal/firmware"
	"sysentra/internal/security"
)

type FirmwareManager interface {
	Change(context.Context, security.FirmwareRequest) (firmware.Result, error)
}

type FirmwareController struct {
	authorizer Authorizer
	manager    FirmwareManager
}

func NewFirmwareController(authorizer Authorizer, manager FirmwareManager) *FirmwareController {
	return &FirmwareController{authorizer: authorizer, manager: manager}
}

func (c *FirmwareController) Change(ctx context.Context, sender, payload string) (firmware.Result, error) {
	if len(payload) == 0 || len(payload) > MaxRequestBytes {
		return firmware.Result{}, errors.New("invalid request size")
	}
	var request security.FirmwareRequest
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return firmware.Result{}, errors.New("invalid firmware request")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return firmware.Result{}, errors.New("firmware request must contain one JSON value")
	}
	if err := request.Validate(); err != nil {
		return firmware.Result{}, err
	}
	if err := c.authorizer.Authorize(ctx, sender, security.FirmwareManagementAction, true); err != nil {
		return firmware.Result{}, err
	}
	return c.manager.Change(ctx, request)
}
