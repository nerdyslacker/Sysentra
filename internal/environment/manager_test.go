package environment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellVariablesRoundTripWithoutReplacingUserContent(t *testing.T) {
	home := t.TempDir()
	rc := filepath.Join(home, ".zshrc")
	if err := os.WriteFile(rc, []byte("alias ll='ls -l'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(home)
	if err := manager.Change("zsh", "set", "EDITOR", "", "$HOME/a'b"); err != nil {
		t.Fatal(err)
	}
	variables, err := manager.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(variables) != 1 || variables[0].Name != "EDITOR" || variables[0].Value != "$HOME/a'b" || variables[0].Scope != "Zsh" {
		t.Fatalf("variables = %#v", variables)
	}
	contents, _ := os.ReadFile(rc)
	if string(contents[:len("alias ll='ls -l'\n")]) != "alias ll='ls -l'\n" {
		t.Fatalf("user content changed: %q", contents)
	}
	if !strings.Contains(string(contents), `export EDITOR='$HOME/a'\''b'`) {
		t.Fatalf("value was not safely shell-quoted: %s", contents)
	}
	if err := manager.Change("zsh", "remove", "EDITOR", "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsUnsafeVariableInput(t *testing.T) {
	manager := NewManager(t.TempDir())
	for _, test := range []struct{ name, value string }{{"BAD-NAME", "x"}, {"GOOD", "line\nbreak"}} {
		if err := manager.Change("bash", "set", test.name, "", test.value); err == nil {
			t.Fatalf("accepted %q=%q", test.name, test.value)
		}
	}
}

func TestListsAndChangesExistingShellExportsInTheirSourceFiles(t *testing.T) {
	home := t.TempDir()
	profile := filepath.Join(home, ".profile")
	if err := os.WriteFile(profile, []byte("# keep\nexport EDITOR=vim\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fish := filepath.Join(home, ".config", "fish", "config.fish")
	if err := os.MkdirAll(filepath.Dir(fish), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fish, []byte("set -gx BROWSER firefox\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	manager := NewManager(home)
	variables, err := manager.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(variables) != 2 || variables[0].Name != "EDITOR" || variables[0].File != profile || variables[1].Name != "BROWSER" || variables[1].File != fish {
		t.Fatalf("variables = %#v", variables)
	}
	if err := manager.Change("bash", "set", "VISUAL", "EDITOR", "helix", profile); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(profile)
	if string(contents) != "# keep\nexport VISUAL='helix'\n" {
		t.Fatalf("edited profile = %q", contents)
	}
	if err := manager.Change("fish", "remove", "BROWSER", "", "", fish); err != nil {
		t.Fatal(err)
	}
	contents, _ = os.ReadFile(fish)
	if len(contents) != 0 {
		t.Fatalf("fish config after removal = %q", contents)
	}
}

func TestRefusesShellConfigOutsideHomeAllowlist(t *testing.T) {
	manager := NewManager(t.TempDir())
	if err := manager.Change("bash", "set", "EDITOR", "", "vim", "/tmp/not-a-shell-config"); err == nil {
		t.Fatal("accepted a shell configuration outside the allowlist")
	}
}
