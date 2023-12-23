package labels

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/karalabe/usb"
)

func PrintLabel(template_identifier string, identifer string, site common.IReadPages, query index.IQueryFrontmatterIndex) error {
	template_data, err := site.ReadMarkdown(template_identifier)
	if err != nil {
		return err
	}

	template_frontmatter, err := site.ReadFrontMatter(template_identifier)
	if err != nil {
		return err
	}

	printerValue, ok := template_frontmatter["label_printer"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("label_printer is not a map")
	}

	vendorValue, ok := printerValue["vendor"].(string)
	if !ok {
		return fmt.Errorf("vendor is not a string")
	}

	vendorValue = strings.TrimPrefix(vendorValue, "0x")
	vendor, err := strconv.ParseUint(vendorValue, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse vendor: %v", err)
	}

	productValue, ok := printerValue["product"].(string)
	if !ok {
		return fmt.Errorf("product is not a string")
	}

	productValue = strings.TrimPrefix(productValue, "0x")
	product, err := strconv.ParseUint(productValue, 16, 16)
	if err != nil {
		return fmt.Errorf("failed to parse product: %v", err)
	}

	config := UsbConfig{
		Vendor:  uint16(vendor),
		Product: uint16(product),
	}

	printer, err := GetPrinter(config)
	if err != nil {
		return err
	}
	defer printer.Close()

	frontmatter, err := site.ReadFrontMatter(identifer)
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

type UsbConfig struct {
	Vendor  uint16
	Product uint16
}

type Printer = usb.Device

var (
	ErrorDeviceNotFound           = errors.New("Can not detect any USB printer")
	ErrorEndpointNotAccessable    = errors.New("Can not access endpoint")
	ErrorVendorNotSpecified       = errors.New("Vendor ID is not specified")
	ErrorPlatformDoesntSupportUsb = errors.New("Platform doesn't support USB")
)

func GetPrinter(config UsbConfig) (Printer, error) {
	if config.Vendor == 0 {
		return nil, ErrorVendorNotSpecified
	}

	if !usb.Supported() {
		return nil, ErrorPlatformDoesntSupportUsb
	}

	devices, err := usb.EnumerateRaw(config.Vendor, config.Product)
	if err != nil {
		return nil, err
	}

	if len(devices) == 0 {
		return nil, ErrorDeviceNotFound
	}

	printer, err := devices[0].Open()
	if err != nil {
		return nil, err
	}
	return printer, nil
}
