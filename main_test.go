package main

import (
	"errors"
	"testing"

	appservice "sysentra/internal/app"
)

type sourceTogglerStub struct{ enabled bool }

func (s *sourceTogglerStub) SetAdditionalSourceEnabled(_ string, enabled bool) ([]appservice.AdditionalSourceView, error) {
	s.enabled = enabled
	return nil, nil
}

func TestFlatpakSourceDefaultsOffWhenExecutableIsMissing(t *testing.T) {
	source := &sourceTogglerStub{enabled: true}
	disableUnavailableFlatpak(source, func(string) (string, error) { return "", errors.New("missing") })
	if source.enabled {
		t.Fatal("Flatpak source remained enabled")
	}
}
