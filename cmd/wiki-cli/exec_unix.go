package main

import (
	"syscall"
)

// syscallExec replaces the current process image. Split out so tests can
// stub process replacement.
func syscallExec(path string, argv []string, envv []string) error {
	return syscall.Exec(path, argv, envv)
}
