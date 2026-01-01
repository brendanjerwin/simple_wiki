package logging_test

import (
	"testing"

	"github.com/brendanjerwin/simple_wiki/pkg/logging"
	"github.com/jcelliott/lumber"
)

// TestLumberImplementsLogger verifies that lumber.ConsoleLogger implements our Logger interface.
func TestLumberImplementsLogger(t *testing.T) {
	logger := lumber.NewConsoleLogger(lumber.WARN)

	// This should compile - lumber.Logger implements our interface
	var _ logging.Logger = logger
}

// TestLumberAdapter verifies that the LumberAdapter works correctly.
func TestLumberAdapter(t *testing.T) {
	lumberLogger := lumber.NewConsoleLogger(lumber.WARN)
	adapter := logging.NewLumberAdapter(lumberLogger)

	// This should compile - adapter implements our interface
	var _ logging.Logger = adapter

	// Basic functionality test - these should not panic
	adapter.Info("test info: %s", "info")
	adapter.Error("test error: %s", "error")
	adapter.Warn("test warn: %s", "warn")
}
