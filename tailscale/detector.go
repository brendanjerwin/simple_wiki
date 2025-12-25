package tailscale

import (
	"context"

	"tailscale.com/client/local"
)

// Status represents the current Tailscale connection status.
type Status struct {
	Available   bool
	DNSName     string // Full DNS name (e.g., "my-laptop.tailnet-name.ts.net")
	TailnetName string // Tailnet name
	Hostname    string // Local hostname
}

// IDetectTailscale provides detection of Tailscale availability.
type IDetectTailscale interface {
	Detect(ctx context.Context) (*Status, error)
}

// Detector uses the local tailscaled daemon to detect Tailscale status.
type Detector struct {
	client *local.Client
}

// NewDetector creates a new Tailscale detector.
func NewDetector() *Detector {
	return &Detector{
		client: &local.Client{},
	}
}

// Detect checks if Tailscale is available and returns status.
// Returns Status{Available: false} on error (graceful fallback).
func (d *Detector) Detect(ctx context.Context) (*Status, error) {
	ipnStatus, err := d.client.StatusWithoutPeers(ctx)
	if err != nil {
		// Tailscale not available - graceful fallback
		return &Status{Available: false}, nil
	}

	// Check if we have a valid state
	if ipnStatus.BackendState != "Running" {
		return &Status{
			Available: false,
			Hostname:  ipnStatus.Self.HostName,
		}, nil
	}

	// Extract DNS name from the first cert domain or self DNS name
	dnsName := ""
	if len(ipnStatus.CertDomains) > 0 {
		dnsName = ipnStatus.CertDomains[0]
	} else if ipnStatus.Self != nil && ipnStatus.Self.DNSName != "" {
		// Remove trailing dot if present
		dnsName = ipnStatus.Self.DNSName
		if len(dnsName) > 0 && dnsName[len(dnsName)-1] == '.' {
			dnsName = dnsName[:len(dnsName)-1]
		}
	}

	hostname := ""
	if ipnStatus.Self != nil {
		hostname = ipnStatus.Self.HostName
	}

	return &Status{
		Available:   true,
		DNSName:     dnsName,
		TailnetName: ipnStatus.CurrentTailnet.Name,
		Hostname:    hostname,
	}, nil
}

// Ensure Detector implements IDetectTailscale
var _ IDetectTailscale = (*Detector)(nil)
