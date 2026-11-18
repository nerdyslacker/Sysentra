package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type Manager struct {
	runner                     runner.Runner
	definitionsDir, enabledDir string
}

func NewManager(commandRunner runner.Runner, definitionsDir, enabledDir string) *Manager {
	return &Manager{runner: commandRunner, definitionsDir: definitionsDir, enabledDir: enabledDir}
}

func (m *Manager) Change(ctx context.Context, request security.ServiceRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	definition, err := m.definition(request.Name)
	if err != nil {
		return err
	}
	switch request.Operation {
	case security.ServiceEnable:
		return m.enable(request.Name, definition)
	case security.ServiceDisable:
		return m.disable(request.Name, definition)
	case security.ServiceStart, security.ServiceStop, security.ServiceRestart, security.ServiceReload, security.ServiceOnce, security.ServiceKill, security.ServicePause, security.ServiceContinue:
		active, err := m.active(request.Name, definition)
		if err != nil {
			return err
		}
		result, err := m.runner.Run(ctx, "/usr/bin/sv", "-w", strconv.Itoa(serviceTimeout(request)), string(request.Operation), active)
		if err != nil {
			return fmt.Errorf("%s service: %w: %s", request.Operation, err, result.Stderr)
		}
		return nil
	default:
		return errors.New("unsupported service operation")
	}
}

func serviceTimeout(request security.ServiceRequest) int {
	if request.TimeoutSeconds > 0 {
		return request.TimeoutSeconds
	}
	return 7
}

func (m *Manager) active(name, definition string) (string, error) {
	path := filepath.Join(m.enabledDir, name)
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", errors.New("service is not enabled in the active runsvdir")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil || target != definition {
		return "", errors.New("enabled service symlink has an unexpected target")
	}
	return path, nil
}

func (m *Manager) definition(name string) (string, error) {
	root, err := filepath.EvalSymlinks(m.definitionsDir)
	if err != nil {
		return "", fmt.Errorf("resolve service directory: %w", err)
	}
	path, err := filepath.EvalSymlinks(filepath.Join(m.definitionsDir, name))
	if err != nil {
		return "", fmt.Errorf("resolve service definition: %w", err)
	}
	if filepath.Dir(path) != root {
		return "", errors.New("service definition escapes the allowed directory")
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("service definition is not a directory")
	}
	return path, nil
}

func (m *Manager) enable(name, definition string) error {
	path := filepath.Join(m.enabledDir, name)
	if target, err := filepath.EvalSymlinks(path); err == nil {
		if target == definition {
			return nil
		}
		return errors.New("enabled service path already exists with a different target")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect enabled service: %w", err)
	}
	if err := os.Symlink(definition, path); err != nil {
		return fmt.Errorf("enable service: %w", err)
	}
	return nil
}

func (m *Manager) disable(name, definition string) error {
	path := filepath.Join(m.enabledDir, name)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return errors.New("enabled service path is not a removable symlink")
	}
	target, err := filepath.EvalSymlinks(path)
	if err != nil || target != definition {
		return errors.New("enabled service symlink has an unexpected target")
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("disable service: %w", err)
	}
	return nil
}
