package firewall

import (
	"context"
	"errors"
	"testing"

	"sysentra/internal/runner"
)

type fakeRunner struct {
	results []runner.Result
	calls   int
}

func (r *fakeRunner) Run(context.Context, string, ...string) (runner.Result, error) {
	result := r.results[r.calls]
	r.calls++
	return result, nil
}

func lookupOnly(names ...string) func(string) (string, error) {
	return func(name string) (string, error) {
		for _, available := range names {
			if name == available {
				return "/usr/bin/" + name, nil
			}
		}
		return "", errors.New("not found")
	}
}

func TestReadsUFWStatusPolicyAndRules(t *testing.T) {
	commands := &fakeRunner{results: []runner.Result{{Stdout: []byte("Status: active\nDefault: deny (incoming), allow (outgoing)\nTo                         Action      From\n--                         ------      ----\n22/tcp                     ALLOW IN    Anywhere\n")}, {}}}
	state, err := newInventory(commands, lookupOnly("ufw", "firewall-cmd")).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || state.Backend != "ufw" || state.Policy == "" || len(state.Rules) != 1 || state.Rules[0].Target != "22" || state.Rules[0].Protocol != "tcp" || len(state.Backends) != 2 {
		t.Fatalf("state = %#v", state)
	}
}

func TestExplicitlyReadsInstalledBackend(t *testing.T) {
	commands := &fakeRunner{results: []runner.Result{{Stdout: []byte("run: firewalld: (pid 42) 10s\n")}, {Stdout: []byte("public\n")}, {Stdout: []byte("services: ssh\n")}}}
	state, err := newInventory(commands, lookupOnly("ufw", "firewall-cmd")).ReadBackend(context.Background(), "firewalld")
	if err != nil || state.Backend != "firewalld" || len(state.Backends) != 2 {
		t.Fatalf("state=%#v err=%v", state, err)
	}
}

func TestReadsFirewalldDefaultZone(t *testing.T) {
	commands := &fakeRunner{results: []runner.Result{
		{Stdout: []byte("run: firewalld: (pid 42) 10s\n")},
		{Stdout: []byte("public\n")},
		{Stdout: []byte("public\n  services: ssh dhcpv6-client\n  ports: 8080/tcp\n")},
	}}
	state, err := newInventory(commands, lookupOnly("firewall-cmd")).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !state.Active || state.Policy != "default zone: public" || len(state.Rules) != 3 {
		t.Fatalf("state = %#v", state)
	}
}

func TestInactiveFirewalldDoesNotCallFirewallCmd(t *testing.T) {
	commands := &fakeRunner{results: []runner.Result{{Stdout: []byte("down: firewalld: 1s\n")}}}
	state, err := newInventory(commands, lookupOnly("firewall-cmd")).Read(context.Background())
	if err != nil || state.Active || commands.calls != 1 {
		t.Fatalf("state=%#v calls=%d err=%v", state, commands.calls, err)
	}
}
