package xbpsconfig

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Rule struct {
	Kind    string
	Value   string
	File    string
	Origin  string
	Mutable bool
}

type Inventory struct {
	localDir  string
	vendorDir string
}

func NewInventory() *Inventory {
	return &Inventory{localDir: "/etc/xbps.d", vendorDir: "/usr/share/xbps.d"}
}

func (i *Inventory) List() ([]Rule, error) {
	files := map[string]string{}
	for _, dir := range []string{i.vendorDir, i.localDir} {
		paths, err := filepath.Glob(filepath.Join(dir, "*.conf"))
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			files[filepath.Base(path)] = path
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	var rules []Rule
	for _, name := range names {
		path := files[name]
		contents, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(contents), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "ignorepkg=") {
				rules = append(rules, Rule{Kind: "Ignored package", Value: strings.TrimPrefix(line, "ignorepkg="), File: name, Origin: path, Mutable: true})
			} else if strings.HasPrefix(line, "virtualpkg=") {
				rules = append(rules, Rule{Kind: "Virtual package", Value: strings.TrimPrefix(line, "virtualpkg="), File: name, Origin: path, Mutable: true})
			}
		}
	}
	return rules, nil
}
