package drivers

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sysentra/internal/runner"
)

type Device struct {
	Category   string
	Bus        string
	ID         string
	Name       string
	Driver     string
	Modalias   string
	Evidence   string
	Candidates []Candidate
}

type Catalog struct {
	Devices     []Device
	Modules     []Module
	Diagnostics []string
	Warnings    []string
}

type Module struct {
	Name    string
	Devices int
}

type Inventory struct{ runner runner.Runner }

func NewInventory(commandRunner runner.Runner) *Inventory { return &Inventory{runner: commandRunner} }

var pciDevicePattern = regexp.MustCompile(`^(\S+)\s+(.+?)\s+\[([0-9A-Fa-f]{4})\]:\s+(.+?)\s+\[([0-9A-Fa-f]{4}):([0-9A-Fa-f]{4})\]`)

func (i *Inventory) List(ctx context.Context) (Catalog, error) {
	catalog := Catalog{Devices: []Device{}, Warnings: []string{}}
	if result, err := i.runner.Run(ctx, "lspci", "-knn"); err == nil {
		catalog.Devices = append(catalog.Devices, parsePCIAt(result.Stdout, "/sys")...)
	} else {
		catalog.Warnings = append(catalog.Warnings, "PCI inventory unavailable (install pciutils)")
	}
	usb := scanUSB("/sys")
	if len(usb) == 0 {
		if result, err := i.runner.Run(ctx, "lsusb"); err == nil {
			usb = parseUSB(result.Stdout)
		} else {
			catalog.Warnings = append(catalog.Warnings, "USB inventory unavailable (install usbutils)")
		}
	}
	catalog.Devices = append(catalog.Devices, usb...)
	rules, err := parseRules(defaultRules)
	if err != nil {
		catalog.Warnings = append(catalog.Warnings, "Driver evidence database could not be read")
	} else {
		enrichRecommendations(ctx, i.runner, rules, catalog.Devices)
	}
	catalog.Modules = activeModules(catalog.Devices)
	catalog.Diagnostics = driverDiagnostics(ctx, i.runner)
	return catalog, nil
}

func parsePCI(output []byte) []Device {
	return parsePCIAt(output, "")
}

func parsePCIAt(output []byte, root string) []Device {
	var devices []Device
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if match := pciDevicePattern.FindStringSubmatch(line); match != nil {
			modalias := ""
			if root != "" {
				modalias = readTrimmed(filepath.Join(root, "bus", "pci", "devices", "0000:"+match[1], "modalias"))
			}
			devices = append(devices, Device{
				Category: pciCategory(match[3], match[2]), Bus: "PCI", ID: strings.ToUpper(match[5] + ":" + match[6]),
				Name: strings.TrimSpace(match[4]), Modalias: modalias, Evidence: "PCI class " + strings.ToUpper(match[3]),
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

func scanUSB(root string) []Device {
	paths, _ := filepath.Glob(filepath.Join(root, "bus", "usb", "devices", "*"))
	var devices []Device
	for _, path := range paths {
		vendor, vendorErr := os.ReadFile(filepath.Join(path, "idVendor"))
		product, productErr := os.ReadFile(filepath.Join(path, "idProduct"))
		if vendorErr != nil || productErr != nil || readTrimmed(filepath.Join(path, "bDeviceClass")) == "09" {
			continue
		}
		manufacturer := readTrimmed(filepath.Join(path, "manufacturer"))
		productName := readTrimmed(filepath.Join(path, "product"))
		name := strings.TrimSpace(manufacturer + " " + productName)
		if name == "" {
			name = "USB device"
		}
		drivers, aliases := usbInterfaceData(root, filepath.Base(path))
		devices = append(devices, Device{
			Category: nameCategory(name), Bus: "USB",
			ID:   strings.ToUpper(strings.TrimSpace(string(vendor)) + ":" + strings.TrimSpace(string(product))),
			Name: name, Driver: strings.Join(drivers, ", "), Modalias: strings.Join(aliases, "\n"), Evidence: "USB sysfs device and interface binding",
		})
	}
	return devices
}

func usbInterfaceData(root, device string) ([]string, []string) {
	interfaces, _ := filepath.Glob(filepath.Join(root, "bus", "usb", "devices", device+":*"))
	driverSet, aliasSet := map[string]bool{}, map[string]bool{}
	for _, intf := range interfaces {
		if target, err := filepath.EvalSymlinks(filepath.Join(intf, "driver")); err == nil {
			driverSet[filepath.Base(target)] = true
		}
		if alias := readTrimmed(filepath.Join(intf, "modalias")); alias != "" {
			aliasSet[alias] = true
		}
	}
	drivers, aliases := mapKeys(driverSet), mapKeys(aliasSet)
	sort.Strings(drivers)
	sort.Strings(aliases)
	return drivers, aliases
}

func readTrimmed(path string) string {
	value, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(value))
}

func mapKeys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	return result
}

func activeModules(devices []Device) []Module {
	counts := map[string]int{}
	for _, device := range devices {
		for _, name := range strings.Split(device.Driver, ",") {
			if name = strings.TrimSpace(name); name != "" {
				counts[name]++
			}
		}
	}
	modules := make([]Module, 0, len(counts))
	for name, count := range counts {
		modules = append(modules, Module{Name: name, Devices: count})
	}
	sort.Slice(modules, func(i, j int) bool { return modules[i].Name < modules[j].Name })
	return modules
}

func driverDiagnostics(ctx context.Context, command runner.Runner) []string {
	var diagnostics []string
	if result, err := command.Run(ctx, "dkms", "status"); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(result.Stdout)), "\n") {
			if line != "" && len(diagnostics) < 100 {
				diagnostics = append(diagnostics, "DKMS: "+line)
			}
		}
	}
	if result, err := command.Run(ctx, "dracut", "--version"); err == nil {
		if version := strings.TrimSpace(string(result.Stdout)); version != "" {
			diagnostics = append(diagnostics, "Initramfs: "+strings.Split(version, "\n")[0])
		}
	}
	return diagnostics
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
