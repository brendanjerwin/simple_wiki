//revive:disable:dot-imports
package observability_test

import (
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockPageReaderWriter is a mock implementation for testing wiki metrics persistence.
type mockPageReaderWriter struct {
	frontmatter map[string]map[string]any
	markdown    map[string]string
}

func newMockPageReaderWriter() *mockPageReaderWriter {
	return &mockPageReaderWriter{
		frontmatter: make(map[string]map[string]any),
		markdown:    make(map[string]string),
	}
}

func (m *mockPageReaderWriter) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if fm, ok := m.frontmatter[identifier]; ok {
		return identifier, fm, nil
	}
	return identifier, nil, nil
}

func (m *mockPageReaderWriter) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if md, ok := m.markdown[identifier]; ok {
		return identifier, md, nil
	}
	return identifier, "", nil
}

func (m *mockPageReaderWriter) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.frontmatter[identifier] = fm
	return nil
}

func (m *mockPageReaderWriter) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.markdown[identifier] = md
	return nil
}

var _ = Describe("WikiMetricsRecorder", func() {
	Describe("NewWikiMetricsRecorder", func() {
		When("creating with valid config", func() {
			var recorder *observability.WikiMetricsRecorder

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{
					PageWriter: mock,
					PageReader: mock,
				})
			})

			It("should return a non-nil recorder", func() {
				Expect(recorder).ToNot(BeNil())
			})
		})

		When("creating without page access", func() {
			var recorder *observability.WikiMetricsRecorder

			BeforeEach(func() {
				recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
			})

			It("should return a non-nil recorder", func() {
				Expect(recorder).ToNot(BeNil())
			})
		})
	})

	Describe("RecordHTTPRequest", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording multiple requests", func() {
			BeforeEach(func() {
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
			})

			It("should increment the HTTP requests counter", func() {
				stats := recorder.GetStats()
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(3)))
			})
		})
	})

	Describe("RecordHTTPError", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording errors", func() {
			BeforeEach(func() {
				recorder.RecordHTTPError()
				recorder.RecordHTTPError()
			})

			It("should increment the HTTP errors counter", func() {
				stats := recorder.GetStats()
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCRequest", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording multiple requests", func() {
			BeforeEach(func() {
				recorder.RecordGRPCRequest()
				recorder.RecordGRPCRequest()
			})

			It("should increment the gRPC requests counter", func() {
				stats := recorder.GetStats()
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCError", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording errors", func() {
			BeforeEach(func() {
				recorder.RecordGRPCError()
			})

			It("should increment the gRPC errors counter", func() {
				stats := recorder.GetStats()
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(1)))
			})
		})
	})

	Describe("RecordTailscaleLookup", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording successful lookups", func() {
			BeforeEach(func() {
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
			})

			It("should increment the lookups counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(2)))
			})

			It("should increment the successes counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleSuccesses).To(Equal(int64(2)))
			})
		})

		When("recording failed lookups", func() {
			BeforeEach(func() {
				recorder.RecordTailscaleLookup(observability.ResultFailure)
			})

			It("should increment the lookups counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
			})

			It("should increment the failures counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleFailures).To(Equal(int64(1)))
			})
		})

		When("recording not_tailnet lookups", func() {
			BeforeEach(func() {
				recorder.RecordTailscaleLookup(observability.ResultNotTailnet)
			})

			It("should increment the lookups counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
			})

			It("should increment the not_tailnet counter", func() {
				stats := recorder.GetStats()
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(1)))
			})
		})
	})

	Describe("RecordHeaderExtraction", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("recording extractions", func() {
			BeforeEach(func() {
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
			})

			It("should increment the header extractions counter", func() {
				stats := recorder.GetStats()
				Expect(stats.HeaderExtractions).To(Equal(int64(3)))
			})
		})
	})

	Describe("Persist", func() {
		var (
			recorder *observability.WikiMetricsRecorder
			mock     *mockPageReaderWriter
		)

		BeforeEach(func() {
			mock = newMockPageReaderWriter()
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{
				PageWriter: mock,
				PageReader: mock,
			})
		})

		When("persisting metrics with data", func() {
			var persistErr error

			BeforeEach(func() {
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPError()
				recorder.RecordGRPCRequest()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordHeaderExtraction()

				persistErr = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(persistErr).ToNot(HaveOccurred())
			})

			It("should write frontmatter to the observability page", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				Expect(fm).ToNot(BeNil())
			})

			It("should include the identifier", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				Expect(fm["identifier"]).To(Equal(observability.ObservabilityMetricsPage))
			})

			It("should include the title", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				Expect(fm["title"]).To(Equal("Observability Metrics"))
			})

			It("should include observability data", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				obsData, ok := fm["observability"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(obsData).ToNot(BeNil())
			})

			It("should include HTTP metrics", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				obsData := fm["observability"].(map[string]any)
				httpData, ok := obsData["http"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(httpData["requests_total"]).To(Equal(int64(2)))
				Expect(httpData["errors_total"]).To(Equal(int64(1)))
			})

			It("should include gRPC metrics", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				obsData := fm["observability"].(map[string]any)
				grpcData, ok := obsData["grpc"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(grpcData["requests_total"]).To(Equal(int64(1)))
			})

			It("should include Tailscale metrics", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				obsData := fm["observability"].(map[string]any)
				tsData, ok := obsData["tailscale"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(tsData["lookups_total"]).To(Equal(int64(1)))
				Expect(tsData["successes_total"]).To(Equal(int64(1)))
				Expect(tsData["header_extractions_total"]).To(Equal(int64(1)))
			})
		})

		When("persisting without page access configured", func() {
			var persistErr error

			BeforeEach(func() {
				recorderWithoutAccess := observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
				persistErr = recorderWithoutAccess.Persist()
			})

			It("should not return an error", func() {
				Expect(persistErr).ToNot(HaveOccurred())
			})
		})
	})

	Describe("PersistWithMarkdown", func() {
		var (
			recorder *observability.WikiMetricsRecorder
			mock     *mockPageReaderWriter
		)

		BeforeEach(func() {
			mock = newMockPageReaderWriter()
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{
				PageWriter: mock,
				PageReader: mock,
			})
		})

		When("persisting with markdown", func() {
			var persistErr error

			BeforeEach(func() {
				recorder.RecordHTTPRequest()
				recorder.RecordGRPCRequest()

				persistErr = recorder.PersistWithMarkdown()
			})

			It("should not return an error", func() {
				Expect(persistErr).ToNot(HaveOccurred())
			})

			It("should write markdown to the observability page", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).ToNot(BeEmpty())
			})

			It("should include a title in markdown", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("# Observability Metrics"))
			})

			It("should include HTTP metrics section", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("## HTTP Metrics"))
			})

			It("should include gRPC metrics section", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("## gRPC Metrics"))
			})

			It("should include Tailscale metrics section", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("## Tailscale Identity Metrics"))
			})
		})
	})

	Describe("GetStats", func() {
		var recorder *observability.WikiMetricsRecorder

		BeforeEach(func() {
			recorder = observability.NewWikiMetricsRecorder(observability.WikiMetricsRecorderConfig{})
		})

		When("getting stats with no recorded data", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				stats = recorder.GetStats()
			})

			It("should return zero values", func() {
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(0)))
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(0)))
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(0)))
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(0)))
				Expect(stats.TailscaleLookups).To(Equal(int64(0)))
				Expect(stats.TailscaleSuccesses).To(Equal(int64(0)))
				Expect(stats.TailscaleFailures).To(Equal(int64(0)))
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(0)))
				Expect(stats.HeaderExtractions).To(Equal(int64(0)))
			})
		})

		When("getting stats after recording various metrics", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPError()
				recorder.RecordGRPCRequest()
				recorder.RecordGRPCError()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordTailscaleLookup(observability.ResultFailure)
				recorder.RecordTailscaleLookup(observability.ResultNotTailnet)
				recorder.RecordHeaderExtraction()

				stats = recorder.GetStats()
			})

			It("should return correct HTTP request count", func() {
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(2)))
			})

			It("should return correct HTTP error count", func() {
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(1)))
			})

			It("should return correct gRPC request count", func() {
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(1)))
			})

			It("should return correct gRPC error count", func() {
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(1)))
			})

			It("should return correct Tailscale lookups count", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(3)))
			})

			It("should return correct Tailscale successes count", func() {
				Expect(stats.TailscaleSuccesses).To(Equal(int64(1)))
			})

			It("should return correct Tailscale failures count", func() {
				Expect(stats.TailscaleFailures).To(Equal(int64(1)))
			})

			It("should return correct Tailscale not_tailnet count", func() {
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(1)))
			})

			It("should return correct header extractions count", func() {
				Expect(stats.HeaderExtractions).To(Equal(int64(1)))
			})
		})
	})
})
