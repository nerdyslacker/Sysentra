package drivers

import (
	"context"
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

func TestScansSDIODevicesWithBoundDriver(t *testing.T) {
	root := t.TempDir()
	device := filepath.Join(root, "bus", "sdio", "devices", "mmc1:0001:1")
	driver := filepath.Join(root, "bus", "sdio", "drivers", "brcmfmac")
	if err := os.MkdirAll(device, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(driver, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{"vendor": "0x02d0\n", "device": "0xa9a6\n", "class": "0x07\n", "modalias": "sdio:c07v02D0dA9A6\n"} {
		if err := os.WriteFile(filepath.Join(device, name), []byte(value), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(driver, filepath.Join(device, "driver")); err != nil {
		t.Fatal(err)
	}

	devices := scanSDIO(root)
	if len(devices) != 1 || devices[0].Bus != "SDIO" || devices[0].ID != "02D0:A9A6" || devices[0].Category != "Wi-Fi" || devices[0].Driver != "brcmfmac" || devices[0].Modalias != "sdio:c07v02D0dA9A6" {
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

func TestReportsDKMSModulesMissingFromInstalledKernels(t *testing.T) {
	status := []byte("nvidia/580.1, 6.12.10_1, x86_64: installed\nnvidia/580.1, 6.6.70_1, x86_64: built\nrtl8821cu/5.12, 6.12.10_1, x86_64: installed\n")
	warnings := missingDKMSBuilds(status, []string{"6.6.70_1", "6.12.10_1"})
	if len(warnings) != 2 || warnings[0] != "DKMS module nvidia/580.1 is not installed for kernel 6.6.70_1 (built)" || warnings[1] != "DKMS module rtl8821cu/5.12 is missing for kernel 6.6.70_1" {
		t.Fatalf("warnings = %#v", warnings)
	}
}

func TestDiscoversQEMUGuestToolsFromDMIAndEnabledRepository(t *testing.T) {
	root := t.TempDir()
	dmi := filepath.Join(root, "class", "dmi", "id")
	if err := os.MkdirAll(dmi, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmi, "sys_vendor"), []byte("QEMU\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dmi, "product_name"), []byte("Standard PC (Q35 + ICH9, 2009)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := evidenceRunner{
		"xbps-query -R -p pkgver qemu-ga": {Stdout: []byte("qemu-ga-11.1.3_2\n")},
	}

	devices := scanVirtualGuest(context.Background(), command, root)
	if len(devices) != 1 || devices[0].Bus != "Virtual" || devices[0].ID != "qemu" || devices[0].Name != "QEMU/KVM guest" || len(devices[0].Candidates) != 1 || devices[0].Candidates[0].Package != "qemu-ga" || devices[0].Candidates[0].Module != "" || !devices[0].Candidates[0].CanInstall {
		t.Fatalf("devices = %#v", devices)
	}
}
