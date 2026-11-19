package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type TurnstileManager struct {
	runner runner.Runner
	root   string
	user   *UserManager
}

func NewTurnstileManager(commandRunner runner.Runner, root string) *TurnstileManager {
	return &TurnstileManager{runner: commandRunner, root: root, user: NewUserManager(commandRunner, root)}
}

func (m *TurnstileManager) Change(ctx context.Context, request security.ServiceRequest) error {
	return m.user.Change(ctx, request)
}

func (m *TurnstileManager) Ready(operation string, names []string) ([]string, error) {
	if operation != "add" && operation != "remove" {
		return nil, errors.New("turnstile ready operation must be add or remove")
	}
	if len(names) == 0 {
		return nil, errors.New("at least one turnstile ready service is required")
	}
	for _, name := range names {
		if err := (security.ServiceRequest{Version: security.APIVersion, Operation: security.ServiceStart, Name: name}).Validate(); err != nil {
			return nil, err
		}
	}
	path := filepath.Join(m.root, "turnstile-ready", "conf")
	current, err := readReadyServices(path)
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(current)+len(names))
	for _, name := range current {
		set[name] = true
	}
	for _, name := range names {
		set[name] = operation == "add"
	}
	updated := make([]string, 0, len(set))
	for name, included := range set {
		if included {
			updated = append(updated, name)
		}
	}
	sort.Strings(updated)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return updated, atomicUserWrite(path, []byte(fmt.Sprintf("core_services=\"%s\"\n", strings.Join(updated, " "))))
}

func (m *TurnstileManager) Environment(ctx context.Context, values []string) error {
	if len(values) == 0 {
		return errors.New("at least one environment variable is required")
	}
	for _, value := range values {
		name, _, _ := strings.Cut(value, "=")
		if !environmentName.MatchString(name) || len(value) > 4096 || strings.ContainsRune(value, 0) {
			return fmt.Errorf("invalid environment variable %q", name)
		}
	}
	result, err := m.runner.Run(ctx, "/usr/bin/turnstile-update-runit-env", values...)
	if err != nil {
		return fmt.Errorf("update turnstile environment: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func readReadyServices(path string) ([]string, error) {
	contents, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(contents), "\n") {
		if value, found := strings.CutPrefix(strings.TrimSpace(line), "core_services="); found {
			return strings.Fields(strings.Trim(value, `"'`)), nil
		}
	}
	return []string{}, nil
}

func atomicUserWrite(path string, contents []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".sysentra-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(contents); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
