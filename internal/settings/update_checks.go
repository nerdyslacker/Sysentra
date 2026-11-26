package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	MinUpdateInterval = 15
	MaxUpdateInterval = 7 * 24 * 60
)

type UpdateChecks struct {
	Scheduled       bool `json:"scheduled"`
	Notifications   bool `json:"notifications"`
	IntervalMinutes int  `json:"intervalMinutes"`
}

type UpdateChecksStore struct{ path string }

func NewUpdateChecksStore() *UpdateChecksStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return &UpdateChecksStore{path: filepath.Join(dir, "sysentra", "update-checks.json")}
}

func defaultUpdateChecks() UpdateChecks {
	return UpdateChecks{Scheduled: true, Notifications: true, IntervalMinutes: 360}
}

func (s *UpdateChecksStore) Read() (UpdateChecks, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return defaultUpdateChecks(), nil
	}
	if err != nil {
		return UpdateChecks{}, fmt.Errorf("read update-check preferences: %w", err)
	}
	var value UpdateChecks
	if err := json.Unmarshal(contents, &value); err != nil {
		return UpdateChecks{}, fmt.Errorf("parse update-check preferences: %w", err)
	}
	if err := validateUpdateChecks(value); err != nil {
		return UpdateChecks{}, err
	}
	return value, nil
}

func (s *UpdateChecksStore) Write(value UpdateChecks) error {
	if err := validateUpdateChecks(value); err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Sysentra config directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".update-checks-*")
	if err != nil {
		return fmt.Errorf("create update-check preferences: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err := json.NewEncoder(temporary).Encode(value); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, s.path); err != nil {
		return fmt.Errorf("replace update-check preferences: %w", err)
	}
	return nil
}

func validateUpdateChecks(value UpdateChecks) error {
	if value.IntervalMinutes < MinUpdateInterval || value.IntervalMinutes > MaxUpdateInterval {
		return fmt.Errorf("update-check interval must be between %d and %d minutes", MinUpdateInterval, MaxUpdateInterval)
	}
	return nil
}
