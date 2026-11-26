package systemconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sysentra/internal/runner"
	"sysentra/internal/security"
)

const (
	managedRepositories = "sysentra-repositories.conf"
	managedMarker       = "# Managed by Sysentra. Manual changes may be replaced."
	defaultMirror       = "https://repo-default.voidlinux.org"
)

type Manager struct {
	runner                      runner.Runner
	configurationDir, vendorDir string
	sambaConfig, fstab          string
	mounts                      string
	bootRoot, efiMount          string
	grubDefaults                string
}

func NewManager(commandRunner runner.Runner, configurationDir, vendorDir, sambaConfig, fstab string) *Manager {
	return &Manager{runner: commandRunner, configurationDir: configurationDir, vendorDir: vendorDir, sambaConfig: sambaConfig, fstab: fstab, mounts: "/proc/self/mounts", bootRoot: "/boot", efiMount: "/boot/efi", grubDefaults: "/etc/default/grub"}
}

func (m *Manager) Change(request security.SystemConfigRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	switch request.Operation {
	case security.RepositoryAdd, security.RepositoryEdit, security.RepositoryRemove, security.RepositoryEnable, security.RepositoryDisable:
		return m.changeRepository(request)
	case security.MirrorUse:
		return m.changeMirror(strings.TrimSuffix(request.Value, "/"))
	case security.MirrorReset:
		return m.changeMirror(defaultMirror)
	case security.SambaAdd, security.SambaEdit, security.SambaRemove:
		return m.changeSamba(request)
	case security.SambaGlobalSet:
		return m.changeSambaGlobal(request)
	case security.NFSAdd, security.NFSEdit, security.NFSRemove:
		return m.changeNFS(request)
	case security.NFSMount:
		return m.mountNFS(request)
	case security.NFSUnmount:
		return m.unmountNFS(request)
	case security.KernelSetDefault:
		return m.setDefaultKernel(request.Value)
	default:
		return errors.New("unsupported system configuration operation")
	}
}

func (m *Manager) changeRepository(request security.SystemConfigRequest) error {
	path := filepath.Join(m.configurationDir, request.Path)
	contents, source, err := m.repositoryConfig(request.Path)
	if err != nil {
		return err
	}
	if request.Operation == security.RepositoryAdd && source != "" {
		return errors.New("repository configuration file already exists")
	}
	if request.Operation != security.RepositoryAdd && source == "" {
		return errors.New("repository configuration file was not found")
	}
	lines := nonemptyLines(contents)
	if len(lines) == 0 {
		lines = []string{managedMarker}
	} else if lines[0] != managedMarker {
		lines = append([]string{managedMarker}, lines...)
	}
	oldURL := request.Value
	if request.Operation == security.RepositoryEdit {
		oldURL = request.OldValue
	}
	index := repositoryLine(lines, oldURL)
	switch request.Operation {
	case security.RepositoryAdd:
		if index >= 0 || m.repositoryExists(request.Value, request.Path) {
			return errors.New("repository already exists")
		}
		lines = append(lines, "repository="+request.Value)
	case security.RepositoryEdit:
		if index < 0 {
			return errors.New("repository was not found in the selected configuration file")
		}
		if repositoryLine(lines, request.Value) >= 0 || m.repositoryExists(request.Value, request.Path) {
			return errors.New("replacement repository already exists")
		}
		prefix := "repository="
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "#repository=") {
			prefix = "#repository="
		}
		lines[index] = prefix + request.Value
	case security.RepositoryRemove:
		if index < 0 {
			return errors.New("repository was not found in the selected configuration file")
		}
		lines = append(lines[:index], lines[index+1:]...)
	case security.RepositoryEnable, security.RepositoryDisable:
		if index < 0 {
			return errors.New("repository was not found in the selected configuration file")
		}
		prefix := "repository="
		if request.Operation == security.RepositoryDisable {
			prefix = "#repository="
		}
		lines[index] = prefix + request.Value
	}
	return writeWithBackup(path, []byte(strings.Join(lines, "\n")+"\n"))
}

func (m *Manager) repositoryConfig(name string) ([]byte, string, error) {
	local := filepath.Join(m.configurationDir, name)
	contents, err := readRegular(local)
	if err != nil || contents != nil {
		return contents, local, err
	}
	vendor := filepath.Join(m.vendorDir, name)
	contents, err = readRegular(vendor)
	if err != nil || contents != nil {
		return contents, vendor, err
	}
	return nil, "", nil
}

