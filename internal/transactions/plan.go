package transactions

import (
	"errors"
	"time"

	"sysentra/internal/security"
)

type Change struct {
	Package        string `json:"package"`
	Action         string `json:"action"`
	FromVersion    string `json:"fromVersion"`
	ToVersion      string `json:"toVersion"`
	InstalledSize  int64  `json:"installedSize"`
	DownloadSize   int64  `json:"downloadSize"`
	ArtifactPath   string `json:"artifactPath,omitempty"`
	ArtifactSHA256 string `json:"artifactSha256,omitempty"`
}

type Plan struct {
	Request          security.PackageRequest `json:"request"`
	Changes          []Change                `json:"changes"`
	StateFingerprint string                  `json:"stateFingerprint"`
	ExpiresAt        time.Time               `json:"expiresAt"`
	Diagnostics      []string                `json:"diagnostics"`
	DownloadSize     int64                   `json:"downloadSize"`
	InstalledSize    int64                   `json:"installedSize"`
	AvailableSpace   int64                   `json:"availableSpace"`
}

func (p Plan) ValidateCommit(now time.Time, currentFingerprint string) error {
	if err := p.Request.Validate(); err != nil {
		return err
	}
	if p.StateFingerprint == "" || currentFingerprint != p.StateFingerprint {
		return errors.New("package state changed; create a new plan")
	}
	if !now.Before(p.ExpiresAt) {
		return errors.New("transaction plan expired; create a new plan")
	}
	return nil
}
