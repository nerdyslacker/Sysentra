package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var kernelVersion = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+-]{0,127}$`)

type Mount struct {
	Target     string
	Source     string
	FileSystem string
	Size       uint64
	Available  uint64
	Mounted    bool
	Separate   bool
}

type BootImage struct {
	Version  string
	Status   string
	Size     int64
	CanPurge bool
}

type DKMSModule struct {
	Module    string
	Version   string
	Kernel    string
	Arch      string
	Status    string
	Package   bool
	Installed bool
}

type Status struct {
	BootMode        string
	Bootloader      string
	Boot            Mount
	EFI             Mount
	InitramfsTool   string
	Images          []BootImage
	DKMS            []DKMSModule
	SwitchSupported bool
	SwitchReason    string
	Warnings        []string
}

func (i *Inventory) Status(ctx context.Context) (Status, error) {
	status := Status{BootMode: "BIOS", Bootloader: detectBootloader(i.bootRoot, i.efiMount), InitramfsTool: "Unknown"}
	switch status.Bootloader {
	case "GRUB":
		status.SwitchSupported = pathExists("/etc/default/grub") && pathExists("/usr/bin/grub-mkconfig") && pathExists("/usr/bin/grub-editenv") && len(existingGRUBConfigs(i.bootRoot)) == 1
	case "systemd-boot":
		status.SwitchSupported = len(existingLoaderEntryDirs(i.bootRoot, i.efiMount)) == 1
	}
	if !status.SwitchSupported {
		status.SwitchReason = "Exactly one supported GRUB or systemd-boot configuration is required."
	}
	if _, err := os.Stat(i.efiRoot); err == nil {
		status.BootMode = "UEFI"
	}
	if _, err := os.Stat("/usr/bin/dracut"); err == nil {
		status.InitramfsTool = "dracut"
	}
	if result, err := i.runner.Run(ctx, "findmnt", "--json", "--bytes", "--output", "TARGET,SOURCE,FSTYPE,SIZE,AVAIL", "--target", i.bootRoot); err == nil {
		status.Boot, err = parseMount(result.Stdout, i.bootRoot)
		if err != nil {
			return Status{}, fmt.Errorf("parse /boot mount: %w", err)
		}
	} else {
		status.Warnings = append(status.Warnings, "Unable to inspect the /boot filesystem.")
	}
	if result, err := i.runner.Run(ctx, "findmnt", "--json", "--bytes", "--output", "TARGET,SOURCE,FSTYPE,SIZE,AVAIL", "--target", i.efiMount); err == nil {
		status.EFI, err = parseMount(result.Stdout, i.efiMount)
		if err != nil {
			return Status{}, fmt.Errorf("parse EFI mount: %w", err)
		}
		status.EFI.Mounted = status.EFI.Separate
	}
	orphans := []string{}
	if result, err := i.runner.Run(ctx, "vkpurge", "list"); err == nil {
		orphans, err = parseVKPurgeVersions(result.Stdout)
		if err != nil {
			return Status{}, err
		}
	} else {
		status.Warnings = append(status.Warnings, "vkpurge is unavailable; old kernel versions cannot be identified safely.")
	}
	runningResult, err := i.runner.Run(ctx, "uname", "-r")
	if err != nil {
		return Status{}, fmt.Errorf("detect running kernel: %w", err)
	}
	versions, sizes, managed, err := i.bootImages(ctx)
	if err != nil {
		return Status{}, err
	}
	status.Images = classifyBootImages(versions, strings.TrimSpace(string(runningResult.Stdout)), orphans, managed)
	for index := range status.Images {
		status.Images[index].Size = sizes[status.Images[index].Version]
	}
	if result, err := i.runner.Run(ctx, "dkms", "status"); err == nil {
		status.DKMS = parseDKMSStatus(result.Stdout)
	}
	if result, err := i.runner.Run(ctx, "xbps-query", "-R", "-s", "dkms"); err == nil {
		status.DKMS = mergeDKMS(status.DKMS, parseDKMSPackages(result.Stdout))
	}
	status.Warnings = append(status.Warnings, bootWarnings(status)...)
	return status, nil
}

func existingGRUBConfigs(bootRoot string) []string {
	return existingStatusPaths(false, filepath.Join(bootRoot, "grub", "grub.cfg"), filepath.Join(bootRoot, "grub2", "grub.cfg"))
}

func existingLoaderEntryDirs(bootRoot, efiMount string) []string {
	return existingStatusPaths(true, filepath.Join(bootRoot, "loader", "entries"), filepath.Join(efiMount, "loader", "entries"))
}

func existingStatusPaths(dirs bool, paths ...string) []string {
	found := []string{}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.IsDir() == dirs {
			found = append(found, path)
		}
	}
	return found
}

func bootWarnings(status Status) []string {
	warnings := []string{}
	if status.Boot.Mounted && status.Boot.Available < 128<<20 {
		warnings = append(warnings, "/boot has less than 128 MB available; kernel installation or initramfs generation may fail.")
	}
	espAtBoot := status.Boot.Separate && (status.Boot.FileSystem == "vfat" || status.Boot.FileSystem == "fat")
	if status.BootMode == "UEFI" && !status.EFI.Mounted && !espAtBoot {
		warnings = append(warnings, "UEFI mode is active but /boot/efi is not a separate mounted filesystem.")
	}
	return warnings
}

func (i *Inventory) bootImages(ctx context.Context) ([]string, map[string]int64, map[string]bool, error) {
	entries, err := os.ReadDir(i.bootRoot)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("read %s: %w", i.bootRoot, err)
	}
	versions := []string{}
	sizes := map[string]int64{}
	managed := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "vmlinuz-") {
			continue
		}
		version := strings.TrimPrefix(entry.Name(), "vmlinuz-")
		if !kernelVersion.MatchString(version) {
			continue
		}
		versions = append(versions, version)
		if info, statErr := entry.Info(); statErr == nil {
			sizes[version] = info.Size()
		}
		path := filepath.Join(i.bootRoot, entry.Name())
		if _, queryErr := i.runner.Run(ctx, "xbps-query", "--ownedby", path); queryErr == nil {
			managed[version] = true
		}
	}
	return versions, sizes, managed, nil
}

func parseMount(output []byte, requested string) (Mount, error) {
	var payload struct {
		Filesystems []struct {
			Target string `json:"target"`
			Source string `json:"source"`
			Type   string `json:"fstype"`
			Size   uint64 `json:"size"`
			Avail  uint64 `json:"avail"`
		} `json:"filesystems"`
	}
	if err := json.Unmarshal(output, &payload); err != nil || len(payload.Filesystems) != 1 {
		return Mount{}, errors.New("invalid findmnt output")
	}
	item := payload.Filesystems[0]
	return Mount{Target: requested, Source: item.Source, FileSystem: item.Type, Size: item.Size, Available: item.Avail, Mounted: true, Separate: filepath.Clean(item.Target) == filepath.Clean(requested)}, nil
}

func parseVKPurgeVersions(output []byte) ([]string, error) {
	versions := strings.Fields(string(output))
	for _, version := range versions {
		if !kernelVersion.MatchString(version) {
			return nil, fmt.Errorf("invalid vkpurge kernel version %q", version)
		}
	}
	return versions, nil
}

func parseDKMSStatus(output []byte) []DKMSModule {
	modules := []DKMSModule{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.Split(line, ", ")
		if len(parts) < 3 {
			continue
		}
		name, version, ok := strings.Cut(parts[0], "/")
		arch, state, okState := strings.Cut(parts[2], ": ")
		if !ok || !okState {
			continue
		}
		modules = append(modules, DKMSModule{Module: name, Version: version, Kernel: parts[1], Arch: arch, Status: state})
	}
	return modules
}

func parseDKMSPackages(output []byte) []DKMSModule {
	modules := []DKMSModule{}
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		separator := strings.LastIndexByte(fields[1], '-')
		if separator < 1 || separator == len(fields[1])-1 {
			continue
		}
		name, version := fields[1][:separator], fields[1][separator+1:]
		if name == "dkms" || !strings.Contains(name, "dkms") {
			continue
		}
		installed := fields[0] == "[*]"
		state := "available"
		if installed {
			state = "package installed"
		}
		modules = append(modules, DKMSModule{Module: name, Version: version, Arch: "all", Status: state, Package: true, Installed: installed})
	}
	return modules
}

func mergeDKMS(registered, packages []DKMSModule) []DKMSModule {
	seen := make(map[string]bool, len(registered))
	for _, module := range registered {
		seen[normalizeDKMSName(module.Module)] = true
	}
	for _, module := range packages {
		if !seen[normalizeDKMSName(module.Module)] {
			registered = append(registered, module)
		}
	}
	return registered
}

func normalizeDKMSName(name string) string {
	name = strings.TrimSuffix(strings.ToLower(name), "-dkms")
	if suffix := strings.LastIndexByte(name, '-'); suffix > 0 {
		if _, err := strconv.Atoi(name[suffix+1:]); err == nil {
			name = name[:suffix]
		}
	}
	return name
}

func classifyBootImages(versions []string, running string, orphans []string, managed map[string]bool) []BootImage {
	orphan := make(map[string]bool, len(orphans))
	for _, version := range orphans {
		orphan[version] = true
	}
	images := make([]BootImage, 0, len(versions))
	for _, version := range versions {
		image := BootImage{Version: version, Status: "Manual"}
		switch {
		case version == running:
			image.Status = "Running"
		case orphan[version]:
			image.Status = "Old"
			image.CanPurge = true
		case managed[version]:
			image.Status = "Managed"
		}
		images = append(images, image)
	}
	sort.SliceStable(images, func(left, right int) bool {
		if images[left].Status == "Running" {
			return true
		}
		if images[right].Status == "Running" {
			return false
		}
		return images[left].Version > images[right].Version
	})
	return images
}

func detectBootloader(bootRoot, efiMount string) string {
	grub := len(existingGRUBConfigs(bootRoot)) > 0
	systemdBoot := len(existingLoaderEntryDirs(bootRoot, efiMount)) > 0
	switch {
	case grub && systemdBoot:
		return "GRUB + systemd-boot"
	case grub:
		return "GRUB"
	case systemdBoot:
		return "systemd-boot"
	default:
		return "Unknown"
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
