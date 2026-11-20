package security

import "testing"

func TestAccountRequestValidation(t *testing.T) {
	valid := []AccountRequest{{Version: APIVersion, Operation: UserAdd, User: "alice", CreateHome: true, Home: "/srv/homes/alice"}, {Version: APIVersion, Operation: GroupAdd, Group: "developers"}, {Version: APIVersion, Operation: MembershipAdd, User: "alice", Groups: []string{"wheel", "audio"}}, {Version: APIVersion, Operation: UserEdit, User: "alice", Home: "/srv/alice", Shell: "/bin/zsh", MoveHome: true}, {Version: APIVersion, Operation: PrimaryGroupChange, User: "alice", Group: "users"}, {Version: APIVersion, Operation: PasswordLock, User: "alice"}, {Version: APIVersion, Operation: PasswordUnlock, User: "alice"}}
	for _, request := range valid {
		if err := request.Validate(); err != nil {
			t.Fatalf("%s: %v", request.Operation, err)
		}
	}
	invalid := []AccountRequest{{Version: APIVersion, Operation: UserRemove, User: "root"}, {Version: APIVersion, Operation: UserAdd, User: "Bad Name"}, {Version: APIVersion, Operation: MembershipAdd, User: "alice"}, {Version: APIVersion, Operation: MembershipAdd, User: "alice", Groups: []string{"wheel", "wheel"}}, {Version: APIVersion, Operation: UserAdd, User: "alice", Home: "/home/alice"}, {Version: APIVersion, Operation: UserAdd, User: "alice", CreateHome: true, Home: "relative"}, {Version: APIVersion, Operation: UserEdit, User: "alice"}, {Version: APIVersion, Operation: UserEdit, User: "alice", Shell: "zsh"}, {Version: APIVersion, Operation: UserEdit, User: "alice", Shell: "/bin/zsh", MoveHome: true}, {Version: APIVersion, Operation: PrimaryGroupChange, User: "alice"}, {Version: APIVersion, Operation: PasswordLock, User: "alice", Group: "wheel"}}
	for _, request := range invalid {
		if err := request.Validate(); err == nil {
			t.Fatalf("expected %#v to fail", request)
		}
	}
}
