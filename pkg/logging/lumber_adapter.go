package logging

import "github.com/jcelliott/lumber"

// LumberAdapter adapts lumber.Logger to implement our Logger interface.
// This allows lumber.Logger to be used wherever our Logger interface is expected.
type LumberAdapter struct {
	*lumber.ConsoleLogger
}

// NewLumberAdapter creates a new adapter for a lumber logger.
func NewLumberAdapter(logger *lumber.ConsoleLogger) Logger {
	return &LumberAdapter{ConsoleLogger: logger}
}

// Compile-time check that lumber.Logger implements Logger interface.
var _ Logger = (*lumber.ConsoleLogger)(nil)
