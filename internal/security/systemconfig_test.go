package security

import "testing"

func TestSystemConfigRequestValidation(t *testing.T) {
	valid := []SystemConfigRequest{
		{Version: APIVersion, Operation: MirrorUse, Value: "https://repo-fast.example.org/"},
		{Version: APIVersion, Operation: MirrorReset},
		{Version: APIVersion, Operation: RepositoryAdd, Value: "https://repo.example.org/current", TrustConfirmed: true},
		{Version: APIVersion, Operation: SambaAdd, Value: "Media", Path: "/srv/media"},
		{Version: APIVersion, Operation: SambaEdit, Value: "Media", OldValue: "Old Media", Path: "/srv/media"},
		{Version: APIVersion, Operation: SambaRemove, Value: "Media"},
		{Version: APIVersion, Operation: NFSAdd, Value: "server:/data", Path: "/mnt/data", Type: "nfs4", Options: "rw,_netdev"},
		{Version: APIVersion, Operation: NFSEdit, Value: "server:/new", OldValue: "server:/data", Path: "/mnt/new", OldPath: "/mnt/data", Type: "nfs4", Options: "rw,_netdev"},
		{Version: APIVersion, Operation: NFSRemove, Value: "server:/data", Path: "/mnt/data"},
		{Version: APIVersion, Operation: NFSMount, Value: "server:/data", Path: "/mnt/data"},
		{Version: APIVersion, Operation: NFSUnmount, Value: "server:/data", Path: "/mnt/data"},
		{Version: APIVersion, Operation: KernelSetDefault, Value: "6.12.10_1"},
	}
	for _, request := range valid {
		if err := request.Validate(); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	invalid := []SystemConfigRequest{
		{Version: APIVersion, Operation: RepositoryAdd, Value: "https://repo.example.org/current"},
		{Version: APIVersion, Operation: MirrorUse, Value: "http://insecure.example.org"},
		{Version: APIVersion, Operation: SambaAdd, Value: "bad\nname", Path: "/srv/data"},
		{Version: APIVersion, Operation: SambaEdit, Value: "Media", OldValue: "", Path: "/srv/data"},
		{Version: APIVersion, Operation: SambaRemove, Value: "global"},
		{Version: APIVersion, Operation: NFSAdd, Value: "server:/data", Path: "relative", Type: "nfs4"},
		{Version: APIVersion, Operation: NFSEdit, Value: "server:/new", OldValue: "server:/old", Path: "/mnt/new", OldPath: "relative", Type: "nfs4"},
		{Version: APIVersion, Operation: NFSRemove, Value: "bad", Path: "/mnt/data"},
		{Version: APIVersion, Operation: KernelSetDefault, Value: "../../boot"},
	}
	for _, request := range invalid {
		if err := request.Validate(); err == nil {
			t.Fatalf("expected %s to fail", request.Operation)
		}
	}
}
