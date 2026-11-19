package systemconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	grubMkconfig = "/usr/bin/grub-mkconfig"
	grubEditenv  = "/usr/bin/grub-editenv"
)

var grubID = regexp.MustCompile(`(?:--id|\$menuentry_id_option)\s+['"]([^'"]+)['"]`)

func (m *Manager) setDefaultKernel(version string) error {
	grubConfigs := existingFiles(filepath.Join(m.bootRoot, "grub", "grub.cfg"), filepath.Join(m.bootRoot, "grub2", "grub.cfg"))
	loaderDirs := existingDirs(filepath.Join(m.bootRoot, "loader", "entries"), filepath.Join(m.efiMount, "loader", "entries"))
	if len(grubConfigs)+len(loaderDirs) != 1 {
		return errors.New("exactly one supported bootloader configuration is required")
	}
	if len(grubConfigs) == 1 {
		return m.setGRUBDefault(grubConfigs[0], version)
	}
	return m.setSystemdBootDefault(loaderDirs[0], version)
}

func (m *Manager) setSystemdBootDefault(entriesDir, version string) error {
	matches, err := systemdBootEntries(entriesDir, version)
	if err != nil {
		return err
	}
	if len(matches) != 1 {
		return fmt.Errorf("kernel %s maps to %d systemd-boot entries", version, len(matches))
	}
	loaderConfig := filepath.Join(filepath.Dir(entriesDir), "loader.conf")
	_, statErr := os.Stat(loaderConfig)
	existed := statErr == nil
	prior, err := readRegular(loaderConfig)
	if err != nil {
		return err
	}
	updated := setDirective(prior, "default", matches[0])
	if err := writeWithBackup(loaderConfig, updated); err != nil {
		return err
	}
	verified, err := readRegular(loaderConfig)
	if err != nil || directive(verified, "default") != matches[0] {
		if existed {
			_ = atomicWrite(loaderConfig, prior)
		} else {
			_ = os.Remove(loaderConfig)
		}
		return errors.New("systemd-boot default entry verification failed")
	}
	return nil
}

func (m *Manager) setGRUBDefault(config, version string) error {
	contents, err := os.ReadFile(config)
	if err != nil {
		return err
	}
	entries := grubEntries(contents, version)
	if len(entries) != 1 {
		return fmt.Errorf("kernel %s maps to %d non-recovery GRUB entries", version, len(entries))
	}
	priorDefaults, err := readRegular(m.grubDefaults)
	if err != nil || len(priorDefaults) == 0 {
		return errors.New("GRUB defaults configuration was not found")
	}
	if err := writeWithBackup(m.grubDefaults, setDirective(priorDefaults, "GRUB_DEFAULT", "saved")); err != nil {
		return err
	}
	rollback := func() {
		_ = atomicWrite(m.grubDefaults, priorDefaults)
		_ = atomicWrite(config, contents)
	}
	temporary, err := os.CreateTemp(filepath.Dir(config), ".sysentra-grub-*")
	if err != nil {
		rollback()
		return err
	}
	temporaryConfig := temporary.Name()
	temporary.Close()
	defer os.Remove(temporaryConfig)
	if result, runErr := m.runner.Run(context.Background(), grubMkconfig, "-o", temporaryConfig); runErr != nil {
		rollback()
		return fmt.Errorf("regenerate GRUB configuration: %w: %s", runErr, strings.TrimSpace(string(result.Stderr)))
	}
	generated, err := os.ReadFile(temporaryConfig)
	if err != nil {
		rollback()
		return err
	}
	entries = grubEntries(generated, version)
	if len(entries) != 1 || !strings.Contains(string(generated), "saved_entry") {
		rollback()
		return errors.New("regenerated GRUB configuration did not preserve one saved-default entry")
	}
	if err := writeWithBackup(config, generated); err != nil {
		rollback()
		return err
	}
	env := filepath.Join(filepath.Dir(config), "grubenv")
	_, envErr := os.Stat(env)
	envExisted := envErr == nil
	priorSaved := ""
	if envExisted {
		result, listErr := m.runner.Run(context.Background(), grubEditenv, env, "list")
		if listErr != nil {
			rollback()
			return fmt.Errorf("read GRUB environment: %w", listErr)
		}
		priorSaved = directive(result.Stdout, "saved_entry")
	} else if errors.Is(envErr, os.ErrNotExist) {
		if _, err := m.runner.Run(context.Background(), grubEditenv, env, "create"); err != nil {
			rollback()
			return fmt.Errorf("create GRUB environment: %w", err)
		}
	} else {
		rollback()
		return envErr
	}
	restore := func() {
		rollback()
		if !envExisted {
			_ = os.Remove(env)
		} else if priorSaved == "" {
			_, _ = m.runner.Run(context.Background(), grubEditenv, env, "unset", "saved_entry")
		} else {
			_, _ = m.runner.Run(context.Background(), grubEditenv, env, "set", "saved_entry="+priorSaved)
		}
	}
	if _, err := m.runner.Run(context.Background(), grubEditenv, env, "set", "saved_entry="+entries[0]); err != nil {
		restore()
		return fmt.Errorf("set GRUB default entry: %w", err)
	}
	result, err := m.runner.Run(context.Background(), grubEditenv, env, "list")
	if err != nil || directive(result.Stdout, "saved_entry") != entries[0] {
		restore()
		return errors.New("GRUB saved default verification failed")
	}
	return nil
}

