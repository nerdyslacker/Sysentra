package repositories

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseRepositoriesDeduplicatesInEffectiveOrder(t *testing.T) {
	got := parseRepositories([]byte("14841 https://repo.example/current (RSA signed)\n21 https://repo.example/nonfree (RSA signed)\n14841 https://repo.example/current (RSA signed)\n"))
	if len(got) != 2 || got[0].URL != "https://repo.example/current" || got[0].Priority != 1 || got[1].Priority != 2 {
		t.Fatalf("repositories = %#v", got)
	}
}

func TestMergeConfiguredRepositoriesKeepsDisabledEntries(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sysentra-repositories.conf"), []byte("repository=https://repo.example/current\n#repository=https://repo.example/disabled\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := mergeConfiguredRepositories([]Repository{{URL: "https://repo.example/current", Priority: 1, Enabled: true}}, dir)
	if len(got) != 2 || !got[0].Mutable || got[1].URL != "https://repo.example/disabled" || got[1].Enabled || !got[1].Mutable {
		t.Fatalf("repositories = %#v", got)
	}
}

func TestMergeConfiguredRepositoriesKeepsForeignFilesReadOnly(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "custom.conf"), []byte("repository=https://repo.example/current\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := mergeConfiguredRepositories(nil, dir)
	if len(got) != 1 || got[0].Mutable {
		t.Fatalf("repositories = %#v", got)
	}
}
