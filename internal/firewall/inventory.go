package firewall

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	"sysentra/internal/runner"
)

type Rule struct {
	Target string
	Action string
	Source string
}

type State struct {
	Backend   string
	Installed bool
	Active    bool
	Unknown   bool
	Policy    string
	Rules     []Rule
	Others    []string
}

type Inventory struct {
	runner runner.Runner
	lookup func(string) (string, error)
}

func NewInventory(commandRunner runner.Runner) *Inventory {
	return &Inventory{runner: commandRunner, lookup: exec.LookPath}
}

func newInventory(commandRunner runner.Runner, lookup func(string) (string, error)) *Inventory {
	return &Inventory{runner: commandRunner, lookup: lookup}
}

func (i *Inventory) Read(ctx context.Context) (State, error) {
	_, ufwErr := i.lookup("ufw")
	_, firewalldErr := i.lookup("firewall-cmd")
	if ufwErr == nil {
		state := i.readUFW(ctx)
		if firewalldErr == nil {
			state.Others = []string{"firewalld"}
		}
		return state, nil
	}
	if firewalldErr == nil {
		return i.readFirewalld(ctx), nil
	}
	return State{}, nil
}

func (i *Inventory) readUFW(ctx context.Context) State {
	state := State{Backend: "ufw", Installed: true, Rules: []Rule{}}
	result, err := i.runner.Run(ctx, "ufw", "status", "verbose")
	if err != nil && len(result.Stdout) == 0 {
		state.Unknown = true
		return state
	}
	for _, line := range strings.Split(string(result.Stdout), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Status:"):
			state.Active = strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:")) == "active"
		case strings.HasPrefix(trimmed, "Default:"):
			state.Policy = strings.TrimSpace(strings.TrimPrefix(trimmed, "Default:"))
		case strings.HasPrefix(trimmed, "To"), strings.HasPrefix(trimmed, "--"):
		default:
			if rule, ok := parseUFWRule(trimmed); ok {
				state.Rules = append(state.Rules, rule)
			}
		}
	}
	return state
}

func parseUFWRule(line string) (Rule, bool) {
	parts := strings.FieldsFunc(line, func(r rune) bool { return r == '\t' })
	if len(parts) < 3 {
		parts = splitColumns(line)
	}
	if len(parts) < 3 {
		return Rule{}, false
	}
	return Rule{Target: strings.TrimSpace(parts[0]), Action: strings.TrimSpace(parts[1]), Source: strings.TrimSpace(parts[2])}, true
}

func splitColumns(line string) []string {
	var columns []string
	for _, value := range strings.Split(strings.TrimSpace(line), "  ") {
		if value = strings.TrimSpace(value); value != "" {
			columns = append(columns, value)
		}
	}
	return columns
}

func (i *Inventory) readFirewalld(ctx context.Context) State {
	state := State{Backend: "firewalld", Installed: true, Rules: []Rule{}}
	result, err := i.runner.Run(ctx, "firewall-cmd", "--state")
	if err != nil {
		return state
	}
	state.Active = strings.TrimSpace(string(result.Stdout)) == "running"
	if !state.Active {
		return state
	}
	zoneResult, err := i.runner.Run(ctx, "firewall-cmd", "--get-default-zone")
	if err != nil {
		state.Unknown = true
		return state
	}
	zone := strings.TrimSpace(string(zoneResult.Stdout))
	state.Policy = "default zone: " + zone
	list, err := i.runner.Run(ctx, "firewall-cmd", "--zone", zone, "--list-all")
	if err != nil {
		state.Unknown = true
		return state
	}
	scanner := bufio.NewScanner(strings.NewReader(string(list.Stdout)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		for _, prefix := range []string{"services:", "ports:"} {
			if values, ok := strings.CutPrefix(line, prefix); ok {
				for _, value := range strings.Fields(strings.TrimSpace(values)) {
					state.Rules = append(state.Rules, Rule{Target: value, Action: "Allow", Source: "Any"})
				}
			}
		}
		if value, ok := strings.CutPrefix(line, "rich rules:"); ok && strings.TrimSpace(value) != "" {
			state.Rules = append(state.Rules, Rule{Target: strings.TrimSpace(value), Action: "Configured", Source: "rich rule"})
		}
	}
	return state
}
