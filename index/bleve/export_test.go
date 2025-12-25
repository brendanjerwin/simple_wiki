package bleve

// Export private methods for testing
// This pattern is acceptable because test-only exports in _test.go files are a common Go practice

var (
	// CalculateFragmentWindowForTest provides test access to the private calculateFragmentWindow method
	CalculateFragmentWindowForTest = (*Index).calculateFragmentWindow
	// ExtractFragmentFromLocationsForTest provides test access to the private extractFragmentFromLocations method
	ExtractFragmentFromLocationsForTest = (*Index).extractFragmentFromLocations
)
