package kernel

import "testing"

func TestParseKernelsMarksRunningInstalledAndAvailable(t *testing.T) {
	installed := []byte("ii linux6.12-6.12.10_1 Linux kernel\nii linux6.12-headers-6.12.10_1 headers\n")
	available := []byte("[*] linux6.12-6.12.10_1 Linux kernel\n[-] linux6.18-6.18.2_1 Linux kernel\n")

	got, err := parseKernels(installed, available, "6.12.10_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Installed || !got[0].Running || !got[0].Headers || got[0].CanRemove {
		t.Fatalf("installed kernel = %#v", got[0])
	}
	if got[1].Installed || !got[1].CanInstall {
		t.Fatalf("available kernel = %#v", got[1])
	}
}

func TestParseKernelsMatchesRunningSeriesAfterPackageUpdate(t *testing.T) {
	installed := []byte("ii linux7.3-7.3.5_1 Linux kernel\n")
	got, err := parseKernels(installed, nil, "7.3.4_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || !got[0].Running {
		t.Fatalf("kernels = %#v", got)
	}
}

func TestParseKernelsIncludesCustomKernelsAndDescriptionContinuations(t *testing.T) {
	available := []byte("[-] linux-neko-rt-7.3.0_5 Custom RT kernel\n Linux kernel with headers\n[-] linux-cachy-void-7.3.0_11 CachyOS kernel\n")
	got, err := parseKernels(nil, available, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Package != "linux-cachy-void" && got[1].Package != "linux-cachy-void" {
		t.Fatalf("kernels = %#v", got)
	}
}

func TestParseKernelsIncludesKernelMetaPackages(t *testing.T) {
	available := []byte("[-] linux-6.18_1 Linux kernel meta package\n[-] linux-lts-6.12_1 Linux LTS kernel meta package\n[*] linux-mainline-7.3_1 Linux mainline kernel meta package\n[-] linux-headers-6.18_1 Linux kernel headers meta package\n")
	got, err := parseKernels(nil, available, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("kernels = %#v", got)
	}
	for _, item := range got {
		if !item.Meta || item.CanRemove {
			t.Fatalf("meta kernel = %#v", item)
		}
	}
}

func TestParseKernelsRejectsMalformedPackageOutput(t *testing.T) {
	if _, err := parseKernels([]byte("broken\n"), nil, ""); err == nil {
		t.Fatal("expected malformed output error")
	}
}

func TestParseKernelsKeepsRunningKernelAndOneFallback(t *testing.T) {
	installed := []byte("ii linux6.12-6.12.10_1 kernel\nii linux6.6-6.6.20_1 kernel\n")
	got, err := parseKernels(installed, nil, "6.12.10_1")
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range got {
		if item.CanRemove {
			t.Fatalf("kernel should remain protected: %#v", item)
		}
	}
}

func TestKernelTypeRecognizesSupportedVariants(t *testing.T) {
	tests := map[string]string{
		"linux6.12": "Stable", "linux6.12-rt": "Realtime", "linux6.12-zen": "Zen",
		"linux6.12-hardened": "Hardened", "linux6.12-bore": "BORE", "linux6.12-mainline": "Mainline",
	}
	for name, want := range tests {
		if got := kernelType(name); got != want {
			t.Fatalf("kernelType(%q)=%q want %q", name, got, want)
		}
	}
}
