package kernel

import "testing"

func TestParseMountReportsBackingFilesystemAndSeparateMount(t *testing.T) {
	output := []byte(`{"filesystems":[{"target":"/boot","source":"/dev/nvme0n1p1","fstype":"ext4","size":1073741824,"avail":536870912}]}`)
	got, err := parseMount(output, "/boot")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Mounted || !got.Separate || got.Source != "/dev/nvme0n1p1" || got.Size != 1073741824 || got.Available != 536870912 {
		t.Fatalf("mount = %#v", got)
	}
}

func TestParseMountMarksBootOnRootAsNotSeparate(t *testing.T) {
	output := []byte(`{"filesystems":[{"target":"/","source":"/dev/vda2","fstype":"ext4","size":1000,"avail":400}]}`)
	got, err := parseMount(output, "/boot")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Mounted || got.Separate || got.Target != "/boot" {
		t.Fatalf("mount = %#v", got)
	}
}

func TestParseVKPurgeVersionsRejectsUnexpectedOutput(t *testing.T) {
	got, err := parseVKPurgeVersions([]byte("6.12.10_1\n6.6.72_1\n"))
	if err != nil || len(got) != 2 || got[0] != "6.12.10_1" {
		t.Fatalf("versions=%#v err=%v", got, err)
	}
	if _, err := parseVKPurgeVersions([]byte("../../boot\n")); err == nil {
		t.Fatal("expected invalid vkpurge version error")
	}
}

func TestParseDKMSStatus(t *testing.T) {
	got := parseDKMSStatus([]byte("nvidia/550.54.14, 6.6.21_1, x86_64: installed\nzfs/2.2.3, 6.6.21_1, x86_64: built\n"))
	if len(got) != 2 || got[0].Module != "nvidia" || got[0].Version != "550.54.14" || got[0].Kernel != "6.6.21_1" || got[0].Status != "installed" {
		t.Fatalf("modules = %#v", got)
	}
}

func TestParseDKMSPackagesListsAvailableAndInstalledModules(t *testing.T) {
	got := parseDKMSPackages([]byte("[-] acpi_call-dkms-1.2.2_1 Kernel module\n[*] nvidia-dkms-595.112.07_2 NVIDIA driver\n[-] dkms-3.3.0_1 Framework\n"))
	if len(got) != 2 || got[0].Module != "acpi_call-dkms" || got[0].Version != "1.2.2_1" || got[0].Status != "available" || !got[0].Package || got[0].Installed || !got[1].Installed {
		t.Fatalf("packages = %#v", got)
	}
}

func TestClassifyBootImages(t *testing.T) {
	got := classifyBootImages([]string{"7.3.5_1", "7.3.4_1", "6.6.2-custom"}, "7.3.5_1", []string{"7.3.4_1"}, map[string]bool{"7.3.5_1": true})
	if len(got) != 3 || got[0].Status != "Running" || got[1].Status != "Old" || got[1].CanPurge || got[2].Status != "Manual" {
		t.Fatalf("images = %#v", got)
	}
}

func TestBootWarningsFlagLowSpaceAndUnmountedEFI(t *testing.T) {
	got := bootWarnings(Status{BootMode: "UEFI", Boot: Mount{Mounted: true, Available: 64 << 20}, EFI: Mount{Mounted: false}})
	if len(got) != 2 {
		t.Fatalf("warnings = %#v", got)
	}
}
