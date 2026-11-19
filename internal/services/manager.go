package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type Manager struct {
	runner                     runner.Runner
	definitionsDir, enabledDir string
	lookupUser                 func(string) (*user.User, error)
}

func NewManager(commandRunner runner.Runner, definitionsDir, enabledDir string) *Manager {
	return &Manager{runner: commandRunner, definitionsDir: definitionsDir, enabledDir: enabledDir, lookupUser: user.Lookup}
}

func (m *Manager) Change(ctx context.Context, request security.ServiceRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	if request.Operation == security.ServiceInitUser {
		return m.initUser(request.Name)
	}
	if request.Operation == security.ServiceAdd {
		return createService(m.definitionsDir, request.Name, request.Command, request.Arguments)
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

func (m *Manager) initUser(name string) error {
	account, err := m.lookupUser(name)
	if err != nil {
		return fmt.Errorf("look up user: %w", err)
	}
	if !filepath.IsAbs(account.HomeDir) || strings.ContainsAny(account.HomeDir, "\n\r'") {
		return errors.New("user has an unsafe home directory")
	}
	serviceName := "runsvdir-" + name
	definition := filepath.Join(m.definitionsDir, serviceName)
	run := filepath.Join(definition, "run")
	userServices := filepath.Join(account.HomeDir, "service")
	script := []byte(fmt.Sprintf("#!/bin/sh\nexec chpst -u '%s' runsvdir '%s'\n", name, userServices))
	if err := os.MkdirAll(definition, 0o755); err != nil {
		return err
	}
	if existing, err := os.ReadFile(run); err == nil && string(existing) != string(script) {
		return errors.New("existing user runsvdir service is not managed by Sysentra")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.WriteFile(run, script, 0o755); err != nil {
		return err
	}
	if err := os.Chmod(run, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(userServices, 0o700); err != nil {
		return err
	}
	uid, uidErr := strconv.Atoi(account.Uid)
	gid, gidErr := strconv.Atoi(account.Gid)
	if uidErr != nil || gidErr != nil {
		return errors.New("user has invalid numeric IDs")
	}
	if err := os.Chown(userServices, uid, gid); err != nil {
		return err
	}
	return m.enable(serviceName, definition)
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
