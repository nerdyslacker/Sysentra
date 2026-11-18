package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMirrorRecoveryDefaultsDisabledAndPersists(t *testing.T) {
	store := &MirrorRecoveryStore{path: filepath.Join(t.TempDir(), "sysentra", "mirror-recovery.json")}
	initial, err := store.Read()
	if err != nil || initial.Enabled {
		t.Fatalf("initial preference = %#v, err = %v", initial, err)
	}
	if err := store.Write(MirrorRecovery{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	got, err := store.Read()
	if err != nil || !got.Enabled {
		t.Fatalf("saved preference = %#v, err = %v", got, err)
	}
	info, err := os.Stat(store.path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("preference mode = %v, err = %v", info.Mode().Perm(), err)
	}
}

func TestMirrorRecoveryRejectsMalformedPreference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mirror-recovery.json")
	if err := os.WriteFile(path, []byte("not-json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := (&MirrorRecoveryStore{path: path}).Read(); err == nil {
		t.Fatal("malformed preference was accepted")
	}
}
