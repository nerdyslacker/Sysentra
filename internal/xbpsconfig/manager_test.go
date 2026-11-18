package xbpsconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sysentra/internal/security"
)

func TestManagerAddsEditsAndRemovesRulesAtomically(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	if err := manager.Change(security.XBPSRuleRequest{Version: 1, Operation: security.RuleAdd, Kind: security.RuleIgnorePackage, Value: "linux6.6"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(security.XBPSRuleRequest{Version: 1, Operation: security.RuleEdit, Kind: security.RuleIgnorePackage, OldValue: "linux6.6", Value: "linux6.12"}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(security.XBPSRuleRequest{Version: 1, Operation: security.RuleRemove, Kind: security.RuleIgnorePackage, Value: "linux6.12"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(dir, ManagedFile))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "ignorepkg=") {
		t.Fatalf("rule remains: %q", contents)
	}
	backup, err := os.ReadFile(filepath.Join(dir, ManagedFile+".bak"))
	if err != nil || !strings.Contains(string(backup), "ignorepkg=linux6.12") {
		t.Fatalf("backup=%q err=%v", backup, err)
	}
}

func TestManagerRejectsDuplicateMissingAndSymlinkFile(t *testing.T) {
	dir := t.TempDir()
	manager := NewManager(dir)
	request := security.XBPSRuleRequest{Version: 1, Operation: security.RuleAdd, Kind: security.RuleIgnorePackage, Value: "linux"}
	if err := manager.Change(request); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(request); err == nil {
		t.Fatal("duplicate rule was accepted")
	}
	if err := manager.Change(security.XBPSRuleRequest{Version: 1, Operation: security.RuleRemove, Kind: security.RuleIgnorePackage, Value: "missing"}); err == nil {
		t.Fatal("missing rule was removed")
	}
	other := filepath.Join(dir, "other")
	if err := os.WriteFile(other, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, ManagedFile)); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(dir, ManagedFile)); err != nil {
		t.Fatal(err)
	}
	if err := manager.Change(request); err == nil {
		t.Fatal("managed symlink was followed")
	}
}
