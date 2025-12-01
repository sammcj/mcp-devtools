package pricing

// FilterCriteria defines filter criteria for pricing queries using AWS Pricing API
type FilterCriteria struct {
	Field string `json:"field"`          // e.g., "instanceType", "location", "operatingSystem"
	Value string `json:"value"`          // e.g., "t2.micro", "US East (N. Virginia)"
	Type  string `json:"type,omitempty"` // TERM_MATCH (default) or other comparison types
}
