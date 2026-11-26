package repositories

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sysentra/internal/runner"
)

type Repository struct {
	Name     string
	URL      string
	Priority int
	Enabled  bool
	Mutable  bool
}

type Inventory struct {
	runner               runner.Runner
	configDir, vendorDir string
}

func NewInventory(commandRunner runner.Runner) *Inventory {
	return &Inventory{runner: commandRunner, configDir: "/etc/xbps.d", vendorDir: "/usr/share/xbps.d"}
}

func (i *Inventory) List(ctx context.Context) ([]Repository, error) {
	result, err := i.runner.Run(ctx, "xbps-query", "--list-repos")
	if err != nil {
		return nil, fmt.Errorf("list XBPS repositories: %w", err)
	}
	return mergeConfiguredRepositories(parseRepositories(result.Stdout), i.configDir, i.vendorDir), nil
}

func parseRepositories(output []byte) []Repository {
	seen := map[string]bool{}
	var result []Repository
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		url := fields[1]
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		result = append(result, Repository{URL: url, Priority: len(result) + 1, Enabled: true})
	}
	return result
}

func mergeConfiguredRepositories(repositories []Repository, configDir, vendorDir string) []Repository {
	byURL := make(map[string]int, len(repositories))
	for index, repository := range repositories {
		byURL[repository.URL] = index
	}
	files := map[string]string{}
	for _, dir := range []string{vendorDir, configDir} {
		paths, _ := filepath.Glob(filepath.Join(dir, "*.conf"))
		for _, path := range paths {
			files[filepath.Base(path)] = path
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := files[name]
		contents, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(line)
			enabled := strings.HasPrefix(line, "repository=")
			url := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "#repository="), "repository="))
			if url == line || url == "" {
				continue
			}
			if index, found := byURL[url]; found {
				repositories[index].Name = name
				repositories[index].Mutable = true
				continue
			}
			byURL[url] = len(repositories)
			repositories = append(repositories, Repository{Name: name, URL: url, Priority: len(repositories) + 1, Enabled: enabled, Mutable: true})
		}
	}
	return repositories
}
