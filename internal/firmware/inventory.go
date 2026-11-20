package firmware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"sort"
	"strings"

	"sysentra/internal/runner"
)

var errNoOutput = errors.New("fwupd returned no output")

type Device struct {
	Name             string
	Vendor           string
	Version          string
	ID               string
	Summary          string
	Updatable        bool
	NewVersion       string
	RebootRequired   bool
	ShutdownRequired bool
	ACRequired       bool
	UpdateRemote     string
	UpdateSummary    string
	UpdateMessage    string
	UpdateUrgency    string
	UpdateSize       uint64
}

type Catalog struct {
	Available bool
	Devices   []Device
	Warning   string
}

type Inventory struct {
	runner   runner.Runner
	lookPath func(string) (string, error)
}

type fwupdDevices struct {
	Devices []struct {
		Name     string   `json:"Name"`
		Vendor   string   `json:"Vendor"`
		Version  string   `json:"Version"`
		DeviceID string   `json:"DeviceId"`
		Summary  string   `json:"Summary"`
		Flags    []string `json:"Flags"`
		Releases []struct {
			Version       string   `json:"Version"`
			Flags         []string `json:"Flags"`
			RemoteID      string   `json:"RemoteId"`
			Summary       string   `json:"Summary"`
			UpdateMessage string   `json:"UpdateMessage"`
			Urgency       string   `json:"Urgency"`
			Size          uint64   `json:"Size"`
		} `json:"Releases"`
	} `json:"Devices"`
}

func NewInventory(commandRunner runner.Runner) *Inventory {
	return &Inventory{runner: commandRunner, lookPath: exec.LookPath}
}

func (i *Inventory) List(ctx context.Context) (Catalog, error) {
	if _, err := i.lookPath("fwupdmgr"); err != nil {
		return Catalog{}, nil
	}
	devices, err := i.query(ctx, "get-devices")
	if err != nil {
		if errors.Is(err, errNoOutput) {
			return Catalog{Available: true, Devices: []Device{}, Warning: "The fwupd daemon is unavailable."}, nil
		}
		return Catalog{}, err
	}
	updates, updateErr := i.query(ctx, "get-updates")
	if updateErr != nil && !errors.Is(updateErr, errNoOutput) {
		return Catalog{}, updateErr
	}
	waiting := map[string]struct {
		version, remote, summary, message, urgency string
		size                                       uint64
		reboot, shutdown, ac                       bool
	}{}
	for _, item := range updates.Devices {
		if len(item.Releases) == 0 {
			continue
		}
		release := item.Releases[0]
		waiting[item.DeviceID] = struct {
			version, remote, summary, message, urgency string
			size                                       uint64
			reboot, shutdown, ac                       bool
		}{release.Version, release.RemoteID, release.Summary, release.UpdateMessage, release.Urgency, release.Size,
			slices.Contains(item.Flags, "needs-reboot") || slices.Contains(release.Flags, "needs-reboot"),
			slices.Contains(item.Flags, "needs-shutdown") || slices.Contains(release.Flags, "needs-shutdown"),
			slices.Contains(item.Flags, "require-ac") || slices.Contains(release.Flags, "require-ac")}
	}
	result := Catalog{Available: true, Devices: make([]Device, 0, len(devices.Devices))}
	for _, item := range devices.Devices {
		device := Device{Name: item.Name, Vendor: item.Vendor, Version: item.Version, ID: item.DeviceID, Summary: item.Summary, Updatable: slices.Contains(item.Flags, "updatable")}
		if update, found := waiting[item.DeviceID]; found {
			device.NewVersion, device.RebootRequired, device.ShutdownRequired, device.ACRequired = update.version, update.reboot, update.shutdown, update.ac
			device.UpdateRemote, device.UpdateSummary, device.UpdateMessage, device.UpdateUrgency, device.UpdateSize = update.remote, update.summary, update.message, update.urgency, update.size
		}
		result.Devices = append(result.Devices, device)
	}
	sort.Slice(result.Devices, func(a, b int) bool { return result.Devices[a].Name < result.Devices[b].Name })
	return result, nil
}

func (i *Inventory) query(ctx context.Context, operation string) (fwupdDevices, error) {
	result, commandErr := i.runner.Run(ctx, "fwupdmgr", operation, "--json")
	if strings.TrimSpace(string(result.Stdout)) == "" {
		if commandErr != nil {
			return fwupdDevices{}, fmt.Errorf("%w: fwupdmgr %s: %v", errNoOutput, operation, commandErr)
		}
		return fwupdDevices{}, nil
	}
	var devices fwupdDevices
	if err := json.Unmarshal(result.Stdout, &devices); err != nil {
		return fwupdDevices{}, fmt.Errorf("parse fwupdmgr %s output: %w", operation, err)
	}
	return devices, nil
}
