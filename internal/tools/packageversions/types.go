package packageversions

// PackageVersion represents version information for a package
type PackageVersion struct {
	Name             string    `json:"name"`
	CurrentVersion   *string   `json:"currentVersion,omitempty"`
	LatestVersion    string    `json:"latestVersion"`
	Registry         string    `json:"registry"`
	Skipped          bool      `json:"skipped,omitempty"`
	SkipReason       string    `json:"skipReason,omitempty"`
	// Detailed information (populated when includeDetails is true)
	Description      *string   `json:"description,omitempty"`
	Homepage         *string   `json:"homepage,omitempty"`
	Repository       *string   `json:"repository,omitempty"`
	Documentation    *string   `json:"documentation,omitempty"`
	License          *string   `json:"license,omitempty"`
	Downloads        *int64    `json:"downloads,omitempty"`
	RecentDownloads  *int64    `json:"recentDownloads,omitempty"`
	CreatedAt        *string   `json:"createdAt,omitempty"`
	UpdatedAt        *string   `json:"updatedAt,omitempty"`
	PublishedAt      *string   `json:"publishedAt,omitempty"`
	NumVersions      *int      `json:"numVersions,omitempty"`
	Keywords         []string  `json:"keywords,omitempty"`
	Categories       []string  `json:"categories,omitempty"`
	Publisher        *string   `json:"publisher,omitempty"`
	RustVersion      *string   `json:"rustVersion,omitempty"`
	Edition          *string   `json:"edition,omitempty"`
	CrateSize        *int64    `json:"crateSize,omitempty"`
}

// VersionConstraint represents constraints for package version updates
type VersionConstraint struct {
	MajorVersion   *int `json:"majorVersion,omitempty"`
	ExcludePackage bool `json:"excludePackage,omitempty"`
}

// VersionConstraints maps package names to their constraints
type VersionConstraints map[string]VersionConstraint

// NpmDependencies represents dependencies in a package.json file
type NpmDependencies map[string]string

// PyProjectDependencies represents dependencies in a pyproject.toml file
type PyProjectDependencies struct {
	Dependencies         map[string]string            `json:"dependencies,omitempty"`
	OptionalDependencies map[string]map[string]string `json:"optional-dependencies,omitempty"`
	DevDependencies      map[string]string            `json:"dev-dependencies,omitempty"`
}

// MavenDependency represents a dependency in a Maven pom.xml file
type MavenDependency struct {
	GroupID    string `json:"groupId"`
	ArtifactID string `json:"artifactId"`
	Version    string `json:"version,omitempty"`
	Scope      string `json:"scope,omitempty"`
}

// GradleDependency represents a dependency in a Gradle build.gradle file
type GradleDependency struct {
	Configuration string `json:"configuration"`
	Group         string `json:"group"`
	Name          string `json:"name"`
	Version       string `json:"version,omitempty"`
}

// GoModule represents a Go module in a go.mod file
type GoModule struct {
	Module  string      `json:"module"`
	Require []GoRequire `json:"require,omitempty"`
	Replace []GoReplace `json:"replace,omitempty"`
}

// GoRequire represents a required dependency in a go.mod file
type GoRequire struct {
	Path    string `json:"path"`
	Version string `json:"version,omitempty"`
}

// GoReplace represents a replacement in a go.mod file
type GoReplace struct {
	Old     string `json:"old"`
	New     string `json:"new"`
	Version string `json:"version,omitempty"`
}

// SwiftDependency represents a dependency in a Swift Package.swift file
type SwiftDependency struct {
	URL         string `json:"url"`
	Version     string `json:"version,omitempty"`
	Requirement string `json:"requirement,omitempty"`
}

// BedrockModel represents an AWS Bedrock model
type BedrockModel struct {
	Provider           string   `json:"provider"`
	ModelName          string   `json:"modelName"`
	ModelID            string   `json:"modelId"`
	RegionsSupported   []string `json:"regionsSupported"`
	InputModalities    []string `json:"inputModalities"`
	OutputModalities   []string `json:"outputModalities"`
	StreamingSupported bool     `json:"streamingSupported"`
}

// BedrockModelSearchResult represents search results for AWS Bedrock models
type BedrockModelSearchResult struct {
	Models     []BedrockModel `json:"models"`
	TotalCount int            `json:"totalCount"`
}

// DockerImageVersion represents version information for a Docker image
type DockerImageVersion struct {
	Name     string  `json:"name"`
	Tag      string  `json:"tag"`
	Registry string  `json:"registry"`
	Digest   *string `json:"digest,omitempty"`
	Created  *string `json:"created,omitempty"`
	Size     *string `json:"size,omitempty"`
}

// DockerImageQuery represents a query for Docker image tags
type DockerImageQuery struct {
	Image          string   `json:"image"`
	Registry       string   `json:"registry,omitempty"`
	CustomRegistry string   `json:"customRegistry,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	FilterTags     []string `json:"filterTags,omitempty"`
	IncludeDigest  bool     `json:"includeDigest,omitempty"`
}

// GitHubAction represents a GitHub Action
type GitHubAction struct {
	Owner          string  `json:"owner"`
	Repo           string  `json:"repo"`
	CurrentVersion *string `json:"currentVersion,omitempty"`
}

// GitHubActionVersion represents version information for a GitHub Action
type GitHubActionVersion struct {
	Owner          string  `json:"owner"`
	Repo           string  `json:"repo"`
	CurrentVersion *string `json:"currentVersion,omitempty"`
	LatestVersion  string  `json:"latestVersion"`
	PublishedAt    *string `json:"publishedAt,omitempty"`
	URL            *string `json:"url,omitempty"`
}

// RustCrate represents a Rust crate dependency
type RustCrate struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Source  string `json:"source,omitempty"` // e.g., "registry+https://github.com/rust-lang/crates.io-index"
}

// CargoToml represents dependencies in a Cargo.toml file
type CargoToml struct {
	Dependencies      map[string]interface{} `json:"dependencies,omitempty"`
	DevDependencies   map[string]interface{} `json:"dev-dependencies,omitempty"`
	BuildDependencies map[string]interface{} `json:"build-dependencies,omitempty"`
}
