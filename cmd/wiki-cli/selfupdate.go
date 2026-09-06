// In-binary self-update: when this binary's embedded commit differs from the
// running wiki server's, download the matching binary from the server's /cli/
// endpoint, atomically replace the running executable's file, and re-exec.
// This removes the need for out-of-band bootstrapper scripts beyond the very
// first download.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const selfUpdateTimeout = 120 * time.Second

// cliBinaryName returns the platform-specific binary name served by the wiki.
// Windows binaries carry an .exe suffix (see build-all.sh).
func cliBinaryName() string {
	name := fmt.Sprintf("wiki-cli-%s-%s", strings.ToLower(runtime.GOOS), runtime.GOARCH)
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

// selfUpdateError reports that a new binary was downloaded and the caller
// should re-exec. It is never returned to the user; os.Exit paths consume it.
type selfUpdateError struct {
	newPath string
}

func (e *selfUpdateError) Error() string {
	return fmt.Sprintf("wiki-cli updated itself at %s; re-exec required", e.newPath)
}

// downloadMatchingBinary fetches the /cli/ binary matching the running wiki
// server's platform and writes it to a sibling temp file of exePath. The
// returned path is executable and atomic-rename ready. It uses a plain GET
// (no If-Modified-Since) because the caller has already determined a mismatch
// exists; the server's Last-Modified is the server's start time, not the
// binary's build time, so conditional semantics are meaningless here.
func downloadMatchingBinary(wikiURL, exePath string) (string, error) {
	binaryName := cliBinaryName()
	downloadURL := strings.TrimRight(wikiURL, "/") + "/cli/" + binaryName

	ctx, cancel := context.WithTimeout(context.Background(), selfUpdateTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return "", fmt.Errorf("self-update: could not build download request for %s: %w", downloadURL, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("self-update: cannot reach %s: %w", downloadURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("self-update: %s returned HTTP %d", downloadURL, resp.StatusCode)
	}

	tmpPath := filepath.Join(filepath.Dir(exePath), "."+binaryName+".update")
	tmpFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", fmt.Errorf("self-update: could not create %s: %w", tmpPath, err)
	}

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("self-update: download truncated: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("self-update: could not finalize %s: %w", tmpPath, err)
	}

	return tmpPath, nil
}

// swapBinary atomically replaces exePath's contents with newPath via rename.
// Same-filesystem rename is atomic; the running process keeps executing the
// old inode until it exits.
func swapBinary(exePath, newPath string) error {
	if err := os.Rename(newPath, exePath); err != nil {
		_ = os.Remove(newPath)
		return fmt.Errorf("self-update: could not replace %s: %w", exePath, err)
	}
	return nil
}

// resolveBinaryPath returns the real path of the running executable.
func resolveBinaryPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("self-update: could not resolve running executable: %w", err)
	}
	return filepath.EvalSymlinks(exePath)
}

// selfUpdateForPath downloads the matching binary and swaps it in over
// exePath, reporting success via *selfUpdateError. Injectable for tests.
func selfUpdateForPath(wikiURL, exePath string) error {
	newPath, err := downloadMatchingBinary(wikiURL, exePath)
	if err != nil {
		return err
	}
	if err := swapBinary(exePath, newPath); err != nil {
		return err
	}
	return &selfUpdateError{newPath: exePath}
}

// selfUpdate downloads the matching binary, swaps it over the running
// executable, and reports success. The caller is responsible for re-exec.
func selfUpdate(wikiURL string) error {
	exePath, err := resolveBinaryPath()
	if err != nil {
		return err
	}
	return selfUpdateForPath(wikiURL, exePath)
}

// reExecSelf replaces the current process with the freshly-swapped binary,
// preserving argv. Never returns on success; on failure it returns so the
// caller can fall back to exiting with the original mismatch error.
func reExecSelf() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return err
	}
	return syscallExec(exePath, os.Args, os.Environ())
}

// backgroundSelfUpdateIfStale checks the wiki server's commit and, when it
// differs from this binary's, silently downloads and swaps the matching
// binary over the on-disk executable — without re-exec (mcp sessions cannot
// restart mid-stdio). Best-effort: all failures are swallowed; the next
// non-mcp invocation's startup check self-heals via re-exec.
func backgroundSelfUpdateIfStale(wikiURL string) {
	defer func() {
		// Defensive: this runs concurrently with stdio serving; a panic here
		// would take down the MCP session. Swallow everything.
		_ = recover()
	}()
	if commit == "dev" {
		return
	}
	if !serverVersionMismatch(wikiURL) {
		return
	}
	if err := selfUpdate(wikiURL); err != nil {
		fmt.Fprintf(os.Stderr, "wiki-cli background refresh skipped: %v\n", err)
	}
}
