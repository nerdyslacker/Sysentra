package xbpsconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListUsesLocalFileInsteadOfSameNamedVendorFile(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "etc")
	vendor := filepath.Join(root, "share")
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(vendor, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendor, "policy.conf"), []byte("ignorepkg=vendor\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, "policy.conf"), []byte("ignorepkg=local\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(local, ManagedFile), []byte("virtualpkg=cron-daemon:dcron\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rules, err := (&Inventory{localDir: local, vendorDir: vendor}).List()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 2 || rules[0].Value != "local" || rules[0].Mutable || rules[1].Value != "cron-daemon:dcron" || !rules[1].Mutable {
		t.Fatalf("unexpected rules: %#v", rules)
	}
}
