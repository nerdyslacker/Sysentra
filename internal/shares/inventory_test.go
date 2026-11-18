package shares

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadSambaShares(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	if err := os.WriteFile(config, []byte("[global]\nworkgroup = WORKGROUP\n[Media]\n path = /srv/media\n read only = no\n guest ok = yes\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	server, err := readSamba(config)
	if err != nil {
		t.Fatal(err)
	}
	if !server.Configured || len(server.Shares) != 1 || server.Shares[0].Name != "Media" || server.Shares[0].Path != "/srv/media" || server.Shares[0].ReadOnly || !server.Shares[0].GuestOK {
		t.Fatalf("server = %#v", server)
	}
}

func TestReadNFSClientMergesConfiguredAndMountedShares(t *testing.T) {
	root := t.TempDir()
	fstab := filepath.Join(root, "fstab")
	mounts := filepath.Join(root, "mounts")
	if err := os.WriteFile(fstab, []byte("server:/data /mnt/data nfs4 rw,_netdev 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mounts, []byte("server:/data /mnt/data nfs4 rw,relatime 0 0\nother:/music /mnt/music nfs rw 0 0\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := readNFS(fstab, mounts)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].Configured || !entries[0].Mounted || entries[1].Configured || !entries[1].Mounted {
		t.Fatalf("entries = %#v", entries)
	}
}
