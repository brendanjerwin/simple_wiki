package tailscale

import (
	"context"

	"tailscale.com/client/local"
	"tailscale.com/ipn/ipnstate"
)

// BackendState constants for Tailscale daemon states.
const (
	BackendStateRunning = "Running"
)

// Status represents the current Tailscale connection status.
type Status struct {
	Available   bool
	DNSName     string // Full DNS name (e.g., "my-laptop.tailnet-name.ts.net")
	TailnetName string // Tailnet name
	Hostname    string // Local hostname
}

// TailscaleDetector provides detection of Tailscale availability.
type TailscaleDetector interface {
	Detect(ctx context.Context) (*Status, error)
}

// StatusQuerier abstracts the Tailscale client for testing.
type StatusQuerier interface {
	StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error)
}

// localClientAdapter wraps *local.Client to implement StatusQuerier.
type localClientAdapter struct {
	client *local.Client
}

func (a *localClientAdapter) StatusWithoutPeers(ctx context.Context) (*ipnstate.Status, error) {
	return a.client.StatusWithoutPeers(ctx)
}

// LocalDetector uses the local tailscaled daemon to detect Tailscale status.
type LocalDetector struct {
	statusProvider StatusQuerier
}

// NewDetector creates a new Tailscale detector.
func NewDetector() *LocalDetector {
	return &LocalDetector{
		statusProvider: &localClientAdapter{client: &local.Client{}},
	}
}

// NewDetectorWithProvider creates a LocalDetector with a custom status provider (for testing).
func NewDetectorWithProvider(provider StatusQuerier) *LocalDetector {
	return &LocalDetector{
		statusProvider: provider,
	}
}

// Detect checks if Tailscale is available and returns status.
// Returns Status{Available: false} on error (graceful fallback).
func (d *LocalDetector) Detect(ctx context.Context) (*Status, error) {
	ipnStatus, err := d.statusProvider.StatusWithoutPeers(ctx)
	if err != nil {
		// Tailscale not available - graceful fallback
		return &Status{Available: false}, nil
	}

	// Check if we have a valid state
	if ipnStatus.BackendState != BackendStateRunning {
		hostname := ""
		if ipnStatus.Self != nil {
			hostname = ipnStatus.Self.HostName
		}
		return &Status{
			Available: false,
			Hostname:  hostname,
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

	tailnetName := ""
	if ipnStatus.CurrentTailnet != nil {
		tailnetName = ipnStatus.CurrentTailnet.Name
	}

	return &Status{
		Available:   true,
		DNSName:     dnsName,
		TailnetName: tailnetName,
		Hostname:    hostname,
	}, nil
}

// Ensure LocalDetector implements TailscaleDetector
var _ TailscaleDetector = (*LocalDetector)(nil)
