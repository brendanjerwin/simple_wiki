// Package common provides common utilities for the simple_wiki project.
package common

import (
	"regexp"
	"strings"

	"github.com/stoewer/go-strcase"
)

var uuidRegex = regexp.MustCompile("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}")

// MungeIdentifier converts an identifier to a consistent format.
// If the identifier contains a UUID, it returns the lowercase identifier.
// Otherwise, it returns the snake case identifier.
func MungeIdentifier(identifier string) string {
	// If the identifier contains a UUID, return the lowercase identifier
	if uuidRegex.MatchString(identifier) {
		return strings.ToLower(identifier)
	}

	// Otherwise, return the snake case identifier
	return strings.ToLower(strcase.SnakeCase(identifier))
}
