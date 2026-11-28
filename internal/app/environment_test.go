package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"sysentra/internal/environment"
	"sysentra/internal/security"
)

type environmentSystemStub struct{ request security.SystemConfigRequest }

func (s *environmentSystemStub) ChangeSystemConfig(_ context.Context, request security.SystemConfigRequest) error {
	s.request = request
	return nil
}

func TestEnvironmentServiceUsesPrivilegeOnlyForGlobalVariables(t *testing.T) {
	home := t.TempDir()
	global := filepath.Join(t.TempDir(), "environment")
	if err := os.WriteFile(global, []byte("LANG=en_US.UTF-8\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := &environmentSystemStub{}
	service := NewEnvironmentService(environment.NewManager(home), client)
	service.globalPath = global
	variables, err := service.ChangeEnvironmentVariable("zsh", "set", "EDITOR", "", "helix", "")
	if err != nil || len(variables) != 2 || client.request.Operation != "" {
		t.Fatalf("shell change: variables=%#v request=%#v err=%v", variables, client.request, err)
	}
	_, err = service.ChangeEnvironmentVariable("global", "set", "BROWSER", "", "firefox", "")
	if err != nil {
		t.Fatal(err)
	}
	if client.request.Operation != security.EnvironmentGlobalSet || client.request.Value != "BROWSER" || client.request.Path != "firefox" {
		t.Fatalf("global request = %#v", client.request)
	}
}
