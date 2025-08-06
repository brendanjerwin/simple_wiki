package lazy

type FrontmatterType int

const (
	FrontmatterUnknown FrontmatterType = iota
	FrontmatterYAML    // ---
	FrontmatterTOML    // +++
	FrontmatterJSON    // { }
)

type FrontmatterMigration interface {
	SupportedTypes() []FrontmatterType
	AppliesTo(content []byte) bool
	Apply(content []byte) ([]byte, error)
}

type FrontmatterMigrationRegistry interface {
	RegisterMigration(migration FrontmatterMigration)
}

type FrontmatterMigrationApplicator interface {
	ApplyMigrations(content []byte) ([]byte, error)
}