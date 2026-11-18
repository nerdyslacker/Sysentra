package security

import "testing"

func TestServiceRequestAllowsFiniteRunitOperations(t *testing.T) {
	for _, operation := range []ServiceOperation{ServiceEnable, ServiceDisable, ServiceStart, ServiceStop, ServiceRestart, ServiceReload, ServiceOnce, ServiceKill, ServicePause, ServiceContinue} {
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
