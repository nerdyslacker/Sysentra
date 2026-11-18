package accounts

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

type Manager struct {
	runner    runner.Runner
	inventory *Inventory
}

func NewManager(commandRunner runner.Runner) *Manager {
	return &Manager{runner: commandRunner, inventory: NewInventory()}
}

func (m *Manager) Change(ctx context.Context, request security.AccountRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	users, groups, err := m.inventory.List()
	if err != nil {
		return err
	}
	user, userFound := findUser(users, request.User)
	group, groupFound := findGroup(groups, request.Group)
	var command string
	var args []string
	switch request.Operation {
	case security.UserAdd:
		if userFound {
			return errors.New("user already exists")
		}
		args = []string{"--user-group"}
		if request.CreateHome {
			args = append(args, "--create-home")
		} else {
			args = append(args, "--no-create-home")
		}
		if request.Home != "" {
			args = append(args, "--home-dir", request.Home)
		}
		command, args = "/usr/bin/useradd", append(args, request.User)
	case security.UserRemove:
		if !userFound {
			return errors.New("user does not exist")
		}
		if user.UID < 1000 {
			return errors.New("system users cannot be removed")
		}
		command, args = "/usr/bin/userdel", []string{request.User}
	case security.GroupAdd:
		if groupFound {
			return errors.New("group already exists")
		}
		command, args = "/usr/bin/groupadd", []string{request.Group}
	case security.GroupRemove:
		if !groupFound {
			return errors.New("group does not exist")
		}
		if group.GID < 1000 {
			return errors.New("system groups cannot be removed")
		}
		command, args = "/usr/bin/groupdel", []string{request.Group}
	case security.MembershipAdd:
		if !userFound {
			return errors.New("user does not exist")
		}
		for _, name := range request.Groups {
			if _, found := findGroup(groups, name); !found {
				return fmt.Errorf("group %s does not exist", name)
			}
		}
		command, args = "/usr/bin/usermod", []string{"--append", "--groups", strings.Join(request.Groups, ","), request.User}
	case security.MembershipRemove:
		if !userFound || !groupFound {
			return errors.New("user or group does not exist")
		}
		if user.GID == group.GID {
			return errors.New("a user's primary group cannot be unassigned")
		}
		command, args = "/usr/bin/gpasswd", []string{"--delete", request.User, request.Group}
	}
	result, err := m.runner.Run(ctx, command, args...)
	if err != nil {
		return fmt.Errorf("change account: %w: %s", err, result.Stderr)
	}
	return nil
}

func findUser(items []User, name string) (User, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return User{}, false
}
func findGroup(items []Group, name string) (Group, bool) {
	for _, item := range items {
		if item.Name == name {
			return item, true
		}
	}
	return Group{}, false
}