func (m *Manager) repositoryExists(url, excludedFile string) bool {
	files := map[string]string{}
	for _, dir := range []string{m.vendorDir, m.configurationDir} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.conf"))
		for _, path := range paths {
			files[filepath.Base(path)] = path
		}
	}
	for name, path := range files {
		if name == excludedFile {
			continue
		}
		contents, err := readRegular(path)
		if err == nil && repositoryLine(nonemptyLines(contents), url) >= 0 {
			return true
		}
	}
	return false
}

func repositoryLine(lines []string, url string) int {
	for index, line := range lines {
		line = strings.TrimSpace(line)
		if line == "repository="+url || line == "#repository="+url {
			return index
		}
	}
	return -1
}

func (m *Manager) changeMirror(mirror string) error {
	paths, err := filepath.Glob(filepath.Join(m.vendorDir, "*-repository-*.conf"))
	if err != nil || len(paths) == 0 {
		return errors.New("Void vendor repository configuration was not found")
	}
	type priorFile struct {
		path     string
		contents []byte
		existed  bool
	}
	prior := make([]priorFile, 0, len(paths))
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, file := range prior {
			if file.existed {
				_ = atomicWrite(file.path, file.contents)
			} else {
				_ = os.Remove(file.path)
			}
		}
	}()
	for _, source := range paths {
		contents, err := os.ReadFile(source)
		if err != nil {
			return fmt.Errorf("read vendor repository configuration: %w", err)
		}
		target := filepath.Join(m.configurationDir, filepath.Base(source))
		existing, err := readRegular(target)
		if err != nil {
			return err
		}
		prior = append(prior, priorFile{path: target, contents: existing, existed: len(existing) > 0})
		updated := strings.ReplaceAll(string(contents), defaultMirror, mirror)
		if err := writeWithBackup(target, []byte(managedMarker+"\n"+updated)); err != nil {
			return err
		}
	}
	if _, err := m.runner.Run(context.Background(), "/usr/bin/xbps-install", "--sync"); err != nil {
		return fmt.Errorf("refresh XBPS repositories: %w", err)
	}
	committed = true
	return nil
}

func (m *Manager) changeSamba(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.sambaConfig)
	if err != nil {
		return err
	}
	name := request.Value
	if request.Operation == security.SambaEdit {
		name = request.OldValue
	}
	start, end := sambaSection(contents, name)
	if request.Operation == security.SambaAdd && start >= 0 {
		return errors.New("Samba share already exists")
	}
	if request.Operation != security.SambaAdd && start < 0 {
		return errors.New("Samba share was not found")
	}
	if request.Operation == security.SambaEdit && !strings.EqualFold(request.OldValue, request.Value) {
		if duplicate, _ := sambaSection(contents, request.Value); duplicate >= 0 {
			return errors.New("replacement Samba share already exists")
		}
	}
	block := ""
	if request.Operation != security.SambaRemove {
		var preserved []string
		if start >= 0 {
			preserved = unknownSambaOptions(strings.Split(string(contents), "\n")[start+1 : end])
		}
		block = sambaBlock(request, preserved)
	}
	updated := string(contents)
	if request.Operation == security.SambaAdd {
		updated = strings.TrimRight(updated, "\n") + "\n\n" + block
	} else {
		lines := strings.Split(updated, "\n")
		lines = append(lines[:start], append(strings.Split(strings.TrimSuffix(block, "\n"), "\n"), lines[end:]...)...)
		updated = strings.Trim(strings.Join(lines, "\n"), "\n") + "\n"
	}
	return m.saveSamba(contents, []byte(updated))
}

func (m *Manager) changeSambaGlobal(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.sambaConfig)
	if err != nil {
		return err
	}
	lines := removeManagedSambaPrinting(strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n"))
	start, end := sambaSection([]byte(strings.Join(lines, "\n")), "global")
	if start < 0 {
		lines = append([]string{"[global]"}, lines...)
		start, end = 0, 1
	}
	known := map[string]bool{"server min protocol": true, "client min protocol": true, "create mask": true, "directory mask": true, "map to guest": true}
	global := []string{lines[start]}
	for _, line := range lines[start+1 : end] {
		key, _, ok := strings.Cut(line, "=")
		if ok && known[strings.ToLower(strings.TrimSpace(key))] {
			continue
		}
		global = append(global, line)
	}
	global = append(global,
		"server min protocol = "+request.Protocol,
		"client min protocol = "+request.Protocol,
		"create mask = "+request.DefaultMask,
		"directory mask = "+request.DefaultMask,
		"map to guest = "+mapGuestValue(request.MapGuest),
	)
	if request.Printing {
		global = append(global, "# BEGIN SYSENTRA PRINTING", "printing = cups", "printcap name = cups", "load printers = yes", "# END SYSENTRA PRINTING")
	}
	lines = append(lines[:start], append(global, lines[end:]...)...)
	if request.Printing {
		joined := "\n" + strings.ToLower(strings.Join(lines, "\n")) + "\n"
		if !strings.Contains(joined, "\n[printers]\n") {
			lines = append(lines, "", "# BEGIN SYSENTRA PRINTING", "[printers]", "comment = All Printers", "path = /var/spool/samba", "browseable = no", "guest ok = no", "writable = no", "printable = yes", "create mask = 0700", "# END SYSENTRA PRINTING")
		}
		if !strings.Contains(joined, "\n[print$]\n") {
			lines = append(lines, "", "# BEGIN SYSENTRA PRINTING", "[print$]", "comment = Printer Drivers", "path = /var/lib/samba/printers", "browseable = yes", "read only = yes", "guest ok = no", "write list = root", "# END SYSENTRA PRINTING")
		}
	}
	return m.saveSamba(contents, []byte(strings.Trim(strings.Join(lines, "\n"), "\n")+"\n"))
}

