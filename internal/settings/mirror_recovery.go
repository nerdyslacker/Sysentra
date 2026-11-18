package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type MirrorRecovery struct {
	Enabled bool `json:"enabled"`
}

type MirrorRecoveryStore struct {
	path string
}

func NewMirrorRecoveryStore() *MirrorRecoveryStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return &MirrorRecoveryStore{path: filepath.Join(dir, "sysentra", "mirror-recovery.json")}
}

func (s *MirrorRecoveryStore) Read() (MirrorRecovery, error) {
	contents, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return MirrorRecovery{}, nil
	}
	if err != nil {
		return MirrorRecovery{}, fmt.Errorf("read mirror recovery preference: %w", err)
	}
	var value MirrorRecovery
	if err := json.Unmarshal(contents, &value); err != nil {
		return MirrorRecovery{}, fmt.Errorf("parse mirror recovery preference: %w", err)
	}
	return value, nil
}

func (s *MirrorRecoveryStore) Write(value MirrorRecovery) error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create Sysentra config directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".mirror-recovery-*")
	if err != nil {
		return fmt.Errorf("create mirror recovery preference: %w", err)
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
		return fmt.Errorf("replace mirror recovery preference: %w", err)
	}
	return nil
}
