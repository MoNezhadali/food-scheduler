package nutrition

// NutritionData holds the result of a nutrition lookup,
// normalised to per-1-base-unit of the ingredient (e.g. per gram).
type NutritionData struct {
	CaloriesPerBase float64
	ProteinPerBase  float64
	CarbsPerBase    float64
	FatPerBase      float64
	Source          string // e.g. "usda:747447"
}

// SearchResult is one candidate returned by a nutrition provider search.
type SearchResult struct {
	ProviderID  string
	Description string
}

// Provider is the port for external nutrition data sources.
// Implementations live in adapters/secondary/nutrition/.
type Provider interface {
	// Search returns candidate foods matching the ingredient name.
	Search(query string) ([]SearchResult, error)
	// GetNutrition fetches and normalises nutrition for a provider food ID.
	// baseUnit is the ingredient's base unit (e.g. "grams") so the adapter
	// can divide USDA's per-100g values correctly.
	GetNutrition(providerID string, baseUnit string) (NutritionData, error)
}
