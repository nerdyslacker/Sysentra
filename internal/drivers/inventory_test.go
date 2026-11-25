package drivers

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsesPCIClassesAndBoundDrivers(t *testing.T) {
	devices := parsePCI([]byte("00:02.0 VGA compatible controller [0300]: Intel Corporation Graphics [8086:1234]\n\tKernel driver in use: i915\n03:00.0 Network controller [0280]: Intel Corporation Wi-Fi [8086:5678]\n\tKernel driver in use: iwlwifi\n"))
	if len(devices) != 2 || devices[0].Category != "Video" || devices[0].Driver != "i915" || devices[1].Category != "Wi-Fi" || devices[1].Driver != "iwlwifi" {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestCategorizesUSBPeripherals(t *testing.T) {
	devices := parseUSB([]byte("Bus 001 Device 003: ID 0bda:58f4 Realtek USB Camera\nBus 001 Device 004: ID 8087:0033 Intel Bluetooth\nBus 001 Device 005: ID 04f9:0001 Brother Printer\n"))
	if len(devices) != 3 || devices[0].Category != "Webcam" || devices[1].Category != "Bluetooth" || devices[2].Category != "Printers" {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestScansUSBDevicesWithBoundInterfaceDrivers(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "bus", "usb", "devices", "1-2")
	if err := os.MkdirAll(device, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "bus", "usb", "drivers", "uvcvideo"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"idVendor": "0bda\n", "idProduct": "58f4\n", "manufacturer": "Realtek\n", "product": "USB Camera\n", "bDeviceClass": "ef\n"} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	intf := filepath.Join(root, "bus", "usb", "devices", "1-2:1.0")
	if err := os.MkdirAll(intf, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(intf, "modalias"), []byte("usb:v0BDAp58F4d0001dcEFdsc02dp01ic0Eisc01ip00in00\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "bus", "usb", "drivers", "uvcvideo"), filepath.Join(intf, "driver")); err != nil {
		t.Fatal(err)
	}

	devices := scanUSB(root)
	if len(devices) != 1 || devices[0].ID != "0BDA:58F4" || devices[0].Name != "Realtek USB Camera" || devices[0].Driver != "uvcvideo" || devices[0].Modalias == "" {
		t.Fatalf("devices = %#v", devices)
	}
}

func TestCollectsBoundModulesSeparatelyFromPackages(t *testing.T) {
	devices := []Device{
		{Driver: "i915"},
		{Driver: "uvcvideo, snd-usb-audio"},
		{Driver: "i915"},
	}
	modules := activeModules(devices)
	if len(modules) != 3 || modules[0].Name != "i915" || modules[0].Devices != 2 || modules[1].Name != "snd-usb-audio" || modules[2].Name != "uvcvideo" {
		t.Fatalf("modules = %#v", modules)
	}
}
