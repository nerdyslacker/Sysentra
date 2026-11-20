package security

import "testing"

func TestPolkitSubjectUsesUniqueSystemBusName(t *testing.T) {
	subject, err := polkitSubjectForSender(":1.42")
	if err != nil {
		t.Fatal(err)
	}
	if subject.Kind != "system-bus-name" || subject.Details["name"].Value() != ":1.42" {
		t.Fatalf("unexpected subject: %#v", subject)
	}
	if _, err := polkitSubjectForSender("io.github.Sysentra"); err == nil {
		t.Fatal("well-known name was accepted as caller identity")
	}
}

func TestKnownPolkitActionsIncludeEveryPrivilegedController(t *testing.T) {
	for _, action := range []string{PackageManagementAction, ServiceManagementAction, XBPSConfigManagementAction, SystemConfigManagementAction, AccountManagementAction, FirewallManagementAction, FirmwareManagementAction} {
		if !knownPolkitAction(action) {
			t.Fatalf("action %q is not allowlisted", action)
		}
	}
	if knownPolkitAction("io.example.arbitrary") {
		t.Fatal("unknown action was allowlisted")
	}
}