func removeManagedSambaPrinting(lines []string) []string {
	result := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "# BEGIN SYSENTRA PRINTING" {
			result = append(result, lines[index])
			continue
		}
		end := -1
		for candidate := index + 1; candidate < len(lines); candidate++ {
			if strings.TrimSpace(lines[candidate]) == "# END SYSENTRA PRINTING" {
				end = candidate
				break
			}
		}
		if end < 0 {
			result = append(result, lines[index])
			continue
		}
		index = end
	}
	return result
}

func mapGuestValue(enabled bool) string {
	if enabled {
		return "Bad User"
	}
	return "Never"
}

func (m *Manager) saveSamba(original, updated []byte) error {
	if err := m.validateSamba(updated); err != nil {
		return err
	}
	if err := writeWithBackup(m.sambaConfig, updated); err != nil {
		return err
	}
	status, _ := m.runner.Run(context.Background(), "/usr/bin/sv", "status", "/etc/runit/runsvdir/default/smbd")
	if !strings.HasPrefix(strings.TrimSpace(string(status.Stdout)), "run:") {
		return nil
	}
	if _, err := m.runner.Run(context.Background(), "/usr/bin/smbcontrol", "all", "reload-config"); err != nil {
		if rollbackErr := atomicWrite(m.sambaConfig, original); rollbackErr != nil {
			return fmt.Errorf("reload Samba configuration: %w; rollback failed: %v", err, rollbackErr)
		}
		return fmt.Errorf("reload Samba configuration: %w", err)
	}
	return nil
}

func sambaBlock(request security.SystemConfigRequest, preserved []string) string {
	createMask, directoryMask := request.CreateMask, request.DirectoryMask
	if createMask == "" {
		createMask = "0744"
	}
	if directoryMask == "" {
		directoryMask = "0755"
	}
	lines := []string{
		"[" + request.Value + "]",
		"path = " + request.Path,
	}
	optional := [][2]string{
		{"comment", request.Comment},
		{"valid users", request.ValidUsers},
		{"write list", request.WriteList},
		{"read list", request.ReadList},
		{"force user", request.ForceUser},
		{"force group", request.ForceGroup},
	}
	for _, option := range optional {
		if option[1] != "" {
			lines = append(lines, option[0]+" = "+option[1])
		}
	}
	lines = append(lines,
		"browseable = "+yesNo(request.Browseable),
		"read only = "+yesNo(request.ReadOnly),
		"guest ok = "+yesNo(request.GuestOK),
		"follow symlinks = "+yesNo(request.FollowSymlinks),
		"wide links = "+yesNo(request.WideLinks),
		"create mask = "+createMask,
		"directory mask = "+directoryMask,
	)
	lines = append(lines, preserved...)
	return strings.Join(lines, "\n") + "\n"
}

func unknownSambaOptions(lines []string) []string {
	known := map[string]bool{
		"path": true, "comment": true, "browseable": true, "browsable": true,
		"read only": true, "readonly": true, "writeable": true, "writable": true,
		"guest ok": true, "public": true, "follow symlinks": true, "wide links": true,
		"valid users": true, "write list": true, "read list": true, "force user": true,
		"force group": true, "create mask": true, "create mode": true,
		"directory mask": true, "directory mode": true,
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		key, _, ok := strings.Cut(line, "=")
		if ok && known[strings.ToLower(strings.TrimSpace(key))] {
			continue
		}
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result
}

func sambaSection(contents []byte, name string) (int, int) {
	lines := strings.Split(string(contents), "\n")
	start := -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			if start >= 0 {
				return start, index
			}
			if strings.EqualFold(strings.TrimSpace(trimmed[1:len(trimmed)-1]), name) {
				start = index
			}
		}
	}
	return start, len(lines)
}

