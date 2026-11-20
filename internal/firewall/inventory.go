package firewall

import (
	"bufio"
	"context"
	"errors"
	"os/exec"
	"strings"

	"sysentra/internal/runner"
)

type Rule struct {
	Kind      string
	Target    string
	Protocol  string
	Direction string
	Action    string
	Source    string
}

type State struct {
	Backend   string
	Installed bool
	Active    bool
	Unknown   bool
	Policy    string
	Zone      string
	Rules     []Rule
	Backends  []string
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
	return i.ReadBackend(ctx, "")
}

func (i *Inventory) ReadBackend(ctx context.Context, requested string) (State, error) {
	_, ufwErr := i.lookup("ufw")
	_, firewalldErr := i.lookup("firewall-cmd")
	backends := []string{}
	if ufwErr == nil {
		backends = append(backends, "ufw")
	}
	if firewalldErr == nil {
		backends = append(backends, "firewalld")
	}
	if requested != "" && !contains(backends, requested) {
		return State{}, errors.New("requested firewall backend is not installed")
	}
	if len(backends) == 0 {
		return State{}, nil
	}
	if requested == "ufw" || (requested == "" && len(backends) == 1 && backends[0] == "ufw") {
		state := i.readUFW(ctx)
		state.Backends = backends
		return state, nil
	}
	if requested == "firewalld" || (requested == "" && len(backends) == 1) {
		state := i.readFirewalld(ctx)
		state.Backends = backends
		return state, nil
	}
	ufw := i.readUFW(ctx)
	firewalld := i.readFirewalld(ctx)
	selected := ufw
	if firewalld.Active && !ufw.Active {
		selected = firewalld
	}
	selected.Backends = backends
	return selected, nil
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
	target := strings.TrimSpace(strings.TrimSuffix(parts[0], " (v6)"))
	actionFields := strings.Fields(parts[1])
	if len(actionFields) == 0 {
		return Rule{}, false
	}
	rule := Rule{Target: target, Action: strings.ToLower(actionFields[0]), Source: strings.TrimSpace(strings.TrimSuffix(parts[2], " (v6)"))}
	if len(actionFields) > 1 {
		rule.Direction = strings.ToLower(actionFields[1])
	}
	if port, protocol, ok := strings.Cut(target, "/"); ok && (protocol == "tcp" || protocol == "udp") {
		rule.Kind, rule.Target, rule.Protocol = "port", port, protocol
	}
	return rule, true
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
	status, err := i.runner.Run(ctx, "/usr/bin/sv", "status", "/etc/runit/runsvdir/default/firewalld")
	if err != nil || !strings.HasPrefix(strings.TrimSpace(string(status.Stdout)), "run:") {
		return state
	}
	state.Active = true
	zoneResult, err := i.runner.Run(ctx, "firewall-cmd", "--get-default-zone")
	if err != nil {
		state.Unknown = true
		return state
	}
	zone := strings.TrimSpace(string(zoneResult.Stdout))
	state.Zone = zone
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
					rule := Rule{Target: value, Action: "allow", Source: "Any", Kind: "service"}
					if prefix == "ports:" {
						rule.Kind = "port"
						rule.Target, rule.Protocol, _ = strings.Cut(value, "/")
					}
					state.Rules = append(state.Rules, rule)
				}
			}
		}
		if value, ok := strings.CutPrefix(line, "rich rules:"); ok && strings.TrimSpace(value) != "" {
			state.Rules = append(state.Rules, Rule{Kind: "rich", Target: strings.TrimSpace(value), Action: "configured", Source: "rich rule"})
		} else if strings.HasPrefix(line, "rule ") {
			state.Rules = append(state.Rules, Rule{Kind: "rich", Target: line, Action: "configured", Source: "rich rule"})
		}
	}
	return state
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
