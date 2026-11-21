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

func TestReadSambaSharesIncludesAccessAndCreationOptions(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	contents := `[global]
workgroup = WORKGROUP
[Media]
 path = /srv/media
 comment = Shared media
 browsable = no
 read only = yes
 guest ok = no
 follow symlinks = yes
 wide links = no
 valid users = alice, @media
 write list = alice, @editors
 read list = bob, @viewers
 force user = media
 force group = media
 create mask = 0770
 directory mask = 0775
`
	if err := os.WriteFile(config, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}

	server, err := readSamba(config)
	if err != nil {
		t.Fatal(err)
	}
	if len(server.Shares) != 1 {
		t.Fatalf("shares = %#v", server.Shares)
	}
	share := server.Shares[0]
	if share.Comment != "Shared media" || share.Browseable || !share.FollowSymlinks || share.WideLinks || share.ValidUsers != "alice, @media" || share.WriteList != "alice, @editors" || share.ReadList != "bob, @viewers" || share.ForceUser != "media" || share.ForceGroup != "media" || share.CreateMask != "0770" || share.DirectoryMask != "0775" {
		t.Fatalf("share = %#v", share)
	}
}

func TestReadSambaGlobalConfiguration(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "smb.conf")
	contents := `[global]
server min protocol = SMB2
client min protocol = SMB2
create mask = 0775
directory mask = 0775
map to guest = Bad User
printing = cups
load printers = yes

[printers]
path = /var/spool/samba
printable = yes
`
	if err := os.WriteFile(config, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	server, err := readSamba(config)
	if err != nil {
		t.Fatal(err)
	}
	if server.Global.Protocol != "SMB2" || server.Global.DefaultMask != "0775" || !server.Global.MapGuest || !server.Global.Printing {
		t.Fatalf("global = %#v", server.Global)
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
