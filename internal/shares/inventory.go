package shares

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strings"
)

type SambaShare struct {
	Name, Path, Comment             string
	ValidUsers, WriteList, ReadList string
	ForceUser, ForceGroup           string
	CreateMask, DirectoryMask       string
	ReadOnly, GuestOK, Browseable   bool
	FollowSymlinks, WideLinks       bool
}
type SambaServer struct {
	Configured bool
	Shares     []SambaShare
	Global     SambaGlobal
}
type SambaGlobal struct {
	Protocol    string
	DefaultMask string
	Printing    bool
	MapGuest    bool
}
type NFSMount struct {
	Source, Target, Type, Options string
	Configured, Mounted           bool
}

type Inventory struct{ sambaConfig, fstab, mounts string }

func NewInventory() *Inventory {
	return &Inventory{sambaConfig: "/etc/samba/smb.conf", fstab: "/etc/fstab", mounts: "/proc/self/mounts"}
}

func (i *Inventory) Samba() (SambaServer, error) { return readSamba(i.sambaConfig) }
func (i *Inventory) NFS() ([]NFSMount, error)    { return readNFS(i.fstab, i.mounts) }

func readSamba(path string) (SambaServer, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return SambaServer{Shares: []SambaShare{}}, nil
	}
	if err != nil {
		return SambaServer{}, err
	}
	defer file.Close()
	server := SambaServer{Configured: true, Shares: []SambaShare{}, Global: SambaGlobal{Protocol: "SMB2", DefaultMask: "0755"}}
	current := -1
	inGlobal, inPrinters := false, false
	printingCUPS, loadPrinters, printable := false, true, false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			current = -1
			inGlobal = strings.EqualFold(name, "global")
			inPrinters = strings.EqualFold(name, "printers")
			if !inGlobal && !inPrinters && !strings.EqualFold(name, "print$") && !strings.EqualFold(name, "homes") {
				server.Shares = append(server.Shares, SambaShare{Name: name, ReadOnly: true, Browseable: true, FollowSymlinks: true, CreateMask: "0744", DirectoryMask: "0755"})
				current = len(server.Shares) - 1
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if inGlobal {
			switch key {
			case "server min protocol", "client min protocol":
				server.Global.Protocol = strings.ToUpper(value)
			case "create mask", "directory mask":
				server.Global.DefaultMask = value
			case "map to guest":
				server.Global.MapGuest = strings.EqualFold(value, "Bad User")
			case "printing":
				printingCUPS = strings.EqualFold(value, "cups")
			case "load printers":
				loadPrinters = !isFalse(value)
			}
			continue
		}
		if inPrinters {
			if key == "printable" {
				printable = isTrue(value)
			}
			continue
		}
		if current < 0 {
			continue
		}
		switch key {
		case "path":
			server.Shares[current].Path = strings.TrimSpace(value)
		case "comment":
			server.Shares[current].Comment = strings.TrimSpace(value)
		case "browseable", "browsable":
			server.Shares[current].Browseable = isTrue(value)
		case "read only":
			server.Shares[current].ReadOnly = !isFalse(value)
		case "guest ok":
			server.Shares[current].GuestOK = isTrue(value)
		case "follow symlinks":
			server.Shares[current].FollowSymlinks = isTrue(value)
		case "wide links":
			server.Shares[current].WideLinks = isTrue(value)
		case "valid users":
			server.Shares[current].ValidUsers = strings.TrimSpace(value)
		case "write list":
			server.Shares[current].WriteList = strings.TrimSpace(value)
		case "read list":
			server.Shares[current].ReadList = strings.TrimSpace(value)
		case "force user":
			server.Shares[current].ForceUser = strings.TrimSpace(value)
		case "force group":
			server.Shares[current].ForceGroup = strings.TrimSpace(value)
		case "create mask", "create mode":
			server.Shares[current].CreateMask = strings.TrimSpace(value)
		case "directory mask", "directory mode":
			server.Shares[current].DirectoryMask = strings.TrimSpace(value)
		}
	}
	server.Global.Printing = printingCUPS && loadPrinters && printable
	return server, scanner.Err()
}

func readNFS(fstabPath, mountsPath string) ([]NFSMount, error) {
	entries := map[string]NFSMount{}
	if err := readMountFile(fstabPath, true, entries); err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err := readMountFile(mountsPath, false, entries); err != nil {
		return nil, err
	}
	result := make([]NFSMount, 0, len(entries))
	for _, entry := range entries {
		result = append(result, entry)
	}
	sort.Slice(result, func(a, b int) bool { return result[a].Target < result[b].Target })
	return result, nil
}

func readMountFile(path string, configured bool, entries map[string]NFSMount) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		fields := strings.Fields(line)
		if len(fields) < 4 || (fields[2] != "nfs" && fields[2] != "nfs4") {
			continue
		}
		key := fields[0] + "\x00" + fields[1]
		entry := entries[key]
		entry.Source, entry.Target, entry.Type, entry.Options = unescape(fields[0]), unescape(fields[1]), fields[2], fields[3]
		if configured {
			entry.Configured = true
		} else {
			entry.Mounted = true
		}
		entries[key] = entry
	}
	return scanner.Err()
}

func isTrue(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "yes" || value == "true" || value == "1"
}
func isFalse(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "no" || value == "false" || value == "0"
}
func unescape(value string) string { return strings.ReplaceAll(value, `\040`, " ") }
