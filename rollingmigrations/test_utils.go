package rollingmigrations

// MockMigration is a test utility for mocking FrontmatterMigration behavior
type MockMigration struct {
	SupportedTypesResult []FrontmatterType
	AppliesToResult      bool
	ApplyResult          []byte
	ApplyError           error
}

func (m *MockMigration) SupportedTypes() []FrontmatterType {
	return m.SupportedTypesResult
}

func (m *MockMigration) AppliesTo(_ []byte) bool {
	return m.AppliesToResult
}

func (m *MockMigration) Apply(content []byte) ([]byte, error) {
	if m.ApplyError != nil {
		return content, m.ApplyError
	}
	if m.ApplyResult != nil {
		return m.ApplyResult, nil
	}
	return content, nil
}

// NewMockMigration creates a new MockMigration with default values
func NewMockMigration() *MockMigration {
	return &MockMigration{
		SupportedTypesResult: []FrontmatterType{FrontmatterTOML},
		AppliesToResult:      true,
	}
}