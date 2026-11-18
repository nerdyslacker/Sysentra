package services

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"sysentra/internal/runner"
)

const (
	DefinitionsDir = "/etc/sv"
	EnabledDir     = "/etc/runit/runsvdir/default"
)

type Service struct {
	Name    string
	Enabled bool
	State   string
	PID     int
	Command string
	Time    string
}

type Inventory struct {
	runner                     runner.Runner
	definitionsDir, enabledDir string
	procDir                    string
}

func NewInventory(commandRunner runner.Runner, definitionsDir, enabledDir, procDir string) *Inventory {
	return &Inventory{runner: commandRunner, definitionsDir: definitionsDir, enabledDir: enabledDir, procDir: procDir}
}

func (i *Inventory) List(ctx context.Context) ([]Service, error) {
	items, err := ListRunitServices(i.definitionsDir, i.enabledDir)
	if err != nil || len(items) == 0 {
		return items, err
	}
	return i.populate(ctx, items), nil
}

func (i *Inventory) populate(ctx context.Context, items []Service) []Service {
	for index := range items {
		if !items[index].Enabled {
			items[index].State = "Stopped"
			continue
		}
		result, _ := i.runner.Run(ctx, "/usr/bin/sv", "status", i.enabledDir+"/"+items[index].Name)
		output := append(append([]byte{}, result.Stdout...), result.Stderr...)
		if runtime, ok := parseStatus(string(output))[items[index].Name]; ok {
			items[index].State, items[index].PID, items[index].Time = runtime.State, runtime.PID, runtime.Time
			if runtime.PID > 0 {
				if command, err := os.ReadFile(i.procDir + "/" + strconv.Itoa(runtime.PID) + "/cmdline"); err == nil {
					if executable := strings.SplitN(string(command), "\x00", 2)[0]; executable != "" {
						items[index].Command = filepath.Base(executable)
					}
				}
			}
		}
		if items[index].State == "" {
			items[index].State = "Unknown"
		}
	}
	return items
}

type runtimeStatus struct {
	State string
	PID   int
	Time  string
}

func parseStatus(output string) map[string]runtimeStatus {
	statuses := map[string]runtimeStatus{}
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		name := strings.TrimSuffix(parts[1][strings.LastIndex(parts[1], "/")+1:], ":")
		status := runtimeStatus{State: strings.TrimSuffix(parts[0], ":")}
		switch status.State {
		case "run":
			status.State = "Running"
		case "down":
			status.State = "Stopped"
		default:
			status.State = strings.ToUpper(status.State[:1]) + status.State[1:]
		}
		for index, part := range parts {
			if part == "(pid" && index+1 < len(parts) {
				status.PID, _ = strconv.Atoi(strings.TrimSuffix(parts[index+1], ")"))
			}
			elapsed := strings.TrimRight(part, ",;")
			if strings.HasSuffix(elapsed, "s") && strings.TrimSuffix(elapsed, "s") != "" {
				if _, err := strconv.Atoi(strings.TrimSuffix(elapsed, "s")); err == nil {
					status.Time = elapsed
				}
			}
		}
		statuses[name] = status
	}
	return statuses
}

func ListRunitServices(definitionsDir, enabledDir string) ([]Service, error) {
	definitions, err := os.ReadDir(definitionsDir)
	if err != nil {
		return nil, err
	}
	enabled := map[string]bool{}
	if entries, err := os.ReadDir(enabledDir); err == nil {
		for _, entry := range entries {
			enabled[entry.Name()] = true
		}
	}
	result := make([]Service, 0, len(definitions))
	for _, entry := range definitions {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			result = append(result, Service{Name: entry.Name(), Enabled: enabled[entry.Name()]})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
