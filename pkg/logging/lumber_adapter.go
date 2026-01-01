package logging

import "github.com/jcelliott/lumber"

// LumberAdapter adapts lumber.ConsoleLogger to implement our Logger interface.
// This allows lumber.ConsoleLogger to be used wherever our Logger interface is expected.
type LumberAdapter struct {
	*lumber.ConsoleLogger
}

// NewLumberAdapter creates a new adapter for a lumber console logger.
func NewLumberAdapter(logger *lumber.ConsoleLogger) Logger {
	return &LumberAdapter{ConsoleLogger: logger}
}

// Compile-time check that lumber.ConsoleLogger implements Logger interface.
var _ Logger = (*lumber.ConsoleLogger)(nil)
