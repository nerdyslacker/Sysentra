package security

import "testing"

func TestSystemConfigRequestValidation(t *testing.T) {
	valid := []SystemConfigRequest{
		{Version: APIVersion, Operation: MirrorUse, Value: "https://repo-fast.example.org/"},
		{Version: APIVersion, Operation: MirrorReset},
		{Version: APIVersion, Operation: RepositoryAdd, Value: "https://repo.example.org/current", Path: "custom.conf", TrustConfirmed: true},
		{Version: APIVersion, Operation: SambaAdd, Value: "Media", Path: "/srv/media", Comment: "Shared media", Browseable: true, FollowSymlinks: true, ValidUsers: "alice, @media", WriteList: "@editors", ReadList: "bob", ForceUser: "media", ForceGroup: "media", CreateMask: "0770", DirectoryMask: "0775"},
		{Version: APIVersion, Operation: SambaEdit, Value: "Media", OldValue: "Old Media", Path: "/srv/media"},
		{Version: APIVersion, Operation: SambaRemove, Value: "Media"},
		{Version: APIVersion, Operation: SambaGlobalSet, Protocol: "SMB2", DefaultMask: "0775", Printing: true, MapGuest: true},
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
		{Version: APIVersion, Operation: RepositoryAdd, Value: "https://repo.example.org/current", Path: "../evil.conf", TrustConfirmed: true},
		{Version: APIVersion, Operation: MirrorUse, Value: "http://insecure.example.org"},
		{Version: APIVersion, Operation: SambaAdd, Value: "bad\nname", Path: "/srv/data"},
		{Version: APIVersion, Operation: SambaEdit, Value: "Media", OldValue: "", Path: "/srv/data"},
		{Version: APIVersion, Operation: SambaRemove, Value: "global"},
		{Version: APIVersion, Operation: SambaGlobalSet, Protocol: "NT1", DefaultMask: "0775"},
		{Version: APIVersion, Operation: SambaGlobalSet, Protocol: "SMB2", DefaultMask: "0700"},
		{Version: APIVersion, Operation: SambaAdd, Value: "Media", Path: "/srv/data", Comment: "bad\ncomment", CreateMask: "0770", DirectoryMask: "0775"},
		{Version: APIVersion, Operation: SambaAdd, Value: "Media", Path: "/srv/data", ValidUsers: "alice;root", CreateMask: "0770", DirectoryMask: "0775"},
		{Version: APIVersion, Operation: SambaAdd, Value: "Media", Path: "/srv/data", CreateMask: "0777", DirectoryMask: "0999"},
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
