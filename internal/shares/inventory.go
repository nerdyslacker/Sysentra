package shares

import (
	"bufio"
	"errors"
	"os"
	"sort"
	"strings"
)

type SambaShare struct {
	Name, Path        string
	ReadOnly, GuestOK bool
}
type SambaServer struct {
	Configured bool
	Shares     []SambaShare
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
	server := SambaServer{Configured: true, Shares: []SambaShare{}}
	current := -1
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			name := strings.TrimSpace(line[1 : len(line)-1])
			current = -1
			if !strings.EqualFold(name, "global") {
				server.Shares = append(server.Shares, SambaShare{Name: name, ReadOnly: true})
				current = len(server.Shares) - 1
			}
			continue
		}
		if current < 0 {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "path":
			server.Shares[current].Path = strings.TrimSpace(value)
		case "read only":
			server.Shares[current].ReadOnly = !isFalse(value)
		case "guest ok":
			server.Shares[current].GuestOK = isTrue(value)
		}
	}
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
