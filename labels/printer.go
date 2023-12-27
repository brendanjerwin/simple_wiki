package labels

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/templating"
)

type Printer interface {
	Write([]byte) (int, error)
	Close() error
}

func PrintLabel(template_identifier string, identifer string, site common.IReadPages, query frontmatter.IQueryFrontmatterIndex) error {
	template_identifier, template_data, err := site.ReadMarkdown(template_identifier)
	if err != nil {
		return err
	}

	template_identifier, template_frontmatter, err := site.ReadFrontMatter(template_identifier)
	if err != nil {
		return err
	}

	config, err := configFromFrontmatter(template_frontmatter)
	if err != nil {
		return err
	}

	var printer Printer
	switch config.ConnectivityMode {
	case USB:
		printer, err = GetUSBPrinter(config)
	case LP:
		printer, err = GetLPPrinter(config)
	}
	if err != nil {
		return err
	}
	defer printer.Close()

	identifer, frontmatter, err := site.ReadFrontMatter(identifer)
	if err != nil {
		return err
	}

	zpl, err := templating.ExecuteTemplate(string(template_data), frontmatter, site, query)
	if err != nil {
		return err
	}

	_, err = printer.Write(zpl)
	return err
}

func configFromFrontmatter(template_frontmatter common.FrontMatter) (PrinterConfig, error) {
	var err error

	printerValue, ok := template_frontmatter["label_printer"].(map[string]interface{})
	if !ok {
		return PrinterConfig{}, fmt.Errorf("label_printer is not a map")
	}

	config := PrinterConfig{}
	modeValue, ok := printerValue["mode"].(string)
	if !ok {
		return PrinterConfig{}, fmt.Errorf("mode is not a string")
	}
	config.ConnectivityMode, err = ParseConnectivityMode(modeValue)
	if err != nil {
		return PrinterConfig{}, err
	}

	switch config.ConnectivityMode {
	case USB:
		vendorValue, ok := printerValue["vendor"].(string)
		if !ok {
			return PrinterConfig{}, fmt.Errorf("vendor is not a string")
		}

		vendorValue = strings.TrimPrefix(vendorValue, "0x")
		vendor, err := strconv.ParseUint(vendorValue, 16, 16)
		if err != nil {
			return PrinterConfig{}, fmt.Errorf("failed to parse vendor: %v", err)
		}

		productValue, ok := printerValue["product"].(string)
		if !ok {
			return PrinterConfig{}, fmt.Errorf("product is not a string")
		}

		productValue = strings.TrimPrefix(productValue, "0x")
		product, err := strconv.ParseUint(productValue, 16, 16)
		if err != nil {
			return PrinterConfig{}, fmt.Errorf("failed to parse product: %v", err)
		}

		config.USBVendor = uint16(vendor)
		config.USBProduct = uint16(product)

	case LP:
		lpPrinterName, ok := printerValue["name"].(string)
		if !ok {
			return PrinterConfig{}, fmt.Errorf("name is not a string")
		}
		config.LPPrinterName = lpPrinterName
	}
	return config, nil
}

type PrinterConfig struct {
	ConnectivityMode ConnectivityMode
	USBVendor        uint16
	USBProduct       uint16
	LPPrinterName    string
}

type ConnectivityMode int

const (
	Unset ConnectivityMode = iota
	USB
	LP
)

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
