package firmware

import (
	"context"
	"fmt"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type Result struct {
	Diagnostics []string `json:"diagnostics"`
}

type Manager struct{ runner runner.Runner }

func NewManager(commandRunner runner.Runner) *Manager { return &Manager{runner: commandRunner} }

func (m *Manager) Change(ctx context.Context, request security.FirmwareRequest) (Result, error) {
	if err := request.Validate(); err != nil {
		return Result{}, err
	}
	args := []string{"refresh", "--assume-yes"}
	if request.Operation == security.FirmwareUpdate {
		args = []string{"install", request.DeviceID, request.VersionName, "--assume-yes"}
	}
	output, err := m.runner.Run(ctx, "/usr/bin/fwupdmgr", args...)
	result := Result{Diagnostics: outputLines(output)}
	if err != nil {
		diagnostic := strings.Join(result.Diagnostics, ": ")
		if diagnostic != "" {
			return result, fmt.Errorf("fwupdmgr %s: %s: %w", request.Operation, diagnostic, err)
		}
		return result, fmt.Errorf("fwupdmgr %s: %w", request.Operation, err)
	}
	return result, nil
}

func outputLines(result runner.Result) []string {
	text := strings.TrimSpace(string(append(append([]byte{}, result.Stdout...), result.Stderr...)))
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}
