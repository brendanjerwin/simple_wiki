package labels

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

		"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
)

const (
	// bitSize for parsing uints from hex strings.
	bitSize = 16
)

// Printer defines the interface for a label printer.
type Printer interface {
	Write([]byte) (int, error)
	Close() error
}

// PrintLabel prints a label using the specified template and identifier.
func PrintLabel(templateIdentifier string, identifier string, site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex) error {
	templateIdentifier, templateData, err := site.ReadMarkdown(templateIdentifier)
	if err != nil {
		return err
	}

	_, templateFrontmatter, err := site.ReadFrontMatter(templateIdentifier)
	if err != nil {
		return err	}

	config, err := configFromFrontmatter(templateFrontmatter)
	if err != nil {
		return err
	}

	var printer Printer
	switch config.ConnectivityMode {
	case USB:
		printer, err = GetUSBPrinter(config)
	case LP:
		printer, err = GetLPPrinter(config)
	default:
		return fmt.Errorf("unknown connectivity mode: %v", config.ConnectivityMode)
	}
	if err != nil {
		return err
	}
	defer func() { _ = printer.Close() }()

		_, pageFrontmatter, err := site.ReadFrontMatter(identifier)
	if err != nil {
		return err
	}

	zpl, err := templating.ExecuteTemplateForLabels(string(templateData), pageFrontmatter, site, query)
	if err != nil {
		return err
	}

	_, err = printer.Write(zpl)
	return err
}

func configFromFrontmatter(templateFrontmatter wikipage.FrontMatter) (PrinterConfig, error) {
	var err error

	printerValue, ok := templateFrontmatter["label_printer"].(map[string]any)
	if !ok {
		return PrinterConfig{}, errors.New("label_printer is not a map")
	}

	config := PrinterConfig{}
	modeValue, ok := printerValue["mode"].(string)
	if !ok {
		return PrinterConfig{}, errors.New("mode is not a string")
	}
	config.ConnectivityMode, err = ParseConnectivityMode(modeValue)
	if err != nil {
		return PrinterConfig{}, err
	}

	switch config.ConnectivityMode {
	case USB:
		vendorValue, ok := printerValue["vendor"].(string)
		if !ok {
			return PrinterConfig{}, errors.New("vendor is not a string")
		}

		vendorValue = strings.TrimPrefix(vendorValue, "0x")
		vendor, err := strconv.ParseUint(vendorValue, bitSize, bitSize)
		if err != nil {
			return PrinterConfig{}, fmt.Errorf("failed to parse vendor: %v", err)
		}

		productValue, ok := printerValue["product"].(string)
		if !ok {
			return PrinterConfig{}, errors.New("product is not a string")
		}

		productValue = strings.TrimPrefix(productValue, "0x")
		product, err := strconv.ParseUint(productValue, bitSize, bitSize)
		if err != nil {
			return PrinterConfig{}, fmt.Errorf("failed to parse product: %v", err)
		}

		config.USBVendor = uint16(vendor)
		config.USBProduct = uint16(product)

	case LP:
		lpPrinterName, ok := printerValue["name"].(string)
		if !ok {
			return PrinterConfig{}, errors.New("name is not a string")
		}
		config.LPPrinterName = lpPrinterName
	default:
		return PrinterConfig{}, fmt.Errorf("unknown connectivity mode: %v", config.ConnectivityMode)
	}
	return config, nil
}

// PrinterConfig holds the configuration for a printer.
type PrinterConfig struct {
	ConnectivityMode ConnectivityMode
	USBVendor        uint16
	USBProduct       uint16
	LPPrinterName    string
}

// ConnectivityMode represents the printer's connectivity mode.
type ConnectivityMode int

const (
	// Unset connectivity mode.
	Unset ConnectivityMode = iota
	// USB connectivity mode.
	USB
	// LP connectivity mode.
	LP
)

// ParseConnectivityMode parses a string into a ConnectivityMode.
func ParseConnectivityMode(mode string) (ConnectivityMode, error) {
	switch strings.ToLower(mode) {
	case "usb":
		return USB, nil
	case "lp":
		return LP, nil
	default:
		return 0, fmt.Errorf("invalid connectivity mode: %s", mode)
	}
}
