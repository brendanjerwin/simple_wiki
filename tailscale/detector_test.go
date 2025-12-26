package tailscale_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"tailscale.com/ipn/ipnstate"

	"github.com/brendanjerwin/simple_wiki/tailscale"
)

// mockStatusProvider implements StatusProvider for testing.
type mockStatusProvider struct {
	status *ipnstate.Status
	err    error
}

func (m *mockStatusProvider) StatusWithoutPeers(_ context.Context) (*ipnstate.Status, error) {
	return m.status, m.err
}

var _ = Describe("LocalDetector", func() {
	Describe("NewDetector", func() {
		When("creating a new detector", func() {
			var detector *tailscale.LocalDetector

			BeforeEach(func() {
				detector = tailscale.NewDetector()
			})

			It("should not be nil", func() {
				Expect(detector).NotTo(BeNil())
			})
		})
	})

	Describe("Detect", func() {
		When("tailscale client returns an error", func() {
			var (
				detector *tailscale.LocalDetector
				status   *tailscale.Status
				err      error
			)

			BeforeEach(func() {
				provider := &mockStatusProvider{
					status: nil,
					err:    errors.New("connection refused"),
				}
				detector = tailscale.NewDetectorWithProvider(provider)
				status, err = detector.Detect(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status with Available=false", func() {
				Expect(status.Available).To(BeFalse())
			})
		})

		When("tailscale backend is not running", func() {
			var (
				detector *tailscale.LocalDetector
				status   *tailscale.Status
				err      error
			)

			BeforeEach(func() {
				provider := &mockStatusProvider{
					status: &ipnstate.Status{
						BackendState: "Stopped",
						Self: &ipnstate.PeerStatus{
							HostName: "my-laptop",
						},
					},
					err: nil,
				}
				detector = tailscale.NewDetectorWithProvider(provider)
				status, err = detector.Detect(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status with Available=false", func() {
				Expect(status.Available).To(BeFalse())
			})

			It("should include the hostname", func() {
				Expect(status.Hostname).To(Equal("my-laptop"))
			})
		})

		When("tailscale is running with cert domains", func() {
			var (
				detector *tailscale.LocalDetector
				status   *tailscale.Status
				err      error
			)

			BeforeEach(func() {
				provider := &mockStatusProvider{
					status: &ipnstate.Status{
						BackendState: "Running",
						CertDomains:  []string{"my-laptop.tailnet.ts.net"},
						CurrentTailnet: &ipnstate.TailnetStatus{
							Name: "tailnet.ts.net",
						},
						Self: &ipnstate.PeerStatus{
							HostName: "my-laptop",
							DNSName:  "my-laptop.tailnet.ts.net.",
						},
					},
					err: nil,
				}
				detector = tailscale.NewDetectorWithProvider(provider)
				status, err = detector.Detect(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status with Available=true", func() {
				Expect(status.Available).To(BeTrue())
			})

			It("should use the cert domain as DNSName", func() {
				Expect(status.DNSName).To(Equal("my-laptop.tailnet.ts.net"))
			})

			It("should include the tailnet name", func() {
				Expect(status.TailnetName).To(Equal("tailnet.ts.net"))
			})

			It("should include the hostname", func() {
				Expect(status.Hostname).To(Equal("my-laptop"))
			})
		})

		When("tailscale is running without cert domains", func() {
			var (
				detector *tailscale.LocalDetector
				status   *tailscale.Status
				err      error
			)

			BeforeEach(func() {
				provider := &mockStatusProvider{
					status: &ipnstate.Status{
						BackendState: "Running",
						CertDomains:  []string{},
						CurrentTailnet: &ipnstate.TailnetStatus{
							Name: "tailnet.ts.net",
						},
						Self: &ipnstate.PeerStatus{
							HostName: "my-laptop",
							DNSName:  "my-laptop.tailnet.ts.net.",
						},
					},
					err: nil,
				}
				detector = tailscale.NewDetectorWithProvider(provider)
				status, err = detector.Detect(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status with Available=true", func() {
				Expect(status.Available).To(BeTrue())
			})

			It("should use the Self DNSName with trailing dot removed", func() {
				Expect(status.DNSName).To(Equal("my-laptop.tailnet.ts.net"))
			})
		})

		When("tailscale is running but Self is nil", func() {
			var (
				detector *tailscale.LocalDetector
				status   *tailscale.Status
				err      error
			)

			BeforeEach(func() {
				provider := &mockStatusProvider{
					status: &ipnstate.Status{
						BackendState: "Running",
						CertDomains:  []string{"my-laptop.tailnet.ts.net"},
						CurrentTailnet: &ipnstate.TailnetStatus{
							Name: "tailnet.ts.net",
						},
						Self: nil,
					},
					err: nil,
				}
				detector = tailscale.NewDetectorWithProvider(provider)
				status, err = detector.Detect(context.Background())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return status with Available=true", func() {
				Expect(status.Available).To(BeTrue())
			})

			It("should have empty hostname", func() {
				Expect(status.Hostname).To(BeEmpty())
			})
		})
	})
})
