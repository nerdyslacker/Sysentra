package security

import "testing"

func TestValidatePackageRequestAcceptsFiniteOperation(t *testing.T) {
	request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: Install, Packages: []string{"firefox", "linux6.12-headers"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackageRequestAcceptsRecursivePurge(t *testing.T) {
	request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: Purge, Packages: []string{"firefox"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackageRequestAcceptsReconfigure(t *testing.T) {
	request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: Reconfigure, Packages: []string{"linux7.3"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidatePackageRequestRejectsUntrustedShapes(t *testing.T) {
	tests := []PackageRequest{
		{Version: 99, RequestID: "request-1234", Operation: Install, Packages: []string{"firefox"}},
		{Version: APIVersion, RequestID: "bad", Operation: Install, Packages: []string{"firefox"}},
		{Version: APIVersion, RequestID: "request-1234", Operation: Operation("shell"), Packages: []string{"firefox"}},
		{Version: APIVersion, RequestID: "request-1234", Operation: Install, Packages: []string{"firefox;reboot"}},
		{Version: APIVersion, RequestID: "request-1234", Operation: Install},
		{Version: APIVersion, RequestID: "request-1234", Operation: Install, Packages: []string{"firefox", "firefox"}},
	}
	for _, request := range tests {
		if err := request.Validate(); err == nil {
			t.Fatalf("accepted invalid request: %#v", request)
		}
	}
}

func TestValidateSystemUpgradeRequiresNoPackageArguments(t *testing.T) {
	valid := PackageRequest{Version: APIVersion, RequestID: "upgrade-1234", Operation: SystemUpgrade}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	valid.Packages = []string{"firefox"}
	if err := valid.Validate(); err == nil {
		t.Fatal("system upgrade accepted package arguments")
	}
}

func TestValidateDatabaseSyncRequiresNoPackageArguments(t *testing.T) {
	request := PackageRequest{Version: APIVersion, RequestID: "sync-db-1234", Operation: SyncDatabase}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.Packages = []string{"firefox"}
	if err := request.Validate(); err == nil {
		t.Fatal("database sync accepted package arguments")
	}
}

func TestValidateMaintenanceOperationShapes(t *testing.T) {
	for _, operation := range []Operation{RemoveOrphans, CleanCache, VerifyDatabase, ReconfigureAll} {
		request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: operation}
		if err := request.Validate(); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
		request.Packages = []string{"firefox"}
		if err := request.Validate(); err == nil {
			t.Fatalf("%s accepted package arguments", operation)
		}
	}
	request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: ForceRemove, Packages: []string{"firefox"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request = PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: PurgeKernels, Packages: []string{"6.12.10_1"}}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateDowngradeRequiresOnePackageAndTargetVersion(t *testing.T) {
	request := PackageRequest{Version: APIVersion, RequestID: "request-1234", Operation: Downgrade, Packages: []string{"firefox"}, TargetVersion: "130.0_1"}
	if err := request.Validate(); err != nil {
		t.Fatal(err)
	}
	request.TargetVersion = ""
	if err := request.Validate(); err == nil {
		t.Fatal("downgrade without target version was accepted")
	}
}
