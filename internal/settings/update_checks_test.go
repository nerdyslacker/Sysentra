package settings

import (
	"path/filepath"
	"testing"
)

func TestUpdateChecksDefaultsAndPersists(t *testing.T) {
	store := &UpdateChecksStore{path: filepath.Join(t.TempDir(), "update-checks.json")}
	got, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !got.Scheduled || !got.Notifications || got.IntervalMinutes != 360 {
		t.Fatalf("defaults = %#v", got)
	}
	want := UpdateChecks{Scheduled: false, Notifications: false, IntervalMinutes: 60}
	if err := store.Write(want); err != nil {
		t.Fatal(err)
	}
	got, err = store.Read()
	if err != nil || got != want {
		t.Fatalf("saved = %#v, err = %v", got, err)
	}
}

func TestUpdateChecksRejectsInvalidInterval(t *testing.T) {
	store := &UpdateChecksStore{path: filepath.Join(t.TempDir(), "update-checks.json")}
	if err := store.Write(UpdateChecks{IntervalMinutes: 1}); err == nil {
		t.Fatal("expected invalid interval error")
	}
}
