package caldav

// Test-only re-exports for unexported helpers. Keeping them in
// export_test.go (compiled only with `go test`) means the public API
// surface stays minimal.

// SanitizePathComponentForTest is the test-only re-export of
// sanitizePathComponent.
func SanitizePathComponentForTest(s string) (string, error) {
	return sanitizePathComponent(s)
}

// ValidateUIDForTest is the test-only re-export of validateUID.
func ValidateUIDForTest(uid string) error {
	return validateUID(uid)
}

// ParsePathForTest is the test-only re-export of parsePath.
//
//revive:disable-next-line:function-result-limit Mirrors parsePath's shape; bundling into a struct adds noise in tests.
func ParsePathForTest(reqURL string) (page, list, uid string, err error) {
	return parsePath(reqURL)
}
