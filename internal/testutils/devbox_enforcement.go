package testutils

import (
	"os"
)

// EnforceDevboxInCI checks if we're running in GitHub Actions and errors if not run via devbox
func EnforceDevboxInCI() {
	// Only enforce in GitHub Actions environment
	if os.Getenv("GITHUB_ACTIONS") != "true" {
		return
	}

	// Check if we're running through devbox
	if os.Getenv("DEVBOX_SHELL_ENABLED") != "1" {
		panic("ERROR: Tests must be run using 'devbox run go:test' in GitHub Actions environment.\nPlease use: devbox run go:test")
	}
}