package shadcnui

// ComponentProp defines the structure for a component's property or variant.
type ComponentProp struct {
	Type        string `json:"type"` // e.g., "variant", "string", "boolean"
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Example     string `json:"example,omitempty"` // For variants, this could be code
}

// ComponentExample defines a usage example for a component.
type ComponentExample struct {
	Title       string `json:"title"`
	Code        string `json:"code"`
	Description string `json:"description,omitempty"`
}

// ComponentInfo holds all details for a shadcn ui component.
type ComponentInfo struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	URL          string                   `json:"url"`                    // Link to the docs page
	SourceURL    string                   `json:"sourceUrl,omitempty"`    // Link to GitHub source
	APIReference string                   `json:"apiReference,omitempty"` // If available
	Installation string                   `json:"installation,omitempty"` // npx command
	Usage        string                   `json:"usage,omitempty"`        // General usage code block
	Props        map[string]ComponentProp `json:"props,omitempty"`        // Component props/variants
	Examples     []ComponentExample       `json:"examples,omitempty"`
}
