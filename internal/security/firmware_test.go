package security

import "testing"

func TestFirmwareRequestValidation(t *testing.T) {
	valid := []FirmwareRequest{
		{Version: APIVersion, Operation: FirmwareRefresh},
		{Version: APIVersion, Operation: FirmwareUpdate, DeviceID: "2f0f4db2-1d4c-5f9e-a3ef-abc123", VersionName: "1.2.3"},
	}
	for _, request := range valid {
		if err := request.Validate(); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	invalid := []FirmwareRequest{
		{Version: APIVersion, Operation: FirmwareRefresh, DeviceID: "device"},
		{Version: APIVersion, Operation: FirmwareUpdate, DeviceID: "../../etc/passwd", VersionName: "1.2"},
		{Version: APIVersion, Operation: FirmwareUpdate, DeviceID: "device"},
	}
	for _, request := range invalid {
		if err := request.Validate(); err == nil {
			t.Fatalf("expected %#v to fail", request)
		}
	}
}
