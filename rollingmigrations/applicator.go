package rollingmigrations

import (
	"bytes"
	"fmt"
	"slices"
)

const (
	// Minimum content length to determine frontmatter type
	minContentLength = 3
)

type DefaultApplicator struct {
	migrations []FrontmatterMigration
}

func NewApplicator() *DefaultApplicator {
	applicator := &DefaultApplicator{
		migrations: make([]FrontmatterMigration, 0),
	}
	
	// Register default migrations - order matters for execution
	applicator.RegisterMigration(NewTOMLDotNotationMigration())
	applicator.RegisterMigration(NewTOMLTableSpacingMigration()) // Must be last for proper formatting
	
	return applicator
}

// NewEmptyApplicator creates an applicator without any default migrations (for testing)
func NewEmptyApplicator() *DefaultApplicator {
	return &DefaultApplicator{
		migrations: make([]FrontmatterMigration, 0),
	}
}

func (a *DefaultApplicator) RegisterMigration(migration FrontmatterMigration) {
	a.migrations = append(a.migrations, migration)
}

func (a *DefaultApplicator) ApplyMigrations(content []byte) ([]byte, error) {
	// Detect frontmatter type once
	fmType := detectFrontmatterType(content)
	if fmType == FrontmatterUnknown {
		return content, nil // No frontmatter or unrecognized format
	}

	current := content
	for _, migration := range a.migrations {
		// Check if migration supports this frontmatter type
		if !slices.Contains(migration.SupportedTypes(), fmType) {
			continue
		}

		if migration.AppliesTo(current) {
			migrated, err := migration.Apply(current)
			if err != nil {
				// On error, return original content; the caller is responsible for logging
				return content, fmt.Errorf("migration failed: %w", err)
			}
			current = migrated
		}
	}
	return current, nil
}

func detectFrontmatterType(content []byte) FrontmatterType {
	if len(content) < minContentLength {
		return FrontmatterUnknown
	}

	if bytes.HasPrefix(content, []byte("---")) {
		return FrontmatterYAML
	}
	if bytes.HasPrefix(content, []byte("+++")) {
		return FrontmatterTOML
	}
	if bytes.HasPrefix(content, []byte("{")) {
		return FrontmatterJSON
	}

	return FrontmatterUnknown
}

