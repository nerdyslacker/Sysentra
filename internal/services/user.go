package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type UserInventory struct {
	runner  runner.Runner
	root    string
	procDir string
}

func NewUserInventory(commandRunner runner.Runner, root, procDir string) *UserInventory {
	return &UserInventory{runner: commandRunner, root: root, procDir: procDir}
}

func (i *UserInventory) List(ctx context.Context) ([]Service, error) {
	items, err := ListUserRunitServices(i.root)
	if err != nil || len(items) == 0 {
		return items, err
	}
	inventory := Inventory{runner: i.runner, enabledDir: i.root, procDir: i.procDir}
	return inventory.populate(ctx, items), nil
}

func (i *UserInventory) Details(name string) (Details, error) {
	if details, err := serviceDetails(i.root, name, false); err == nil {
		return details, nil
	}
	return serviceDetails(filepath.Join(i.root, ".disabled"), name, false)
}

func ListUserRunitServices(root string) ([]Service, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return []Service{}, nil
	}
	if err != nil {
		return nil, err
	}
	items := make([]Service, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".disabled" || (!entry.IsDir() && entry.Type()&os.ModeSymlink == 0) {
			continue
		}
		items = append(items, Service{Name: entry.Name(), Enabled: true})
	}
	disabled, err := os.ReadDir(filepath.Join(root, ".disabled"))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range disabled {
		if entry.IsDir() {
			items = append(items, Service{Name: entry.Name()})
		}
	}
	sort.Slice(items, func(left, right int) bool { return items[left].Name < items[right].Name })
	return items, nil
}

type UserManager struct {
	runner runner.Runner
	root   string
}

func NewUserManager(commandRunner runner.Runner, root string) *UserManager {
	return &UserManager{runner: commandRunner, root: root}
}

func (m *UserManager) Change(ctx context.Context, request security.ServiceRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	switch request.Operation {
	case security.ServiceAdd:
		return createService(filepath.Join(m.root, ".disabled"), request.Name, request.Command, request.Arguments)
	case security.ServiceEnable:
		return m.move(request.Name, true)
	case security.ServiceDisable:
		path, err := m.servicePath(request.Name, false)
		if err != nil {
			return err
		}
		if result, err := m.runner.Run(ctx, "/usr/bin/sv", "-w", strconv.Itoa(serviceTimeout(request)), "down", path); err != nil {
			return fmt.Errorf("stop service before disabling: %w: %s", err, result.Stderr)
		}
		return m.move(request.Name, false)
	case security.ServiceStart, security.ServiceStop, security.ServiceRestart, security.ServiceReload, security.ServiceOnce, security.ServiceKill, security.ServicePause, security.ServiceContinue:
		path, err := m.servicePath(request.Name, false)
		if err != nil {
			return err
		}
		result, err := m.runner.Run(ctx, "/usr/bin/sv", "-w", strconv.Itoa(serviceTimeout(request)), string(request.Operation), path)
		if err != nil {
			return fmt.Errorf("%s user service: %w: %s", request.Operation, err, result.Stderr)
		}
		return nil
	default:
		return errors.New("unsupported user service operation")
	}
}

func createService(root, name, command string, arguments []string) error {
	info, err := os.Stat(command)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return errors.New("service command is not an executable regular file")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	target := filepath.Join(root, name)
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		return errors.New("service already exists")
	}
	if err := os.Mkdir(target, 0o755); err != nil {
		return err
	}
	values := append([]string{command}, arguments...)
	for index := range values {
		values[index] = shellQuote(values[index])
	}
	run := filepath.Join(target, "run")
	if err := os.WriteFile(run, []byte("#!/bin/sh\nexec "+strings.Join(values, " ")+"\n"), 0o755); err != nil {
		_ = os.Remove(run)
		_ = os.Remove(target)
		return err
	}
	return nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (m *UserManager) servicePath(name string, disabled bool) (string, error) {
	root := m.root
	if disabled {
		root = filepath.Join(root, ".disabled")
	}
	path := filepath.Join(root, name)
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("user service is not a real directory")
	}
	return path, nil
}

func (m *UserManager) move(name string, enable bool) error {
	fromDisabled := enable
	from, err := m.servicePath(name, fromDisabled)
	if err != nil {
		return err
	}
	toRoot := filepath.Join(m.root, ".disabled")
	if enable {
		toRoot = m.root
	} else if err := os.MkdirAll(toRoot, 0o700); err != nil {
		return fmt.Errorf("create disabled service directory: %w", err)
	}
	to := filepath.Join(toRoot, name)
	if _, err := os.Lstat(to); !os.IsNotExist(err) {
		return errors.New("user service destination already exists")
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("move user service: %w", err)
	}
	return nil
}
