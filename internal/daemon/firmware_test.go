package daemon

import (
	"context"
	"testing"

	"sysentra/internal/firmware"
	"sysentra/internal/security"
)

type firmwareManagerStub struct{ request security.FirmwareRequest }

func (m *firmwareManagerStub) Change(_ context.Context, request security.FirmwareRequest) (firmware.Result, error) {
	m.request = request
	return firmware.Result{Diagnostics: []string{"done"}}, nil
}

func TestFirmwareControllerValidatesAndAuthorizes(t *testing.T) {
	authorizer := &authorizationStub{}
	manager := &firmwareManagerStub{}
	result, err := NewFirmwareController(authorizer, manager).Change(context.Background(), ":1.42", `{"version":1,"operation":"update","deviceId":"device-1","versionName":"1.2.3"}`)
	if err != nil || manager.request.DeviceID != "device-1" || len(result.Diagnostics) != 1 || authorizer.action != security.FirmwareManagementAction {
		t.Fatalf("result=%#v request=%#v action=%q err=%v", result, manager.request, authorizer.action, err)
	}
}
