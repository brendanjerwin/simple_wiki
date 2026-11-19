package main

import (
	"syscall"
)

// syscallExec replaces the current process image. Split out so tests can
// stub process replacement.
func syscallExec(path string, argv, envv []string) error {
	return syscall.Exec(path, argv, envv)
}
