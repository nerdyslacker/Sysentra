package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const managedMarker = "X-Sysentra-Managed=true"

type Manager struct {
	path       string
	executable string
}

func New() (*Manager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("locate user configuration directory: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate Sysentra executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return nil, fmt.Errorf("resolve Sysentra executable: %w", err)
	}
	return &Manager{path: filepath.Join(dir, "autostart", "sysentra.desktop"), executable: executable}, nil
}

func (m *Manager) Enabled() (bool, error) {
	contents, err := os.ReadFile(m.path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read Sysentra autostart entry: %w", err)
	}
	return strings.Contains(string(contents), managedMarker), nil
}

func (m *Manager) Set(enabled bool) error {
	contents, err := os.ReadFile(m.path)
	if err == nil && !strings.Contains(string(contents), managedMarker) {
		return fmt.Errorf("refusing to replace non-Sysentra autostart entry %s", m.path)
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read Sysentra autostart entry: %w", err)
	}
	if !enabled {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err := os.Remove(m.path); err != nil {
			return fmt.Errorf("remove Sysentra autostart entry: %w", err)
		}
		return nil
	}
	if strings.ContainsAny(m.executable, "\n\r") {
		return fmt.Errorf("invalid Sysentra executable path")
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}
	entry := fmt.Sprintf("[Desktop Entry]\nType=Application\nName=Sysentra\nComment=System management center for Void Linux\nExec=\"%s\" --background\nIcon=io.github.nerdyslacker.Sysentra\nTerminal=false\nX-GNOME-Autostart-enabled=true\n%s\n", escapeExec(m.executable), managedMarker)
	temporary, err := os.CreateTemp(filepath.Dir(m.path), ".sysentra-autostart-*")
	if err != nil {
		return fmt.Errorf("create Sysentra autostart entry: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(entry); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, m.path); err != nil {
		return fmt.Errorf("replace Sysentra autostart entry: %w", err)
	}
	return nil
}

func escapeExec(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.ReplaceAll(value, "`", "\\`")
	return strings.ReplaceAll(value, "$", `\$`)
}
