package backends

import (
	"context"

	"sysentra/internal/core"
)

type Backend interface {
	ID() string
	Capabilities(context.Context) (core.Capabilities, error)
	Search(context.Context, string) ([]core.Package, error)
	ListInstalled(context.Context) ([]core.Package, error)
	Get(context.Context, core.PackageRef) (core.PackageDetails, error)
}
