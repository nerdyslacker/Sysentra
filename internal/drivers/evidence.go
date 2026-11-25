package drivers

import (
	"bufio"
	"context"
	_ "embed"
	"fmt"
	"strings"

	"sysentra/internal/runner"
)

//go:embed driver_rules.tsv
var defaultRules string

type Candidate struct {
	Module      string
	Package     string
	Version     string
	NewVersion  string
	Description string
	Evidence    string
	Reason      string
	Available   bool
	Installed   bool
	Active      bool
	CanInstall  bool
	CanUpdate   bool
	CanRemove   bool
}

type rule struct {
	bus, id, module, packageName, description string
}

func parseRules(input string) ([]rule, error) {
	var rules []rule
	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			return nil, fmt.Errorf("invalid driver rule %q", line)
		}
		rules = append(rules, rule{bus: strings.ToUpper(fields[0]), id: strings.ToUpper(fields[1]), module: fields[2], packageName: fields[3], description: fields[4]})
	}
	return rules, scanner.Err()
}

func enrichRecommendations(ctx context.Context, command runner.Runner, rules []rule, devices []Device) {
	activeModules := map[string]bool{}
	for _, device := range devices {
		for _, module := range strings.Split(device.Driver, ",") {
			activeModules[strings.TrimSpace(module)] = true
		}
	}
	for index := range devices {
		for _, rule := range rules {
			if devices[index].Bus != rule.bus || devices[index].ID != rule.id {
				continue
			}
			installedVersion, installed := packageVersion(ctx, command, false, rule.packageName)
			repositoryVersion, available := packageVersion(ctx, command, true, rule.packageName)
			if !installed && !available {
				continue
			}
			active := activeModules[rule.module]
			reason := ""
			if devices[index].Driver != "" && !driverContains(devices[index].Driver, rule.module) {
				reason = "Another kernel driver is active; automatic switching is not supported"
			}
			devices[index].Candidates = append(devices[index].Candidates, Candidate{
				Module: rule.module, Package: rule.packageName, Version: installedVersion, NewVersion: repositoryVersion, Description: rule.description,
				Evidence: fmt.Sprintf("Exact %s ID %s; package found in an enabled XBPS repository", rule.bus, rule.id), Reason: reason,
				Available: available, Installed: installed, Active: active,
				CanInstall: available && !installed && reason == "", CanUpdate: installed && available && installedVersion != repositoryVersion, CanRemove: installed && !active,
			})
		}
	}
}

func packageVersion(ctx context.Context, command runner.Runner, repository bool, name string) (string, bool) {
	args := []string{"-p", "pkgver", name}
	if repository {
		args = append([]string{"-R"}, args...)
	}
	result, err := command.Run(ctx, "xbps-query", args...)
	if err != nil {
		return "", false
	}
	pkgver := strings.TrimSpace(string(result.Stdout))
	if pkgver == "" {
		return "", false
	}
	return pkgver, true
}

func driverContains(drivers, module string) bool {
	for _, driver := range strings.Split(drivers, ",") {
		if strings.TrimSpace(driver) == module {
			return true
		}
	}
	return false
}
