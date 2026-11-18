package backends

import (
	"context"
	"errors"
	"strings"

	"sysentra/internal/core"
)

var ErrPackageNotFound = errors.New("package not found")

type Mock struct {
	Packages []core.PackageDetails
}

func (Mock) ID() string { return "mock" }

func (Mock) Capabilities(context.Context) (core.Capabilities, error) {
	return core.Capabilities{Search: true, ListInstalled: true, Details: true}, nil
}

func (m Mock) Search(ctx context.Context, query string) ([]core.Package, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	result := make([]core.Package, 0)
	for _, details := range m.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if query == "" || strings.Contains(strings.ToLower(details.Name), query) || strings.Contains(strings.ToLower(details.Description), query) {
			result = append(result, details.Package)
		}
	}
	return result, nil
}

func (m Mock) ListInstalled(ctx context.Context) ([]core.Package, error) {
	result := make([]core.Package, 0)
	for _, details := range m.Packages {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if details.Installed {
			result = append(result, details.Package)
		}
	}
	return result, nil
}

func (m Mock) Get(ctx context.Context, ref core.PackageRef) (core.PackageDetails, error) {
	if err := ctx.Err(); err != nil {
		return core.PackageDetails{}, err
	}
	for _, details := range m.Packages {
		if details.Ref == ref {
			return details, nil
		}
	}
	return core.PackageDetails{}, ErrPackageNotFound
}
