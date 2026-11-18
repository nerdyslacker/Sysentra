package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

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
