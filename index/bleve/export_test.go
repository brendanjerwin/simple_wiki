package bleve

// Export private methods for testing
// This avoids the capitalization linting error by using a different naming pattern

var (
	// CalculateFragmentWindowForTest exports the private calculateFragmentWindow for testing
	CalculateFragmentWindowForTest = (*Index).calculateFragmentWindow
	// ExtractFragmentFromLocationsForTest exports the private extractFragmentFromLocations for testing
	ExtractFragmentFromLocationsForTest = (*Index).extractFragmentFromLocations
)
