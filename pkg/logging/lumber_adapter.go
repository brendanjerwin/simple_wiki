package logging

import "github.com/jcelliott/lumber"

// Compile-time check that lumber.ConsoleLogger implements Logger interface.
var _ Logger = (*lumber.ConsoleLogger)(nil)
