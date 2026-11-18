package fonts

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"sysentra/internal/runner"
)

type Inventory struct{ runner runner.Runner }

func NewInventory(commandRunner runner.Runner) *Inventory {
	return &Inventory{runner: commandRunner}
}

func (i *Inventory) List() ([]string, error) {
	unique := make(map[string]bool)
	for _, generic := range []string{"sans-serif", "serif", "monospace"} {
		result, err := i.runner.Run(context.Background(), "/usr/bin/fc-match", "-s", generic, "family")
		if err != nil {
			return nil, fmt.Errorf("list %s system fonts: %w", generic, err)
		}
		for _, line := range strings.Split(string(result.Stdout), "\n") {
			for _, family := range strings.Split(line, ",") {
				family = strings.Trim(strings.TrimSpace(family), `"`)
				if family != "" && len(family) <= 128 {
					unique[family] = true
				}
			}
		}
	}
	fonts := make([]string, 0, len(unique))
	for family := range unique {
		fonts = append(fonts, family)
	}
	sort.Strings(fonts)
	return fonts, nil
}