func (m *Manager) validateSamba(contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(m.sambaConfig), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(m.sambaConfig), ".sysentra-smb-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err = temporary.Write(contents); err == nil {
		err = temporary.Close()
	} else {
		temporary.Close()
	}
	if err != nil {
		return err
	}
	if result, err := m.runner.Run(context.Background(), "/usr/bin/testparm", "-s", name); err != nil {
		return fmt.Errorf("validate Samba configuration: %s: %w", strings.TrimSpace(string(result.Stderr)), err)
	}
	return nil
}

func (m *Manager) changeNFS(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.fstab)
	if err != nil {
		return err
	}
	oldSource, oldTarget := request.Value, request.Path
	if request.Operation == security.NFSEdit {
		oldSource, oldTarget = request.OldValue, request.OldPath
	}
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	index := nfsEntry(lines, oldSource, oldTarget)
	if request.Operation == security.NFSAdd && index >= 0 {
		return errors.New("NFS source and mount point already exist")
	}
	if request.Operation != security.NFSAdd && index < 0 {
		return errors.New("NFS entry was not found")
	}
	if request.Operation != security.NFSAdd && m.isNFSMounted(oldSource, oldTarget) {
		return errors.New("unmount the NFS filesystem before changing persistent configuration")
	}
	if request.Operation != security.NFSRemove {
		for candidate, line := range lines {
			fields := strings.Fields(line)
			if candidate != index && len(fields) >= 3 && (fields[2] == "nfs" || fields[2] == "nfs4") && (fields[0] == escapeFstab(request.Value) || fields[1] == escapeFstab(request.Path)) {
				return errors.New("NFS source or mount point already exists")
			}
		}
		if err := os.MkdirAll(request.Path, 0o755); err != nil {
			return fmt.Errorf("create NFS mount point: %w", err)
		}
	}
	if request.Operation == security.NFSRemove {
		lines = append(lines[:index], lines[index+1:]...)
	} else {
		options := request.Options
		if options == "" {
			options = "defaults,_netdev"
		}
		line := fmt.Sprintf("%s %s %s %s 0 0 # sysentra", escapeFstab(request.Value), escapeFstab(request.Path), request.Type, options)
		if request.Operation == security.NFSAdd {
			lines = append(lines, line)
		} else {
			lines[index] = line
		}
	}
	return writeWithBackup(m.fstab, []byte(strings.Trim(strings.Join(lines, "\n"), "\n")+"\n"))
}

func nfsEntry(lines []string, source, target string) int {
	for index, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && (fields[2] == "nfs" || fields[2] == "nfs4") && fields[0] == escapeFstab(source) && fields[1] == escapeFstab(target) {
			return index
		}
	}
	return -1
}

func (m *Manager) mountNFS(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.fstab)
	if err != nil {
		return err
	}
	if nfsEntry(strings.Split(string(contents), "\n"), request.Value, request.Path) < 0 {
		return errors.New("NFS entry is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	result, err := m.runner.Run(ctx, "/usr/bin/mount", "--", request.Path)
	cancel()
	if err != nil {
		rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, _ = m.runner.Run(rollbackCtx, "/usr/bin/umount", "--", request.Path)
		rollbackCancel()
		return commandFailure("mount NFS filesystem", result, err)
	}
	return nil
}

func (m *Manager) unmountNFS(request security.SystemConfigRequest) error {
	if !m.isNFSMounted(request.Value, request.Path) {
		return errors.New("requested NFS filesystem is not mounted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if result, err := m.runner.Run(ctx, "/usr/bin/umount", "--", request.Path); err != nil {
		return commandFailure("unmount NFS filesystem", result, err)
	}
	return nil
}

func commandFailure(action string, result runner.Result, err error) error {
	diagnostic := strings.TrimSpace(string(result.Stderr))
	if diagnostic != "" {
		return fmt.Errorf("%s: %s: %w", action, diagnostic, err)
	}
	return fmt.Errorf("%s: %w", action, err)
}

func (m *Manager) isNFSMounted(source, target string) bool {
	contents, err := os.ReadFile(m.mounts)
	return err == nil && nfsEntry(strings.Split(string(contents), "\n"), source, target) >= 0
}

func nonemptyLines(contents []byte) []string {
	if len(contents) == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}
func escapeFstab(value string) string { return strings.ReplaceAll(value, " ", `\040`) }

func readRegular(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return os.ReadFile(path)
}

func writeWithBackup(path string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := readRegular(path)
	if err != nil {
		return err
	}
	if len(existing) > 0 {
		if err := atomicWrite(path+".bak", existing); err != nil {
			return fmt.Errorf("back up %s: %w", path, err)
		}
	}
	return atomicWrite(path, contents)
}

func atomicWrite(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sysentra-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0o644); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
