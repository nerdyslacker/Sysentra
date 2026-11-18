package drivers

import (
	"bufio"
	"context"
	"regexp"
	"strings"

	"sysentra/internal/runner"
)

type Device struct {
	Category string
	Bus      string
	ID       string
	Name     string
	Driver   string
	Evidence string
}

type Catalog struct {
	Devices  []Device
	Warnings []string
}

type Inventory struct{ runner runner.Runner }

func NewInventory(commandRunner runner.Runner) *Inventory { return &Inventory{runner: commandRunner} }

var pciDevicePattern = regexp.MustCompile(`^\S+\s+(.+?)\s+\[([0-9A-Fa-f]{4})\]:\s+(.+?)\s+\[([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})\]`)

func (i *Inventory) List(ctx context.Context) (Catalog, error) {
	catalog := Catalog{Devices: []Device{}, Warnings: []string{}}
	if result, err := i.runner.Run(ctx, "lspci", "-knn"); err == nil {
		catalog.Devices = append(catalog.Devices, parsePCI(result.Stdout)...)
	} else {
		catalog.Warnings = append(catalog.Warnings, "PCI inventory unavailable (install pciutils)")
	}
	if result, err := i.runner.Run(ctx, "lsusb"); err == nil {
		catalog.Devices = append(catalog.Devices, parseUSB(result.Stdout)...)
	} else {
		catalog.Warnings = append(catalog.Warnings, "USB inventory unavailable (install usbutils)")
	}
	return catalog, nil
}

func parsePCI(output []byte) []Device {
	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if match := pciDevicePattern.FindStringSubmatch(line); match != nil {
			devices = append(devices, Device{
				Category: pciCategory(match[2], match[1]), Bus: "PCI", ID: strings.ToUpper(match[4] + ":" + match[5]),
				Name: strings.TrimSpace(match[3]), Evidence: "PCI class " + strings.ToUpper(match[2]),
			})
			continue
		}
		if len(devices) > 0 {
			if driver, ok := strings.CutPrefix(strings.TrimSpace(line), "Kernel driver in use:"); ok {
				devices[len(devices)-1].Driver = strings.TrimSpace(driver)
			}
		}
	}
	return devices
}

func parseUSB(output []byte) []Device {
	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		marker := strings.Index(line, " ID ")
		if marker < 0 {
			continue
		}
		fields := strings.Fields(line[marker+4:])
		if len(fields) < 2 || len(fields[0]) != 9 || fields[0][4] != ':' {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line[marker+4+len(fields[0]):], " "))
		devices = append(devices, Device{Category: nameCategory(name), Bus: "USB", ID: strings.ToUpper(fields[0]), Name: name, Evidence: "USB device description"})
	}
	return devices
}

func pciCategory(class, description string) string {
	switch strings.ToUpper(class)[:2] {
	case "03":
		return "Video"
	case "02":
		if strings.Contains(strings.ToLower(description), "wireless") || strings.HasPrefix(strings.ToUpper(class), "0280") {
			return "Wi-Fi"
		}
		return "Ethernet"
	case "04":
		return "Audio"
	default:
		return nameCategory(description)
	}
}

func nameCategory(name string) string {
	lower := strings.ToLower(name)
	checks := []struct{ category, term string }{
		{"Bluetooth", "bluetooth"}, {"Wi-Fi", "wireless"}, {"Wi-Fi", "wifi"}, {"Ethernet", "ethernet"},
		{"Webcam", "camera"}, {"Webcam", "webcam"}, {"Touchscreen", "touch"}, {"Printers", "printer"},
		{"Scanners", "scanner"}, {"Audio", "audio"}, {"Audio", "sound"}, {"Video", "display"}, {"Video", "graphics"},
	}
	for _, check := range checks {
		if strings.Contains(lower, check.term) {
			return check.category
		}
	}
	return "Other"
}

func (d Device) Status() string {
	if d.Driver != "" {
		return "Driver active"
	}
	return "No bound driver"
}
