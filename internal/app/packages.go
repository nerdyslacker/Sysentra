package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"sysentra/internal/core"
)

type packageReader interface {
	ID() string
	ListInstalled(context.Context) ([]core.Package, error)
	ListAvailable(context.Context) ([]core.Package, error)
	ListUpdates(context.Context) ([]core.Package, error)
}

type packageOwner interface {
	FindOwners(context.Context, string) ([]string, error)
}

type packageDetailsReader interface {
	Get(context.Context, core.PackageRef) (core.PackageDetails, error)
}

type PackageView struct {
	BackendID   string `json:"backendId"`
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	NewVersion  string `json:"newVersion"`
	Repository  string `json:"repository"`
	Scope       string `json:"scope"`
	Status      string `json:"status"`
	Installed   bool   `json:"installed"`
	Held        bool   `json:"held"`
	Orphaned    bool   `json:"orphaned"`
	RepoLocked  bool   `json:"repoLocked"`
	Description string `json:"description"`
}

type PackageCatalog struct {
	Packages []PackageView `json:"packages"`
	Warnings []string      `json:"warnings"`
}

type FileOwnerResult struct {
	Matches []string `json:"matches"`
}

type AdditionalSourceView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type PackageDetailsView struct {
	BackendID           string   `json:"backendId"`
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Architecture        string   `json:"architecture"`
	Repository          string   `json:"repository"`
	Scope               string   `json:"scope"`
	Description         string   `json:"description"`
	Homepage            string   `json:"homepage"`
	License             string   `json:"license"`
	IconDataURL         string   `json:"iconDataUrl"`
	DownloadSize        string   `json:"downloadSize"`
	InstalledSize       string   `json:"installedSize"`
	InstallDate         string   `json:"installDate"`
	Maintainer          string   `json:"maintainer"`
	SourcePackage       string   `json:"sourcePackage"`
	Provides            []string `json:"provides"`
	Requires            []string `json:"requires"`
	Exports             []string `json:"exports"`
	Installed           bool     `json:"installed"`
	Dependencies        []string `json:"dependencies"`
	ReverseDependencies []string `json:"reverseDependencies"`
	Files               []string `json:"files"`
	CachedVersions      []string `json:"cachedVersions"`
}

type PackageService struct {
	primary         packageReader
	optional        []packageReader
	additionalMu    sync.RWMutex
	additionalState map[string]bool
}

func NewPackageService(primary packageReader, optional ...packageReader) *PackageService {
	state := make(map[string]bool, len(optional))
	for _, backend := range optional {
		state[backend.ID()] = true
	}
	return &PackageService{primary: primary, optional: optional, additionalState: state}
}

func (s *PackageService) ListAdditionalSources() []AdditionalSourceView {
	s.additionalMu.RLock()
	defer s.additionalMu.RUnlock()
	result := make([]AdditionalSourceView, 0, len(s.optional))
	for _, backend := range s.optional {
		name := backend.ID()
		if name == "flatpak" {
			name = "Flathub / Flatpak"
		}
		result = append(result, AdditionalSourceView{ID: backend.ID(), Name: name, Enabled: s.additionalState[backend.ID()]})
	}
	return result
}

func (s *PackageService) SetAdditionalSourceEnabled(id string, enabled bool) ([]AdditionalSourceView, error) {
	s.additionalMu.Lock()
	if _, exists := s.additionalState[id]; !exists {
		s.additionalMu.Unlock()
		return nil, fmt.Errorf("unknown additional package source %q", id)
	}
	s.additionalState[id] = enabled
	s.additionalMu.Unlock()
	return s.ListAdditionalSources(), nil
}

func (s *PackageService) ListInstalled() (PackageCatalog, error) {
	return s.catalog("installed")
}

func (s *PackageService) Discover() (PackageCatalog, error) {
	return s.catalog("available")
}

func (s *PackageService) ListUpdates() (PackageCatalog, error) {
	return s.catalog("updates")
}

func (s *PackageService) FindOwningPackage(query string) (FileOwnerResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 4096 {
		return FileOwnerResult{}, fmt.Errorf("file path or pattern must contain 1 to 4096 characters")
	}
	owner, ok := s.primary.(packageOwner)
	if !ok {
		return FileOwnerResult{}, fmt.Errorf("the primary package backend does not support file ownership queries")
	}
	matches, err := owner.FindOwners(context.Background(), query)
	if err != nil {
		return FileOwnerResult{}, err
	}
	return FileOwnerResult{Matches: matches}, nil
}

