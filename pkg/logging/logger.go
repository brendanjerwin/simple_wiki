package logging

// Logger defines a minimal logging interface used throughout the application.
// This interface follows the Interface Segregation Principle and enables
// easier testing with mock loggers while decoupling components from specific
// logger implementations.
type Logger interface {
	// Info logs an informational message with printf-style formatting.
	Info(format string, args ...any)

	// Error logs an error message with printf-style formatting.
	Error(format string, args ...any)

	// Warn logs a warning message with printf-style formatting.
	Warn(format string, args ...any)
}
