package security

import (
	"errors"
	"regexp"
	"strings"
)

const FirmwareManagementAction = "io.github.nerdyslacker.sysentra.manage-firmware"

type FirmwareOperation string

const (
	FirmwareRefresh FirmwareOperation = "refresh"
	FirmwareUpdate  FirmwareOperation = "update"
)

var firmwareDevicePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)

type FirmwareRequest struct {
	Version     int               `json:"version"`
	Operation   FirmwareOperation `json:"operation"`
	DeviceID    string            `json:"deviceId,omitempty"`
	VersionName string            `json:"versionName,omitempty"`
}

func (r FirmwareRequest) Validate() error {
	if r.Version != APIVersion {
		return errors.New("unsupported API version")
	}
	switch r.Operation {
	case FirmwareRefresh:
		if r.DeviceID != "" || r.VersionName != "" {
			return errors.New("firmware refresh does not accept a device")
		}
	case FirmwareUpdate:
		if !firmwareDevicePattern.MatchString(r.DeviceID) || r.VersionName == "" || len(r.VersionName) > 128 || strings.ContainsAny(r.VersionName, "\n\r\x00") {
			return errors.New("firmware update requires a valid device and reviewed version")
		}
	default:
		return errors.New("unsupported firmware operation")
	}
	return nil
}
