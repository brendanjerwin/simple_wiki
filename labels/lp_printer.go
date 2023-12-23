package labels

import "os/exec"

func GetLPPrinter(config PrinterConfig) (Printer, error) {
	return &LPPrinter{config.LPPrinterName}, nil
}

type LPPrinter struct {
	Name string
}

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

func (p *LPPrinter) Close() error {
	return nil
}
