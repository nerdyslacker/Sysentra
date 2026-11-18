package flatpakplan

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"sysentra/internal/runner"
)

type Change struct {
	Ref            string
	Action         string
	CurrentVersion string
	NewVersion     string
	DownloadSize   int64
	InstalledSize  int64
	RequiresAuth   bool
}

type Planner struct{ runner runner.Runner }

func New(commandRunner runner.Runner) *Planner { return &Planner{runner: commandRunner} }

func (p *Planner) Preview(ctx context.Context, packageIDs []string, operation, scope, remote string) ([]Change, error) {
	refs := make([]string, len(packageIDs))
	for index, id := range packageIDs {
		args := []string{"--" + scope}
		if operation == "install" {
			args = append(args, "remote-info", "--show-ref", remote, id)
		} else {
			args = append(args, "info", "--show-ref", id)
		}
		result, err := p.runner.Run(ctx, "flatpak", args...)
		if err != nil {
			return nil, fmt.Errorf("resolve Flatpak ref %s: %w", id, err)
		}
		refs[index] = strings.TrimSpace(string(result.Stdout))
		if refs[index] == "" {
			return nil, errors.New("Flatpak returned an empty ref")
		}
	}
	changes, err := resolve(scope, remote, operation, refs)
	if err != nil {
		return nil, err
	}
	for index := range changes {
		change := &changes[index]
		if operation == "update" || operation == "remove" {
			if result, infoErr := p.runner.Run(ctx, "flatpak", "--"+scope, "info", "--show-version", change.Ref); infoErr == nil {
				change.CurrentVersion = strings.TrimSpace(string(result.Stdout))
			}
		}
		if operation == "update" || operation == "install" {
			if result, infoErr := p.runner.Run(ctx, "flatpak", "--"+scope, "remote-info", "--show-version", remote, change.Ref); infoErr == nil {
				change.NewVersion = strings.TrimSpace(string(result.Stdout))
			}
		}
		if operation == "remove" {
			result, infoErr := p.runner.Run(ctx, "flatpak", "--"+scope, "info", "--show-size", change.Ref)
			if infoErr == nil {
				if size, parseErr := strconv.ParseInt(strings.TrimSpace(string(result.Stdout)), 10, 64); parseErr == nil {
					change.InstalledSize = size
				}
			}
		}
	}
	return changes, nil
}
