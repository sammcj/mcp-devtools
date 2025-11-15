package aceternityui

// ComponentInfo represents an Aceternity UI component
type ComponentInfo struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	InstallCommand string   `json:"installCommand"`
	Dependencies   []string `json:"dependencies"`
	Tags           []string `json:"tags"`
	IsPro          bool     `json:"isPro"`
	Documentation  string   `json:"documentation,omitempty"`
}

// CategoryInfo represents a component category
type CategoryInfo struct {
	Name        string   `json:"name"`
	Components  []string `json:"components"`
	Description string   `json:"description"`
}

const (
	AceternityDocsURL   = "https://ui.aceternity.com"
	AceternityGitHubURL = "https://github.com/aceternity/ui"
)
