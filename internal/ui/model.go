package ui

import (
	"context"

	"sysentra/internal/backends"
	"sysentra/internal/core"
)

// Model keeps toolkit concerns out of backend operations.
type Model struct {
	backend backends.Backend
}

func NewModel(backend backends.Backend) Model {
	return Model{backend: backend}
}

func (m Model) Search(ctx context.Context, query string) ([]core.Package, error) {
	return m.backend.Search(ctx, query)
}

func (m Model) Installed(ctx context.Context) ([]core.Package, error) {
	return m.backend.ListInstalled(ctx)
}
