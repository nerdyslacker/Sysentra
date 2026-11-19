package security

import "testing"

func TestServiceRequestAllowsFiniteRunitOperations(t *testing.T) {
	for _, operation := range []ServiceOperation{ServiceEnable, ServiceDisable, ServiceStart, ServiceStop, ServiceRestart, ServiceReload, ServiceOnce, ServiceKill, ServicePause, ServiceContinue, ServiceInitUser} {
		if err := (ServiceRequest{Version: APIVersion, Operation: operation, Name: "dbus-1"}).Validate(); err != nil {
			t.Fatalf("%s: %v", operation, err)
		}
	}
}

func TestServiceRequestRejectsPathAndUnknownOperation(t *testing.T) {
	for _, request := range []ServiceRequest{
		{Version: APIVersion, Operation: ServiceEnable, Name: "../sshd"},
		{Version: APIVersion, Operation: "remove", Name: "sshd"},
		{Version: APIVersion, Operation: ServiceStart, Name: "sshd", TimeoutSeconds: 61},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
}

func TestServiceRequestValidatesCreateCommand(t *testing.T) {
	valid := ServiceRequest{Version: APIVersion, Operation: ServiceAdd, Name: "backup", Command: "/usr/bin/rsync", Arguments: []string{"--archive", "/home/"}}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, request := range []ServiceRequest{
		{Version: APIVersion, Operation: ServiceAdd, Name: "backup", Command: "rsync"},
		{Version: APIVersion, Operation: ServiceAdd, Name: "backup", Command: "/usr/bin/rsync", Arguments: []string{"bad\x00arg"}},
		{Version: APIVersion, Operation: ServiceStart, Name: "backup", Command: "/usr/bin/rsync"},
	} {
		if err := request.Validate(); err == nil {
			t.Fatalf("accepted %#v", request)
		}
	}
}
