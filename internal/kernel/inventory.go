package kernel

import (
	"bufio"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"sysentra/internal/runner"
)

var (
	kernelName       = regexp.MustCompile(`^linux([0-9]+\.[0-9]+)(?:-[a-z0-9][a-z0-9.-]*)?$`)
	customKernelName = regexp.MustCompile(`^linux-(?:neko-(?:rolling|rt|zen)|cachy-void)$`)
	kernelMetaName   = regexp.MustCompile(`^linux(?:-lts|-mainline)?$`)
	kernelSeries     = regexp.MustCompile(`^[0-9]+\.[0-9]+`)
)

type Kernel struct {
	Package     string
	Version     string
	Series      string
	Type        string
	Size        string
	InstallDate string
	Installed   bool
	Running     bool
	Meta        bool
	Headers     bool
	CanInstall  bool
	CanRemove   bool
}

type Inventory struct{ runner runner.Runner }

func NewInventory(commandRunner runner.Runner) *Inventory { return &Inventory{runner: commandRunner} }

func (i *Inventory) List(ctx context.Context) ([]Kernel, error) {
	installed, err := i.runner.Run(ctx, "xbps-query", "--list-pkgs")
	if err != nil {
		return nil, fmt.Errorf("list installed kernels: %w", err)
	}
	available, err := i.runner.Run(ctx, "xbps-query", "-R", "-s", "linux")
	if err != nil {
		return nil, fmt.Errorf("list available kernels: %w", err)
	}
	running, err := i.runner.Run(ctx, "uname", "-r")
	if err != nil {
		return nil, fmt.Errorf("detect running kernel: %w", err)
	}
	kernels, err := parseKernels(installed.Stdout, available.Stdout, strings.TrimSpace(string(running.Stdout)))
	if err != nil {
		return nil, err
	}
	for index := range kernels {
		item := &kernels[index]
		item.Type = kernelType(item.Package)
		args := []string{"-p", "installed_size", item.Package}
		if !item.Installed {
			args = append([]string{"-R"}, args...)
		}
		if result, queryErr := i.runner.Run(ctx, "xbps-query", args...); queryErr == nil {
			item.Size = strings.TrimSpace(string(result.Stdout))
		}
		if item.Installed {
			if result, queryErr := i.runner.Run(ctx, "xbps-query", "-p", "install-date", item.Package); queryErr == nil {
				item.InstallDate = strings.TrimSpace(string(result.Stdout))
			}
		}
	}
	return kernels, nil
}

func kernelType(name string) string {
	switch {
	case strings.Contains(name, "bore"):
		return "BORE"
	case strings.Contains(name, "hardened"):
		return "Hardened"
	case strings.Contains(name, "mainline"):
		return "Mainline"
	case name == "linux-lts":
		return "LTS"
	case strings.Contains(name, "zen"):
		return "Zen"
	case strings.HasSuffix(name, "-rt") || strings.Contains(name, "-rt-"):
		return "Realtime"
	case strings.Contains(name, "neko") || strings.Contains(name, "cachy"):
		return "Custom"
	default:
		return "Stable"
	}
}

func parseKernels(installedOutput, availableOutput []byte, running string) ([]Kernel, error) {
	byPackage := map[string]*Kernel{}
	if err := scanKernelPackages(installedOutput, true, byPackage); err != nil {
		return nil, err
	}
	if err := scanKernelPackages(availableOutput, false, byPackage); err != nil {
		return nil, err
	}
	result := make([]Kernel, 0, len(byPackage))
	installedCount := 0
	for _, item := range byPackage {
		if item.Installed && !item.Meta {
			installedCount++
		}
	}
	runningPackage := matchRunningKernel(byPackage, running)
	runningFound := runningPackage != ""
	for _, item := range byPackage {
		item.Running = item.Package == runningPackage
		item.CanInstall = !item.Installed
		item.CanRemove = item.Installed && !item.Meta && !item.Running && runningFound && installedCount > 2
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Running != result[j].Running {
			return result[i].Running
		}
		if result[i].Installed != result[j].Installed {
			return result[i].Installed
		}
		return newerSeries(result[i].Series, result[j].Series)
	})
	return result, nil
}

func matchRunningKernel(kernels map[string]*Kernel, running string) string {
	for _, item := range kernels {
		if item.Installed && !item.Meta && item.Version == running {
			return item.Package
		}
	}
	series := kernelSeries.FindString(running)
	if series == "" {
		return ""
	}
	if item := kernels["linux"+series]; item != nil && item.Installed {
		return item.Package
	}
	match := ""
	for _, item := range kernels {
		if item.Installed && !item.Meta && item.Series == series {
			if match != "" {
				return ""
			}
			match = item.Package
		}
	}
	return match
}

func scanKernelPackages(output []byte, installedList bool, byPackage map[string]*Kernel) error {
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return fmt.Errorf("invalid kernel package line %q", scanner.Text())
		}
		pkgver := fields[1]
		separator := strings.LastIndexByte(pkgver, '-')
		if separator < 1 || separator == len(pkgver)-1 {
			return fmt.Errorf("invalid kernel package version %q", pkgver)
		}
		name, version := pkgver[:separator], pkgver[separator+1:]
		headers := strings.HasSuffix(name, "-headers")
		baseName := strings.TrimSuffix(name, "-headers")
		match := kernelName.FindStringSubmatch(baseName)
		series := ""
		meta := kernelMetaName.MatchString(name)
		if meta {
			series = kernelSeries.FindString(version)
		} else if match != nil {
			series = match[1]
		} else if customKernelName.MatchString(baseName) {
			series = kernelSeries.FindString(version)
		}
		if series == "" {
			continue
		}
		item := byPackage[baseName]
		if item == nil {
			item = &Kernel{Package: baseName, Series: series, Type: kernelType(baseName), Meta: meta}
			byPackage[baseName] = item
		}
		installed := installedList || fields[0] == "[*]"
		if headers {
			item.Headers = item.Headers || installed
		} else {
			item.Installed = item.Installed || installed
			if item.Version == "" || installed {
				item.Version = version
			}
		}
	}
	return scanner.Err()
}

func newerSeries(a, b string) bool {
	parts := func(value string) (int, int) {
		major, minor, _ := strings.Cut(value, ".")
		first, _ := strconv.Atoi(major)
		second, _ := strconv.Atoi(minor)
		return first, second
	}
	aMajor, aMinor := parts(a)
	bMajor, bMinor := parts(b)
	return aMajor > bMajor || aMajor == bMajor && aMinor > bMinor
}
