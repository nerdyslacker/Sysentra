package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadAccountsIncludesPrimaryAndSupplementaryGroups(t *testing.T) {
	root := t.TempDir()
	passwd := filepath.Join(root, "passwd")
	group := filepath.Join(root, "group")
	if err := os.WriteFile(passwd, []byte("alice:x:1000:1000::/home/alice:/bin/sh\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(group, []byte("alice:x:1000:\nwheel:x:10:alice\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	users, groups, err := readAccounts(passwd, group)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || len(groups) != 2 || len(users[0].Groups) != 2 || users[0].Groups[0] != "alice" || users[0].Groups[1] != "wheel" {
		t.Fatalf("users=%#v groups=%#v", users, groups)
	}
}
