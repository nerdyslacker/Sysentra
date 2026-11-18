package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"sysentra/internal/runner"
)

type TransactionHistoryView struct {
	RequestID string `json:"requestId"`
	Operation string `json:"operation"`
	Status    string `json:"status"`
	Message   string `json:"message"`
	Time      string `json:"time"`
}

func NewPersistentPackageTransactionService(client packageTransactionClient, commandRunner runner.Runner, planners ...flatpakTransactionPlanner) *PackageTransactionService {
	service := NewPackageTransactionService(client, commandRunner, planners...)
	if configDir, err := os.UserConfigDir(); err == nil {
		service.historyPath = filepath.Join(configDir, "sysentra", "transactions.json")
	}
	return service
}

func (s *PackageTransactionService) ListTransactionHistory() ([]TransactionHistoryView, error) {
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	if s.historyPath == "" {
		return []TransactionHistoryView{}, nil
	}
	contents, err := os.ReadFile(s.historyPath)
	if os.IsNotExist(err) {
		return []TransactionHistoryView{}, nil
	}
	if err != nil {
		return nil, err
	}
	var history []TransactionHistoryView
	if err := json.Unmarshal(contents, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func (s *PackageTransactionService) recordTransaction(requestID, operation, status, message string) {
	if s.historyPath == "" {
		return
	}
	s.historyMu.Lock()
	defer s.historyMu.Unlock()
	var history []TransactionHistoryView
	if contents, err := os.ReadFile(s.historyPath); err == nil {
		_ = json.Unmarshal(contents, &history)
	}
	history = append([]TransactionHistoryView{{RequestID: requestID, Operation: operation, Status: status, Message: message, Time: time.Now().Format(timeLayout)}}, history...)
	if len(history) > 100 {
		history = history[:100]
	}
	contents, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return
	}
	if os.MkdirAll(filepath.Dir(s.historyPath), 0o700) != nil {
		return
	}
	temporary, err := os.CreateTemp(filepath.Dir(s.historyPath), ".transactions-*")
	if err != nil {
		return
	}
	name := temporary.Name()
	defer os.Remove(name)
	if temporary.Chmod(0o600) != nil || func() error { _, err := temporary.Write(contents); return err }() != nil || temporary.Sync() != nil || temporary.Close() != nil {
		_ = temporary.Close()
		return
	}
	_ = os.Rename(name, s.historyPath)
}
