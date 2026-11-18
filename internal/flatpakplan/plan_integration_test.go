package flatpakplan

import (
	"context"
	"os"
	"strings"
	"testing"

	"sysentra/internal/runner"
)

func TestPreviewResolvesApplicationAndRuntime(t *testing.T) {
	if os.Getenv("SYSENTRA_FLATPAK_INTEGRATION") != "1" {
		t.Skip("set SYSENTRA_FLATPAK_INTEGRATION=1 to query Flathub")
	}
	changes, err := New(runner.Exec{}).Preview(context.Background(), []string{"org.gnome.Calculator"}, "install", "system", "flathub")
	if err != nil {
		t.Fatal(err)
	}
	var app, runtime bool
	for _, change := range changes {
		app = app || strings.Contains(change.Ref, "org.gnome.Calculator/")
		runtime = runtime || strings.Contains(change.Ref, "org.gnome.Platform/")
		if change.DownloadSize <= 0 {
			t.Fatalf("resolved operation has no download size: %#v", change)
		}
	}
	if !app || !runtime {
		t.Fatalf("resolved changes do not include app and runtime: %#v", changes)
	}
}