func grubEntries(contents []byte, version string) []string {
	entries := []string{}
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "menuentry ") || strings.Contains(strings.ToLower(trimmed), "recovery") || !containsVersion(trimmed, version) {
			continue
		}
		match := grubID.FindStringSubmatch(trimmed)
		if len(match) == 2 {
			entries = append(entries, match[1])
		}
	}
	return entries
}

func systemdBootEntries(dir, version string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.conf"))
	if err != nil {
		return nil, err
	}
	matches := []string{}
	for _, path := range paths {
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		linux := directive(contents, "linux")
		if filepath.Base(linux) == "vmlinuz-"+version {
			matches = append(matches, strings.TrimSuffix(filepath.Base(path), ".conf"))
		}
	}
	return matches, nil
}

func containsVersion(value, version string) bool {
	for start := 0; ; {
		index := strings.Index(value[start:], version)
		if index < 0 {
			return false
		}
		index += start
		before := index == 0 || !versionChar(value[index-1])
		afterIndex := index + len(version)
		after := afterIndex == len(value) || !versionChar(value[afterIndex])
		if before && after {
			return true
		}
		start = index + 1
	}
}

func versionChar(value byte) bool {
	return value >= 'a' && value <= 'z' || value >= 'A' && value <= 'Z' || value >= '0' && value <= '9' || strings.ContainsRune("._-", rune(value))
}

func directive(contents []byte, key string) string {
	for _, line := range strings.Split(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && strings.EqualFold(strings.TrimSuffix(fields[0], "="), key) {
			return strings.Trim(strings.Join(fields[1:], " "), "'\"")
		}
		if strings.HasPrefix(strings.TrimSpace(line), key+"=") {
			return strings.Trim(strings.TrimSpace(strings.SplitN(line, "=", 2)[1]), "'\"")
		}
	}
	return ""
}

func setDirective(contents []byte, key, value string) []byte {
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	found := false
	separator := " "
	if strings.HasPrefix(key, "GRUB_") {
		separator = "="
	}
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, key+"=") || strings.HasPrefix(trimmed, key+" ") {
			lines[index] = key + separator + value
			found = true
		}
	}
	if !found {
		lines = append(lines, key+separator+value)
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func existingFiles(paths ...string) []string { return existingPaths(false, paths...) }
func existingDirs(paths ...string) []string  { return existingPaths(true, paths...) }
func existingPaths(dirs bool, paths ...string) []string {
	found := []string{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err == nil && info.IsDir() == dirs {
			found = append(found, path)
		}
	}
	return found
}
