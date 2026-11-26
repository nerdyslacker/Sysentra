package xbpsconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sysentra/internal/security"
)

const ManagedFile = "sysentra-rules.conf"

type Manager struct {
	dir       string
	vendorDir string
}

func NewManager(dir string, vendorDir ...string) *Manager {
	vendor := "/usr/share/xbps.d"
	if len(vendorDir) > 0 {
		vendor = vendorDir[0]
	}
	return &Manager{dir: dir, vendorDir: vendor}
}

func (m *Manager) Change(request security.XBPSRuleRequest) error {
	if err := request.Validate(); err != nil {
		return err
	}
	name := request.File
	if request.Operation == security.RuleAdd {
		name = ManagedFile
	}
	path := filepath.Join(m.dir, name)
	contents, err := readManagedFile(path)
	if err != nil {
		return err
	}
	localContents := contents
	if contents == nil && request.Operation != security.RuleAdd {
		contents, err = readManagedFile(filepath.Join(m.vendorDir, name))
		if err != nil {
			return err
		}
		if contents == nil {
			return errors.New("XBPS configuration file was not found")
		}
	}
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	if len(contents) == 0 {
		lines = []string{"# Managed by Sysentra. Manual changes may be replaced."}
	}
	oldLine := string(request.Kind) + "=" + request.Value
	newLine := oldLine
	if request.Operation == security.RuleEdit {
		oldLine = string(request.Kind) + "=" + request.OldValue
	}
	index := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == oldLine {
			index = i
			break
		}
	}
	switch request.Operation {
	case security.RuleAdd:
		if index >= 0 {
			return errors.New("XBPS rule already exists")
		}
		lines = append(lines, newLine)
	case security.RuleEdit:
		if index < 0 {
			return errors.New("XBPS rule to edit was not found")
		}
		for _, line := range lines {
			if strings.TrimSpace(line) == newLine {
				return errors.New("replacement XBPS rule already exists")
			}
		}
		lines[index] = newLine
	case security.RuleRemove:
		if index < 0 {
			return errors.New("XBPS rule to remove was not found")
		}
		lines = append(lines[:index], lines[index+1:]...)
	}
	if len(localContents) > 0 {
		if err := atomicWrite(path+".bak", localContents); err != nil {
			return fmt.Errorf("back up XBPS rules: %w", err)
		}
	}
	if err := atomicWrite(path, []byte(strings.Join(lines, "\n")+"\n")); err != nil {
		return fmt.Errorf("write XBPS rules: %w", err)
	}
	return nil
}

func readManagedFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("managed XBPS configuration is not a regular file")
	}
	return os.ReadFile(path)
}

func atomicWrite(path string, contents []byte) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sysentra-rules-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
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
	return os.Rename(temporaryPath, path)
}
