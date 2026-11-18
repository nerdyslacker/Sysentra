package app

import (
	"context"

	"sysentra/internal/flatpakremote"
)

type FlatpakRemoteView struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
}

type FlatpakRemoteService struct{ manager *flatpakremote.Manager }

func NewFlatpakRemoteService(manager *flatpakremote.Manager) *FlatpakRemoteService {
	return &FlatpakRemoteService{manager: manager}
}

func (s *FlatpakRemoteService) ListFlatpakRemotes() ([]FlatpakRemoteView, error) {
	remotes, err := s.manager.List(context.Background())
	if err != nil {
		return nil, err
	}
	views := make([]FlatpakRemoteView, len(remotes))
	for index, remote := range remotes {
		views[index] = FlatpakRemoteView{Name: remote.Name, Title: remote.Title, URL: remote.URL, Enabled: remote.Enabled()}
	}
	return views, nil
}

func (s *FlatpakRemoteService) ChangeFlatpakRemote(operation, name, remoteURL string) ([]FlatpakRemoteView, error) {
	if err := s.manager.Change(context.Background(), operation, name, remoteURL); err != nil {
		return nil, err
	}
	return s.ListFlatpakRemotes()
}
