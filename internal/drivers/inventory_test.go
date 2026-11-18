package drivers

import "testing"

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
