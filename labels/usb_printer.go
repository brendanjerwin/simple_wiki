package labels

import (
	"errors"

	"github.com/karalabe/usb"
)

var (
	ErrorDeviceNotFound           = errors.New("Can not detect any USB printer")
	ErrorEndpointNotAccessable    = errors.New("Can not access endpoint")
	ErrorVendorNotSpecified       = errors.New("Vendor ID is not specified")
	ErrorPlatformDoesntSupportUsb = errors.New("Platform doesn't support USB")
)

func GetUSBPrinter(config PrinterConfig) (Printer, error) {
	if config.USBVendor == 0 {
		return nil, ErrorVendorNotSpecified
	}

	if !usb.Supported() {
		return nil, ErrorPlatformDoesntSupportUsb
	}

	devices, err := usb.EnumerateRaw(config.USBVendor, config.USBProduct)
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
