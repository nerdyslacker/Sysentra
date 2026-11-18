package app

import (
	"context"

	"sysentra/internal/repositories"
)

type RepositoryKeyView struct {
	Signer      string `json:"signer"`
	Type        string `json:"type"`
	Bits        string `json:"bits"`
	Fingerprint string `json:"fingerprint"`
}

type RepositoryTrustService struct{ inspector *repositories.TrustInspector }

func NewRepositoryTrustService(inspector *repositories.TrustInspector) *RepositoryTrustService {
	return &RepositoryTrustService{inspector: inspector}
}

func (s *RepositoryTrustService) InspectRepositoryKey(repositoryURL string) (RepositoryKeyView, error) {
	identity, err := s.inspector.Inspect(context.Background(), repositoryURL)
	if err != nil {
		return RepositoryKeyView{}, err
	}
	return RepositoryKeyView{Signer: identity.Signer, Type: identity.Type, Bits: identity.Bits, Fingerprint: identity.Fingerprint}, nil
}
