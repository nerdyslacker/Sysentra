package flatpakremote

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"
	"strings"

	"sysentra/internal/runner"
)

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type Remote struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Options string `json:"options"`
}

func (r Remote) Enabled() bool { return !strings.Contains(r.Options, "disabled") }

func (r Remote) Scope() string {
	if strings.Contains(r.Options, "system") {
		return "System"
	}
	return "User"
}

type Manager struct{ runner runner.Runner }

func New(commandRunner runner.Runner) *Manager { return &Manager{runner: commandRunner} }

func (m *Manager) List(ctx context.Context) ([]Remote, error) {
	result, err := m.runner.Run(ctx, "flatpak", "remotes", "--show-disabled", "--columns=name,title,url,options", "--json")
	if err != nil {
		return nil, err
	}
	var remotes []Remote
	if err := json.Unmarshal(result.Stdout, &remotes); err != nil {
		return nil, err
	}
	return remotes, nil
}

func (m *Manager) Change(ctx context.Context, operation, name, remoteURL string) error {
	if !namePattern.MatchString(name) {
		return errors.New("invalid Flatpak remote name")
	}
	var args []string
	switch operation {
	case "add":
		if !validURL(remoteURL) {
			return errors.New("a valid HTTPS Flatpak repository URL is required")
		}
		args = []string{"remote-add", "--user", "--if-not-exists", name, remoteURL}
	case "edit":
		if !validURL(remoteURL) {
			return errors.New("a valid HTTPS Flatpak repository URL is required")
		}
		args = []string{"remote-modify", "--user", "--url=" + remoteURL, name}
	case "enable":
		args = []string{"remote-modify", "--user", "--enable", name}
	case "disable":
		args = []string{"remote-modify", "--user", "--disable", name}
	case "remove":
		args = []string{"remote-delete", "--user", "--force", name}
	default:
		return errors.New("unsupported Flatpak remote operation")
	}
	_, err := m.runner.Run(ctx, "flatpak", args...)
	return err
}

func validURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == ""
}
