package core

// PackageRef preserves backend-native identity so similarly named packages are not merged.
type PackageRef struct {
	BackendID string
	ID        string
}

type Package struct {
	Ref          PackageRef
	Name         string
	Description  string
	Version      string
	Architecture string
	Origin       string
	Scope        string
	NewVersion   string
	Installed    bool
	Held         bool
	Orphaned     bool
	RepoLocked   bool
}

type PackageDetails struct {
	Package
	Description         string
	Homepage            string
	License             string
	IconDataURL         string
	DownloadSize        string
	InstalledSize       string
	InstallDate         string
	Maintainer          string
	SourcePackage       string
	Provides            []string
	Requires            []string
	Exports             []string
	Dependencies        []string
	ReverseDependencies []string
	Files               []string
	CachedVersions      []string
}

type Capabilities struct {
	Search        bool
	ListInstalled bool
	Details       bool
}