func (s *PackageService) GetPackageDetails(backendID, id string) (PackageDetailsView, error) {
	if backendID == "" || id == "" {
		return PackageDetailsView{}, fmt.Errorf("package backend and ID are required")
	}
	readers := append([]packageReader{s.primary}, s.optional...)
	for _, reader := range readers {
		if reader.ID() != backendID {
			continue
		}
		detailReader, ok := reader.(packageDetailsReader)
		if !ok {
			return PackageDetailsView{}, fmt.Errorf("%s does not provide package details", backendID)
		}
		details, err := detailReader.Get(context.Background(), core.PackageRef{BackendID: backendID, ID: id})
		if err != nil {
			return PackageDetailsView{}, err
		}
		return PackageDetailsView{
			BackendID: backendID, ID: id, Name: details.Name, Version: details.Version,
			Architecture: details.Architecture, Repository: details.Origin, Scope: details.Scope, Description: details.Description,
			Homepage: details.Homepage, License: details.License, Installed: details.Installed,
			IconDataURL: details.IconDataURL, DownloadSize: details.DownloadSize, InstalledSize: details.InstalledSize,
			InstallDate: details.InstallDate, Maintainer: details.Maintainer, SourcePackage: details.SourcePackage,
			Provides: stringsOrEmpty(details.Provides), Requires: stringsOrEmpty(details.Requires), Exports: stringsOrEmpty(details.Exports),
			Dependencies: stringsOrEmpty(details.Dependencies), ReverseDependencies: stringsOrEmpty(details.ReverseDependencies), Files: stringsOrEmpty(details.Files), CachedVersions: stringsOrEmpty(details.CachedVersions),
		}, nil
	}
	return PackageDetailsView{}, fmt.Errorf("unknown package backend %q", backendID)
}

func stringsOrEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func (s *PackageService) catalog(kind string) (PackageCatalog, error) {
	packages, err := loadPackages(context.Background(), s.primary, kind)
	if err != nil {
		return PackageCatalog{}, err
	}
	catalog := PackageCatalog{Packages: packageViews(packages)}
	for _, backend := range s.optional {
		s.additionalMu.RLock()
		enabled := s.additionalState[backend.ID()]
		s.additionalMu.RUnlock()
		if !enabled {
			continue
		}
		packages, err := loadPackages(context.Background(), backend, kind)
		if err != nil {
			if kind != "updates" {
				catalog.Warnings = append(catalog.Warnings, fmt.Sprintf("%s is unavailable", backend.ID()))
			}
			continue
		}
		catalog.Packages = append(catalog.Packages, packageViews(packages)...)
	}
	sort.Slice(catalog.Packages, func(i, j int) bool {
		if catalog.Packages[i].Name == catalog.Packages[j].Name {
			return catalog.Packages[i].BackendID < catalog.Packages[j].BackendID
		}
		return catalog.Packages[i].Name < catalog.Packages[j].Name
	})
	return catalog, nil
}

func loadPackages(ctx context.Context, backend packageReader, kind string) ([]core.Package, error) {
	var packages []core.Package
	var err error
	switch kind {
	case "installed":
		return backend.ListInstalled(ctx)
	case "available":
		packages, err = backend.ListAvailable(ctx)
	default:
		packages, err = backend.ListUpdates(ctx)
	}
	if err != nil {
		return nil, err
	}
	installed, err := backend.ListInstalled(ctx)
	if err != nil {
		return nil, err
	}
	installedByID := make(map[core.PackageRef]core.Package, len(installed))
	for _, pkg := range installed {
		installedByID[pkg.Ref] = pkg
	}
	for index := range packages {
		if current, found := installedByID[packages[index].Ref]; found {
			packages[index].Installed = true
			packages[index].Held = current.Held
			packages[index].Orphaned = current.Orphaned
			packages[index].RepoLocked = current.RepoLocked
			if kind == "updates" {
				packages[index].Version = current.Version
			}
			if packages[index].Scope == "" {
				packages[index].Scope = current.Scope
			}
			if packages[index].Description == "" {
				packages[index].Description = current.Description
			}
			if packages[index].Origin == "" {
				packages[index].Origin = current.Origin
			}
		}
	}
	return packages, nil
}

func packageViews(packages []core.Package) []PackageView {
	views := make([]PackageView, len(packages))
	for index, pkg := range packages {
		status := "Available"
		if pkg.NewVersion != "" {
			status = "Update available"
		} else if pkg.Installed {
			status = "Installed"
		}
		views[index] = PackageView{
			BackendID:   pkg.Ref.BackendID,
			ID:          pkg.Ref.ID,
			Name:        pkg.Name,
			Version:     pkg.Version,
			NewVersion:  pkg.NewVersion,
			Repository:  pkg.Origin,
			Scope:       pkg.Scope,
			Status:      status,
			Installed:   pkg.Installed,
			Held:        pkg.Held,
			Orphaned:    pkg.Orphaned,
			RepoLocked:  pkg.RepoLocked,
			Description: pkg.Description,
		}
	}
	return views
}
