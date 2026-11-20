package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"sysentra/internal/security"
)

func (s *SystemService) ListAccountHistory() ([]AccountHistoryView, error) {
	s.accountHistoryMu.Lock()
	defer s.accountHistoryMu.Unlock()
	if s.accountHistoryPath == "" {
		return []AccountHistoryView{}, nil
	}
	contents, err := os.ReadFile(s.accountHistoryPath)
	if os.IsNotExist(err) {
		return []AccountHistoryView{}, nil
	}
	if err != nil {
		return nil, err
	}
	var history []AccountHistoryView
	if err := json.Unmarshal(contents, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func (s *SystemService) recordAccountHistory(request security.AccountRequest) {
	if s.accountHistoryPath == "" {
		return
	}
	s.accountHistoryMu.Lock()
	defer s.accountHistoryMu.Unlock()
	var history []AccountHistoryView
	if contents, err := os.ReadFile(s.accountHistoryPath); err == nil {
		_ = json.Unmarshal(contents, &history)
	}
	target := request.User
	if target == "" {
		target = request.Group
	}
	history = append([]AccountHistoryView{{Time: time.Now().Format(time.RFC3339), Operation: string(request.Operation), Target: target}}, history...)
	if len(history) > 100 {
		history = history[:100]
	}
	contents, err := json.MarshalIndent(history, "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(s.accountHistoryPath), 0o700) != nil {
		return
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.accountHistoryPath), ".accounts-*")
	if err != nil {
		return
	}
	name := temporary.Name()
	defer os.Remove(name)
	if temporary.Chmod(0o600) != nil || func() error { _, err := temporary.Write(contents); return err }() != nil || temporary.Sync() != nil || temporary.Close() != nil {
		_ = temporary.Close()
		return
	}
	_ = os.Rename(name, s.accountHistoryPath)
}
