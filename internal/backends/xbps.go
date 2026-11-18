package backends

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sysentra/internal/core"
	"sysentra/internal/runner"
)

var ErrInvalidBackendOutput = errors.New("invalid backend output")

type XBPS struct {
	runner        runner.Runner
	packageDBPath string
	packageStates bool
	iconRoots     []string
	cacheDir      string
}

var _ Backend = (*XBPS)(nil)

func NewXBPS(commandRunner runner.Runner) *XBPS {
	return &XBPS{runner: commandRunner, cacheDir: "/var/cache/xbps", iconRoots: []string{"/usr/share/icons", "/usr/share/pixmaps", "/usr/local/share/icons", "/usr/local/share/pixmaps"}}
}

func NewXBPSWithPackageDB(commandRunner runner.Runner, path string) *XBPS {
	backend := NewXBPS(commandRunner)
	backend.packageDBPath = path
	return backend
}

func NewSystemXBPS(commandRunner runner.Runner) *XBPS {
	paths, _ := filepath.Glob("/var/db/xbps/pkgdb-*.plist")
	if len(paths) == 0 {
		backend := NewXBPS(commandRunner)
		backend.packageStates = true
		return backend
	}
	backend := NewXBPSWithPackageDB(commandRunner, paths[len(paths)-1])
	backend.packageStates = true
	return backend
}

func (*XBPS) ID() string { return "xbps" }

func (*XBPS) Capabilities(context.Context) (core.Capabilities, error) {
	return core.Capabilities{Search: true, ListInstalled: true, Details: true}, nil
}

func (x *XBPS) ListInstalled(ctx context.Context) ([]core.Package, error) {
	result, err := x.runner.Run(ctx, "xbps-query", "--list-pkgs")
	if err != nil {
		return nil, fmt.Errorf("list installed XBPS packages: %w", err)
	}
	packages, err := parseXBPSList(result.Stdout, true)
	if err != nil {
		return nil, err
	}
	if x.packageDBPath != "" {
		origins, err := readPackageOrigins(x.packageDBPath)
		if err != nil {
			return nil, fmt.Errorf("read installed XBPS package repositories: %w", err)
		}
		for index := range packages {
			packages[index].Origin = origins[packages[index].Ref.ID]
		}
	}
	if x.packageStates {
		if err := x.applyPackageStates(ctx, packages); err != nil {
			return nil, err
		}
	}
	return packages, nil
}

func (x *XBPS) applyPackageStates(ctx context.Context, packages []core.Package) error {
	queries := []struct {
		arg string
		set func(*core.Package)
	}{
		{"--list-hold-pkgs", func(pkg *core.Package) { pkg.Held = true }},
		{"--list-orphans", func(pkg *core.Package) { pkg.Orphaned = true }},
		{"--list-repolock-pkgs", func(pkg *core.Package) { pkg.RepoLocked = true }},
	}
	byName := make(map[string]*core.Package, len(packages))
	for index := range packages {
		byName[packages[index].Ref.ID] = &packages[index]
	}
	for _, query := range queries {
		result, err := x.runner.Run(ctx, "xbps-query", query.arg)
		if err != nil {
			return fmt.Errorf("query XBPS package state %s: %w", query.arg, err)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(result.Stdout)))
		for scanner.Scan() {
			name, _, ok := splitXBPSPkgver(strings.TrimSpace(scanner.Text()))
			if !ok {
				return fmt.Errorf("%w: package state line %q", ErrInvalidBackendOutput, scanner.Text())
			}
			if pkg := byName[name]; pkg != nil {
				query.set(pkg)
			}
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("parse XBPS package state: %w", err)
		}
	}
	return nil
}

