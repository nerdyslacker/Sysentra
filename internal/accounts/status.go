package accounts

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Status struct {
	Name      string `json:"name"`
	Password  string `json:"password"`
	Processes int    `json:"processes"`
	Sessions  int    `json:"sessions"`
}

func (m *Manager) Status(ctx context.Context) ([]Status, error) {
	users, _, err := m.inventory.List()
	if err != nil {
		return nil, err
	}
	passwords, err := passwordStates(m.shadow)
	if err != nil {
		return nil, err
	}
	processes := processCounts(m.proc)
	sessions := map[string]int{}
	if result, err := m.runner.Run(ctx, "/usr/bin/who"); err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(result.Stdout)))
		for scanner.Scan() {
			if fields := strings.Fields(scanner.Text()); len(fields) > 0 {
				sessions[fields[0]]++
			}
		}
	}
	statuses := make([]Status, len(users))
	for index, user := range users {
		password := passwords[user.Name]
		if password == "" {
			password = "Unknown"
		}
		statuses[index] = Status{Name: user.Name, Password: password, Processes: processes[user.UID], Sessions: sessions[user.Name]}
	}
	return statuses, nil
}

func passwordStates(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read password state: %w", err)
	}
	defer file.Close()
	states := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), ":")
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid shadow entry")
		}
		switch {
		case fields[1] == "":
			states[fields[0]] = "No password"
		case strings.HasPrefix(fields[1], "!") || strings.HasPrefix(fields[1], "*"):
			states[fields[0]] = "Locked"
		default:
			states[fields[0]] = "Password set"
		}
	}
	return states, scanner.Err()
}

func processCounts(root string) map[int]int {
	counts := map[int]int{}
	entries, _ := os.ReadDir(root)
	for _, entry := range entries {
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		file, err := os.Open(filepath.Join(root, entry.Name(), "status"))
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if !strings.HasPrefix(scanner.Text(), "Uid:") {
				continue
			}
			fields := strings.Fields(scanner.Text())
			if len(fields) > 1 {
				if uid, err := strconv.Atoi(fields[1]); err == nil {
					counts[uid]++
				}
			}
			break
		}
		file.Close()
	}
	return counts
}
