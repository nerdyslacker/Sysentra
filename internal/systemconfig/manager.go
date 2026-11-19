package systemconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	bootRoot, efiMount          string
	grubDefaults                string
}

func NewManager(commandRunner runner.Runner, configurationDir, vendorDir, sambaConfig, fstab string) *Manager {
	return &Manager{runner: commandRunner, configurationDir: configurationDir, vendorDir: vendorDir, sambaConfig: sambaConfig, fstab: fstab, bootRoot: "/boot", efiMount: "/boot/efi", grubDefaults: "/etc/default/grub"}
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
	case security.SambaAdd:
		return m.addSamba(request)
	case security.NFSAdd:
		return m.addNFS(request)
	case security.KernelSetDefault:
		return m.setDefaultKernel(request.Value)
	default:
		return errors.New("unsupported system configuration operation")
	}
}

func (m *Manager) changeRepository(request security.SystemConfigRequest) error {
	path := filepath.Join(m.configurationDir, managedRepositories)
	contents, err := readRegular(path)
	if err != nil {
		return err
	}
	lines := nonemptyLines(contents)
	if len(lines) == 0 {
		lines = []string{managedMarker}
	}
	oldURL := request.Value
	if request.Operation == security.RepositoryEdit {
		oldURL = request.OldValue
	}
	index := repositoryLine(lines, oldURL)
	switch request.Operation {
	case security.RepositoryAdd:
		if index >= 0 {
			return errors.New("repository already exists")
		}
		lines = append(lines, "repository="+request.Value)
	case security.RepositoryEdit:
		if index < 0 {
			return errors.New("repository is not managed by Sysentra")
		}
		if repositoryLine(lines, request.Value) >= 0 {
			return errors.New("replacement repository already exists")
		}
		prefix := "repository="
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "#repository=") {
			prefix = "#repository="
		}
		lines[index] = prefix + request.Value
	case security.RepositoryRemove:
		if index < 0 {
			return errors.New("repository is not managed by Sysentra")
		}
		lines = append(lines[:index], lines[index+1:]...)
	case security.RepositoryEnable, security.RepositoryDisable:
		if index < 0 {
			return errors.New("repository is not managed by Sysentra")
		}
		prefix := "repository="
		if request.Operation == security.RepositoryDisable {
			prefix = "#repository="
		}
		lines[index] = prefix + request.Value
	}
	return writeWithBackup(path, []byte(strings.Join(lines, "\n")+"\n"))
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

func (m *Manager) addSamba(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.sambaConfig)
	if err != nil {
		return err
	}
	header := "[" + request.Value + "]"
	for _, line := range strings.Split(string(contents), "\n") {
		if strings.EqualFold(strings.TrimSpace(line), header) {
			return errors.New("Samba share already exists")
		}
	}
	separator := ""
	if len(contents) > 0 && !strings.HasSuffix(string(contents), "\n") {
		separator = "\n"
	}
	block := fmt.Sprintf("%s\n%s\npath = %s\nread only = %s\nguest ok = %s\n", separator, header, request.Path, yesNo(request.ReadOnly), yesNo(request.GuestOK))
	return writeWithBackup(m.sambaConfig, append(contents, []byte(block)...))
}

func (m *Manager) addNFS(request security.SystemConfigRequest) error {
	contents, err := readRegular(m.fstab)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(request.Path, 0o755); err != nil {
		return fmt.Errorf("create NFS mount point: %w", err)
	}
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && (fields[0] == escapeFstab(request.Value) || fields[1] == escapeFstab(request.Path)) {
			return errors.New("NFS source or mount point already exists")
		}
	}
	options := request.Options
	if options == "" {
		options = "defaults,_netdev"
	}
	separator := ""
	if len(contents) > 0 && !strings.HasSuffix(string(contents), "\n") {
		separator = "\n"
	}
	line := fmt.Sprintf("%s%s %s %s %s 0 0 # sysentra\n", separator, escapeFstab(request.Value), escapeFstab(request.Path), request.Type, options)
	return writeWithBackup(m.fstab, append(contents, []byte(line)...))
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
