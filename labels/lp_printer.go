// Package labels provides functionality for printing labels.
package labels

import (
	"bytes"
	"fmt"
	"os/exec"
)

// GetLPPrinter returns a Printer for an LP printer.
func GetLPPrinter(config PrinterConfig) (Printer, error) {
	lpPath, err := exec.LookPath("lp")
	if err != nil {
		return nil, fmt.Errorf("lp not found: %w", err)
	}
	return &LPPrinter{name: config.LPPrinterName, lpPath: lpPath}, nil
}

// LPPrinter is a printer that uses the `lp` command.
type LPPrinter struct {
	name   string
	lpPath string
}

// Write writes data to the LP printer.
func (p *LPPrinter) Write(data []byte) (int, error) {
	cmd := exec.Command(p.lpPath, "-d", p.name)
	cmd.Stdin = bytes.NewReader(data)

	if err := cmd.Run(); err != nil {
		return 0, err
	}
	return len(data), nil
}

// Close closes the LP printer.
func (LPPrinter) Close() error {
	return nil
}
