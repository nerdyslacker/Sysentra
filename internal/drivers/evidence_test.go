package drivers

import (
	"context"
	"errors"
	"strings"
	"testing"

	"sysentra/internal/runner"
)

type evidenceRunner map[string]runner.Result

func (r evidenceRunner) Run(_ context.Context, name string, args ...string) (runner.Result, error) {
	result, ok := r[strings.Join(append([]string{name}, args...), " ")]
	if !ok {
		return runner.Result{}, errors.New("not found")
	}
	return result, nil
}

func TestRecommendationsRequireExactIDAndEnabledVoidPackage(t *testing.T) {
	rules, err := parseRules("USB\t0BDA:C811\trtl8821cu\trtl8821cu-dkms\tRealtek RTL8821CU USB Wi-Fi driver\n")
	if err != nil {
		t.Fatal(err)
	}
	command := evidenceRunner{
		"xbps-query -R -p pkgver rtl8821cu-dkms": {Stdout: []byte("rtl8821cu-dkms-20210916+1_1\n")},
	}
	devices := []Device{{Bus: "USB", ID: "0BDA:C811", Driver: "", Category: "Wi-Fi"}, {Bus: "USB", ID: "0BDA:FFFF", Category: "Wi-Fi"}}
	enrichRecommendations(context.Background(), command, rules, devices)
	if len(devices[0].Candidates) != 1 || devices[0].Candidates[0].Package != "rtl8821cu-dkms" || !devices[0].Candidates[0].Available || !devices[0].Candidates[0].CanInstall || devices[0].Candidates[0].Evidence != "Exact USB ID 0BDA:C811; package found in an enabled XBPS repository" {
		t.Fatalf("candidate = %#v", devices[0].Candidates)
	}
	if len(devices[1].Candidates) != 0 {
		t.Fatalf("unmatched device candidates = %#v", devices[1].Candidates)
	}
}

func TestActiveDriverPackageCannotBeRemoved(t *testing.T) {
	rules, err := parseRules("USB\t0BDA:C811\trtl8821cu\trtl8821cu-dkms\tRealtek RTL8821CU USB Wi-Fi driver\n")
	if err != nil {
		t.Fatal(err)
	}
	command := evidenceRunner{
		"xbps-query -p pkgver rtl8821cu-dkms":    {Stdout: []byte("rtl8821cu-dkms-20210916+1_1\n")},
		"xbps-query -R -p pkgver rtl8821cu-dkms": {Stdout: []byte("rtl8821cu-dkms-20210916+1_1\n")},
	}
	devices := []Device{{Bus: "USB", ID: "0BDA:C811", Driver: "rtl8821cu", Category: "Wi-Fi"}}
	enrichRecommendations(context.Background(), command, rules, devices)
	candidate := devices[0].Candidates[0]
	if !candidate.Installed || !candidate.Active || candidate.CanInstall || candidate.CanRemove {
		t.Fatalf("candidate = %#v", candidate)
	}
}

func TestDoesNotOfferUnmodelledDriverSwitch(t *testing.T) {
	rules, err := parseRules("USB\t0BDA:C811\t8821cu\trtl8821cu-dkms\tRealtek RTL8821CU USB Wi-Fi driver\n")
	if err != nil {
		t.Fatal(err)
	}
	command := evidenceRunner{"xbps-query -R -p pkgver rtl8821cu-dkms": {Stdout: []byte("rtl8821cu-dkms-20210916+1_1\n")}}
	devices := []Device{{Bus: "USB", ID: "0BDA:C811", Driver: "rtw88_8821cu", Category: "Wi-Fi"}}
	enrichRecommendations(context.Background(), command, rules, devices)
	if devices[0].Candidates[0].CanInstall || devices[0].Candidates[0].Reason == "" {
		t.Fatalf("candidate = %#v", devices[0].Candidates[0])
	}
}

func TestAddsVoidFirmwareForDetectedGPUVendor(t *testing.T) {
	command := evidenceRunner{
		"xbps-query -R -p pkgver linux-firmware-nvidia": {Stdout: []byte("linux-firmware-nvidia-20260810_1\n")},
	}
	devices := []Device{{Bus: "PCI", ID: "10DE:2684", Category: "Video", Name: "NVIDIA graphics", Driver: "nouveau"}}

	enrichVendorFirmware(context.Background(), command, devices)
	if len(devices[0].Candidates) != 1 || devices[0].Candidates[0].Package != "linux-firmware-nvidia" || !devices[0].Candidates[0].CanInstall || devices[0].Candidates[0].CanRemove || devices[0].Candidates[0].Module != "" || devices[0].Candidates[0].Reason != "" {
		t.Fatalf("candidate = %#v", devices[0].Candidates)
	}
}

func TestAddsVoidFirmwareForDetectedWiFiVendor(t *testing.T) {
	command := evidenceRunner{
		"xbps-query -R -p pkgver linux-firmware-broadcom": {Stdout: []byte("linux-firmware-broadcom-20260810_1\n")},
	}
	devices := []Device{{Bus: "SDIO", ID: "02D0:A9A6", Category: "Wi-Fi", Driver: "brcmfmac"}}

	enrichVendorFirmware(context.Background(), command, devices)
	if len(devices[0].Candidates) != 1 || devices[0].Candidates[0].Package != "linux-firmware-broadcom" || !devices[0].Candidates[0].CanInstall || devices[0].Candidates[0].CanRemove {
		t.Fatalf("candidate = %#v", devices[0].Candidates)
	}
}
