package labels

import (
	"errors"

	"github.com/karalabe/usb"
)

// ErrorDeviceNotFound indicates that no USB printer was detected.
var ErrorDeviceNotFound = errors.New("can not detect any USB printer")

// ErrorEndpointNotAccessable indicates that the USB endpoint is not accessible.
var ErrorEndpointNotAccessable = errors.New("can not access endpoint")

// ErrorVendorNotSpecified indicates that the vendor ID was not specified.
var ErrorVendorNotSpecified = errors.New("vendor ID is not specified")

// ErrorPlatformDoesntSupportUsb indicates that the platform does not support USB.
var ErrorPlatformDoesntSupportUsb = errors.New("platform doesn't support USB")

// GetUSBPrinter returns a Printer for a USB printer.
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
