// Package labels provides functionality for printing labels.
package labels

import "os/exec"

// GetLPPrinter returns a Printer for an LP printer.
func GetLPPrinter(config PrinterConfig) (Printer, error) {
	return &LPPrinter{config.LPPrinterName}, nil
}

// LPPrinter is a printer that uses the `lp` command.
type LPPrinter struct {
	Name string
}

// Write writes data to the LP printer.
func (p *LPPrinter) Write(data []byte) (int, error) {
	cmd := exec.Command("lp", "-d", p.Name)

	// Get a pipe to the command's standard input
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}

	// Write some bytes to the command's standard input
	_, err = stdin.Write(data)
	if err != nil {
		return 0, err
	}

	// Close the pipe to indicate that you're done writing
	err = stdin.Close()
	if err != nil {
		return 0, err
	}

	// Run the command
	err = cmd.Run()
	if err != nil {
		return 0, err
	}
	return 0, nil
}

// Close closes the LP printer.
func (LPPrinter) Close() error {
	return nil
}
