package app

import "testing"

func TestParseOSReleasePrefersPrettyName(t *testing.T) {
	got := parseOSRelease([]byte("NAME=Void\nVERSION_ID=2026.11\nPRETTY_NAME=\"Void Linux 2026.11\"\n"))
	if got.Name != "Void Linux 2026.11" || got.Version != "2026.11" {
		t.Fatalf("system info = %#v", got)
	}
}

func TestParseOSReleaseFallsBackToName(t *testing.T) {
	got := parseOSRelease([]byte("NAME=Void\n"))
	if got.Name != "Void" {
		t.Fatalf("system info = %#v", got)
	}
}