func readPackageOrigins(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoder := xml.NewDecoder(file)
	origins := map[string]string{}
	depth, packageName, pending := 0, "", false
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return origins, nil
			}
			return nil, err
		}
		switch value := token.(type) {
		case xml.StartElement:
			if value.Name.Local == "dict" {
				depth++
			}
			if value.Name.Local == "key" {
				var key string
				if err := decoder.DecodeElement(&key, &value); err != nil {
					return nil, err
				}
				if depth == 1 {
					packageName = key
				}
				pending = depth == 2 && key == "repository"
			} else if value.Name.Local == "string" && pending {
				var repository string
				if err := decoder.DecodeElement(&repository, &value); err != nil {
					return nil, err
				}
				origins[packageName], pending = repository, false
			}
		case xml.EndElement:
			if value.Name.Local == "dict" {
				depth--
			}
		}
	}
}

func (x *XBPS) Search(ctx context.Context, query string) ([]core.Package, error) {
	result, err := x.runner.Run(ctx, "xbps-query", "--repository", "--search", query)
	if err != nil {
		return nil, fmt.Errorf("search XBPS packages: %w", err)
	}
	return parseXBPSList(result.Stdout, false)
}

func (x *XBPS) ListAvailable(ctx context.Context) ([]core.Package, error) {
	result, err := x.runner.Run(ctx, "xbps-query", "--list-repos")
	if err != nil {
		return nil, fmt.Errorf("list XBPS repositories: %w", err)
	}
	var packages []core.Package
	seen := map[core.PackageRef]bool{}
	for _, repository := range parseRepositoryURLs(result.Stdout) {
		result, err := x.runner.Run(ctx, "xbps-query", "--ignore-conf-repos", "--repository", repository, "--search", "")
		if err != nil {
			return nil, fmt.Errorf("list XBPS packages from %q: %w", repository, err)
		}
		items, err := parseXBPSList(result.Stdout, false)
		if err != nil {
			return nil, err
		}
		for _, pkg := range items {
			if seen[pkg.Ref] {
				continue
			}
			pkg.Origin = repository
			seen[pkg.Ref] = true
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}

func parseRepositoryURLs(output []byte) []string {
	var repositories []string
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		repositories = append(repositories, fields[1])
	}
	return repositories
}

func (x *XBPS) ListUpdates(ctx context.Context) ([]core.Package, error) {
	result, err := x.runner.Run(ctx, "xbps-install", "-Mun")
	if err != nil {
		return nil, fmt.Errorf("check XBPS updates: %w", err)
	}
	var packages []core.Package
	scanner := bufio.NewScanner(strings.NewReader(string(result.Stdout)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 || fields[1] != "update" {
			continue
		}
		name, version, ok := splitXBPSPkgver(fields[0])
		if !ok {
			return nil, fmt.Errorf("%w: update line %q", ErrInvalidBackendOutput, scanner.Text())
		}
		origin := ""
		if len(fields) > 3 {
			origin = fields[3]
		}
		packages = append(packages, core.Package{
			Ref:        core.PackageRef{BackendID: "xbps", ID: name},
			Name:       name,
			NewVersion: version,
			Origin:     origin,
			Installed:  true,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse XBPS updates: %w", err)
	}
	return packages, nil
}

func (x *XBPS) FindOwners(ctx context.Context, query string) ([]string, error) {
	result, err := x.runner.Run(ctx, "xbps-query", "--ownedby", query)
	if err != nil {
		return nil, fmt.Errorf("find package owning %q: %w", query, err)
	}
	var matches []string
	scanner := bufio.NewScanner(strings.NewReader(string(result.Stdout)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			matches = append(matches, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse owning packages: %w", err)
	}
	return matches, nil
}

func (x *XBPS) Get(ctx context.Context, ref core.PackageRef) (core.PackageDetails, error) {
	if ref.BackendID != x.ID() || ref.ID == "" {
		return core.PackageDetails{}, ErrPackageNotFound
	}

	result, err := x.runner.Run(ctx, "xbps-query", "--show", ref.ID)
	var details core.PackageDetails
	if err == nil {
		details, err = parseXBPSShow(result.Stdout, ref)
	} else {
		if err := ctx.Err(); err != nil {
			return core.PackageDetails{}, err
		}
		var commandError interface{ ExitCode() int }
		if !errors.As(err, &commandError) || commandError.ExitCode() != 2 {
			return core.PackageDetails{}, fmt.Errorf("show installed XBPS package %q: %w", ref.ID, err)
		}
		result, err = x.runner.Run(ctx, "xbps-query", "--repository", "--show", ref.ID)
		if err != nil {
			return core.PackageDetails{}, fmt.Errorf("show XBPS package %q: %w", ref.ID, err)
		}
		details, err = parseXBPSShow(result.Stdout, ref)
	}
	if err != nil {
		return core.PackageDetails{}, err
	}

	modePrefix := ""
	if !details.Installed {
		modePrefix = "R"
	}
	details.Dependencies, err = x.queryLines(ctx, "-"+modePrefix+"x", ref.ID)
	if err != nil {
		return core.PackageDetails{}, fmt.Errorf("query dependencies for %q: %w", ref.ID, err)
	}
	details.ReverseDependencies, err = x.queryLines(ctx, "-"+modePrefix+"X", ref.ID)
	if err != nil {
		return core.PackageDetails{}, fmt.Errorf("query reverse dependencies for %q: %w", ref.ID, err)
	}
	if details.Installed {
		details.Files, err = x.queryLines(ctx, "--files", ref.ID)
		if err != nil {
			return core.PackageDetails{}, fmt.Errorf("query files for %q: %w", ref.ID, err)
		}
		details.CachedVersions = x.cachedVersions(ref.ID, details.Version)
	}
	details.IconDataURL = x.packageIcon(details.Name, details.Files)
	return details, nil
}

func (x *XBPS) cachedVersions(packageName, installedVersion string) []string {
	paths, _ := filepath.Glob(filepath.Join(x.cacheDir, packageName+"-*.xbps"))
	seen := map[string]bool{}
	var versions []string
	for _, path := range paths {
		name := strings.TrimSuffix(filepath.Base(path), ".xbps")
		if dot := strings.LastIndex(name, "."); dot > 0 {
			name = name[:dot]
		}
		archivePackage, version, ok := splitXBPSPkgver(name)
		if ok && archivePackage == packageName && version != installedVersion && !seen[version] {
			seen[version] = true
			versions = append(versions, version)
		}
	}
	sort.Strings(versions)
	return versions
}

func (x *XBPS) queryLines(ctx context.Context, args ...string) ([]string, error) {
	result, err := x.runner.Run(ctx, "xbps-query", args...)
	if err != nil {
		if ctx.Err() == nil {
			var commandError interface{ ExitCode() int }
			if errors.As(err, &commandError) && commandError.ExitCode() == 2 {
				return nil, nil
			}
		}
		return nil, err
	}
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(string(result.Stdout)))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func (x *XBPS) packageIcon(packageName string, files []string) string {
	names := desktopIconNames(files)
	names = append(names, packageName)
	return resolveIcon(names, x.iconRoots)
}

func resolveIcon(names, roots []string) string {
	for _, name := range names {
		if !validIconName(name) {
			continue
		}
		for _, root := range roots {
			patterns := []string{
				filepath.Join(root, "*", "scalable", "apps", name+".svg"),
				filepath.Join(root, "*", "128x128", "apps", name+".png"),
				filepath.Join(root, "*", "64x64", "apps", name+".png"),
				filepath.Join(root, "*", "48x48", "apps", name+".png"),
				filepath.Join(root, "*", "*", "apps", name+".svg"),
				filepath.Join(root, name+".svg"), filepath.Join(root, name+".png"),
			}
			for _, pattern := range patterns {
				matches, _ := filepath.Glob(pattern)
				for _, match := range matches {
					if dataURL := iconDataURL(root, match); dataURL != "" {
						return dataURL
					}
				}
			}
		}
	}
	return ""
}

func desktopIconNames(files []string) []string {
	var names []string
	for _, path := range files {
		clean := filepath.Clean(path)
		if filepath.Ext(clean) != ".desktop" || (!strings.HasPrefix(clean, "/usr/share/applications/") && !strings.HasPrefix(clean, "/usr/local/share/applications/")) {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 128*1024 {
			continue
		}
		file, err := os.Open(clean)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(io.LimitReader(file, 128*1024))
		for scanner.Scan() {
			if value, ok := strings.CutPrefix(strings.TrimSpace(scanner.Text()), "Icon="); ok && validIconName(value) {
				names = append(names, value)
				break
			}
		}
		file.Close()
	}
	return names
}

func validIconName(name string) bool {
	if len(name) == 0 || len(name) > 128 {
		return false
	}
	for _, char := range name {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') && !(char >= '0' && char <= '9') && !strings.ContainsRune("._+-", char) {
			return false
		}
	}
	return true
}

func iconDataURL(root, path string) string {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return ""
	}
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return ""
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return ""
	}
	info, err := os.Stat(resolvedPath)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1024*1024 {
		return ""
	}
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return ""
	}
	mime := "image/png"
	if strings.EqualFold(filepath.Ext(resolvedPath), ".svg") {
		mime = "image/svg+xml"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func parseXBPSShow(output []byte, ref core.PackageRef) (core.PackageDetails, error) {
	properties := make(map[string]string)
	lists := make(map[string][]string)
	currentKey := ""
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			if value := strings.TrimSpace(line); currentKey != "" && value != "" {
				lists[currentKey] = append(lists[currentKey], value)
			}
			continue
		}
		key, value, ok := strings.Cut(line, ": ")
		if ok {
			properties[key] = value
			currentKey = key
			continue
		}
		if key, ok := strings.CutSuffix(line, ":"); ok {
			currentKey = key
		}
	}
	if err := scanner.Err(); err != nil {
		return core.PackageDetails{}, fmt.Errorf("parse XBPS package details: %w", err)
	}

	name, version, ok := splitXBPSPkgver(properties["pkgver"])
	if !ok || properties["pkgname"] != ref.ID || name != ref.ID {
		return core.PackageDetails{}, fmt.Errorf("%w: package details for %q", ErrInvalidBackendOutput, ref.ID)
	}
	return core.PackageDetails{
		Package: core.Package{
			Ref:          ref,
			Name:         name,
			Version:      version,
			Architecture: properties["architecture"],
			Origin:       properties["repository"],
			Installed:    properties["state"] == "installed",
		},
		Description:   properties["short_desc"],
		Homepage:      properties["homepage"],
		License:       properties["license"],
		DownloadSize:  properties["filename-size"],
		InstalledSize: properties["installed_size"],
		InstallDate:   properties["install-date"],
		Maintainer:    properties["maintainer"],
		SourcePackage: properties["sourcepkg"],
		Provides:      lists["provides"],
		Requires:      lists["shlib-requires"],
		Exports:       lists["shlib-provides"],
	}, nil
}

func parseXBPSList(output []byte, installedOnly bool) ([]core.Package, error) {
	packages := make([]core.Package, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || line[0] == ' ' || line[0] == '\t' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("%w: package line %q", ErrInvalidBackendOutput, scanner.Text())
		}
		installed := fields[0] == "ii" || fields[0] == "[*]"
		validStatus := installed || (!installedOnly && fields[0] == "[-]")
		if !validStatus {
			return nil, fmt.Errorf("%w: package line %q", ErrInvalidBackendOutput, scanner.Text())
		}
		name, version, ok := splitXBPSPkgver(fields[1])
		if !ok {
			return nil, fmt.Errorf("%w: package version %q", ErrInvalidBackendOutput, fields[1])
		}
		packages = append(packages, core.Package{
			Ref:         core.PackageRef{BackendID: "xbps", ID: name},
			Name:        name,
			Description: strings.Join(fields[2:], " "),
			Version:     version,
			Installed:   installed,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse installed XBPS packages: %w", err)
	}
	return packages, nil
}

func splitXBPSPkgver(pkgver string) (string, string, bool) {
	separator := strings.LastIndexByte(pkgver, '-')
	if separator < 1 || separator == len(pkgver)-1 {
		return "", "", false
	}
	version := pkgver[separator+1:]
	revision := strings.LastIndexByte(version, '_')
	if revision < 1 || revision == len(version)-1 {
		return "", "", false
	}
	for _, digit := range version[revision+1:] {
		if digit < '0' || digit > '9' {
			return "", "", false
		}
	}
	return pkgver[:separator], version, true
}
