package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"sysentra/internal/core"
	"sysentra/internal/runner"
)

type Flatpak struct {
	runner      runner.Runner
	iconRoots   []string
	metadataURL string
	httpClient  *http.Client
}

type FlatpakOption func(*Flatpak)

func WithFlathubMetadata(baseURL string, client *http.Client) FlatpakOption {
	return func(flatpak *Flatpak) {
		flatpak.metadataURL, flatpak.httpClient = strings.TrimRight(baseURL, "/"), client
	}
}

type flatpakItem struct {
	ID           string `json:"application_id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	Version      string `json:"version"`
	Architecture string `json:"arch"`
	Origin       string `json:"origin"`
	Installation string `json:"installation"`
}

func NewFlatpak(commandRunner runner.Runner, options ...FlatpakOption) *Flatpak {
	roots := []string{"/var/lib/flatpak/exports/share/icons"}
	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".local/share/flatpak/exports/share/icons"))
	}
	flatpak := &Flatpak{runner: commandRunner, iconRoots: roots}
	for _, option := range options {
		option(flatpak)
	}
	return flatpak
}

func (*Flatpak) ID() string { return "flatpak" }

func (f *Flatpak) ListInstalled(ctx context.Context) ([]core.Package, error) {
	return f.list(ctx, true, "list", "--app", "--columns=name,description,application,version,branch,arch,origin,installation", "-j")
}

func (f *Flatpak) ListAvailable(ctx context.Context) ([]core.Package, error) {
	return f.list(ctx, false, "remote-ls", "--app", "--columns=name,description,application,version,branch,arch,origin", "-j")
}

func (f *Flatpak) ListUpdates(ctx context.Context) ([]core.Package, error) {
	packages, err := f.list(ctx, true, "remote-ls", "--updates", "--app", "--columns=name,description,application,version,branch,arch,origin", "-j")
	if err != nil {
		return nil, err
	}
	for index := range packages {
		packages[index].NewVersion = packages[index].Version
		packages[index].Version = ""
	}
	return packages, nil
}

func (f *Flatpak) Get(ctx context.Context, ref core.PackageRef) (core.PackageDetails, error) {
	if ref.BackendID != f.ID() || ref.ID == "" {
		return core.PackageDetails{}, ErrPackageNotFound
	}
	packages, err := f.ListInstalled(ctx)
	if err != nil {
		return core.PackageDetails{}, err
	}
	pkg, found := findPackage(packages, ref)
	if !found {
		packages, err = f.ListAvailable(ctx)
		if err != nil {
			return core.PackageDetails{}, err
		}
		pkg, found = findPackage(packages, ref)
	}
	if !found {
		return core.PackageDetails{}, ErrPackageNotFound
	}
	details := core.PackageDetails{Package: pkg, Description: pkg.Description}
	details.IconDataURL = resolveIcon([]string{ref.ID}, f.iconRoots)
	if pkg.Origin == "flathub" {
		f.enrichFromFlathub(ctx, &details)
	}
	if !pkg.Installed {
		return details, nil
	}
	if result, err := f.runner.Run(ctx, "flatpak", "info", "--show-size", ref.ID); err == nil {
		details.InstalledSize = formatByteSize(strings.TrimSpace(string(result.Stdout)))
	}
	if result, err := f.runner.Run(ctx, "flatpak", "info", "--show-runtime", ref.ID); err == nil {
		if runtime := strings.TrimSpace(string(result.Stdout)); runtime != "" {
			details.Requires = []string{runtime}
		}
	}
	return details, nil
}

type flathubMetadata struct {
	Description    string `json:"description"`
	Summary        string `json:"summary"`
	Developer      string `json:"developer_name"`
	ProjectLicense string `json:"project_license"`
	URLs           struct {
		Homepage string `json:"homepage"`
	} `json:"urls"`
	Bundle struct {
		Runtime string `json:"runtime"`
	} `json:"bundle"`
}

var htmlTag = regexp.MustCompile(`<[^>]+>`)

func (f *Flatpak) enrichFromFlathub(ctx context.Context, details *core.PackageDetails) {
	if f.httpClient == nil || f.metadataURL == "" {
		return
	}
	requestCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, f.metadataURL+"/"+url.PathEscape(details.Ref.ID)+"?locale=en", nil)
	if err != nil {
		return
	}
	response, err := f.httpClient.Do(request)
	if err != nil {
		return
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return
	}
	var metadata flathubMetadata
	if json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&metadata) != nil {
		return
	}
	if description := strings.TrimSpace(html.UnescapeString(htmlTag.ReplaceAllString(metadata.Description, " "))); description != "" {
		details.Description = strings.Join(strings.Fields(description), " ")
	}
	if details.Description == "" {
		details.Description = metadata.Summary
	}
	details.Homepage, details.License, details.Maintainer = metadata.URLs.Homepage, metadata.ProjectLicense, metadata.Developer
	if metadata.Bundle.Runtime != "" && len(details.Requires) == 0 {
		details.Requires = []string{metadata.Bundle.Runtime}
	}
}

func findPackage(packages []core.Package, ref core.PackageRef) (core.Package, bool) {
	for _, pkg := range packages {
		if pkg.Ref == ref {
			return pkg, true
		}
	}
	return core.Package{}, false
}

func formatByteSize(value string) string {
	bytes, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return ""
	}
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exponent := uint64(unit), 0
	for amount := bytes / unit; amount >= unit && exponent < 4; amount /= unit {
		div *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exponent])
}

func (f *Flatpak) list(ctx context.Context, installed bool, args ...string) ([]core.Package, error) {
	result, err := f.runner.Run(ctx, "flatpak", args...)
	if err != nil {
		return nil, fmt.Errorf("flatpak %v: %w", args, err)
	}
	var items []flatpakItem
	if err := json.Unmarshal(result.Stdout, &items); err != nil {
		return nil, fmt.Errorf("parse Flatpak output: %w", err)
	}
	packages := make([]core.Package, len(items))
	for index, item := range items {
		name := item.Name
		if name == "" {
			name = item.ID
		}
		scope := item.Installation
		if scope == "" {
			scope = "system"
		}
		packages[index] = core.Package{
			Ref:          core.PackageRef{BackendID: "flatpak", ID: item.ID},
			Name:         name,
			Description:  item.Description,
			Version:      item.Version,
			Architecture: item.Architecture,
			Origin:       item.Origin,
			Scope:        scope,
			Installed:    installed,
		}
	}
	return packages, nil
}
