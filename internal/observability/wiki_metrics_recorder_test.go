//revive:disable:dot-imports
package observability_test

import (
	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// mockPageReaderWriter is a mock implementation for testing wiki metrics persistence.
// It implements observability.PageReader and wikipage.PageWriter.
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
		return identifier, wikipage.Markdown(md), nil
	}
	return identifier, "", nil
}

func (m *mockPageReaderWriter) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.frontmatter[identifier] = fm
	return nil
}

func (m *mockPageReaderWriter) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.markdown[identifier] = string(md)
	return nil
}

// createRecorder is a helper to create a recorder for tests that don't need page access.
func createRecorder() *observability.WikiMetricsRecorder {
	recorder, _ := observability.NewWikiMetricsRecorder(nil, nil, nil, nil)
	return recorder
}

// createRecorderWithMock is a helper to create a recorder with mock page access and job queue.
func createRecorderWithMock(mock *mockPageReaderWriter) *observability.WikiMetricsRecorder {
	coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
	recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
	return recorder
}

var _ = Describe("WikiMetricsRecorder", func() {
	Describe("NewWikiMetricsRecorder", func() {
		When("creating with all dependencies", func() {
			It("should return a recorder without error", func() {
				mock := newMockPageReaderWriter()
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, err := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(recorder).ToNot(BeNil())
			})
		})

		When("creating without any dependencies (metrics-only mode)", func() {
			It("should return a recorder without error", func() {
				recorder, err := observability.NewWikiMetricsRecorder(nil, nil, nil, nil)
				Expect(err).ToNot(HaveOccurred())
				Expect(recorder).ToNot(BeNil())
			})
		})

		When("creating with only pageWriter and pageReader (missing jobQueue)", func() {
			It("should return an error", func() {
				mock := newMockPageReaderWriter()
				_, err := observability.NewWikiMetricsRecorder(mock, mock, nil, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("all be provided together"))
			})
		})

		When("creating with only pageWriter", func() {
			It("should return an error", func() {
				mock := newMockPageReaderWriter()
				_, err := observability.NewWikiMetricsRecorder(mock, nil, nil, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("all be provided together"))
			})
		})

		When("creating with only pageReader", func() {
			It("should return an error", func() {
				mock := newMockPageReaderWriter()
				_, err := observability.NewWikiMetricsRecorder(nil, mock, nil, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("all be provided together"))
			})
		})

		When("creating with only jobQueue", func() {
			It("should return an error", func() {
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				_, err := observability.NewWikiMetricsRecorder(nil, nil, coordinator, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("all be provided together"))
			})
		})
	})

	Describe("RecordHTTPRequest", func() {
		When("recording multiple requests", func() {
			It("should increment the HTTP requests counter", func() {
				recorder := createRecorder()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				stats := recorder.GetStats()
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(3)))
			})
		})
	})

	Describe("RecordHTTPError", func() {
		When("recording errors", func() {
			It("should increment the HTTP errors counter", func() {
				recorder := createRecorder()
				recorder.RecordHTTPError()
				recorder.RecordHTTPError()
				stats := recorder.GetStats()
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCRequest", func() {
		When("recording multiple requests", func() {
			It("should increment the gRPC requests counter", func() {
				recorder := createRecorder()
				recorder.RecordGRPCRequest()
				recorder.RecordGRPCRequest()
				stats := recorder.GetStats()
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCError", func() {
		When("recording errors", func() {
			It("should increment the gRPC errors counter", func() {
				recorder := createRecorder()
				recorder.RecordGRPCError()
				stats := recorder.GetStats()
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(1)))
			})
		})
	})

	Describe("RecordTailscaleLookup", func() {
		When("recording successful lookups", func() {
			It("should increment the lookups and successes counters", func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(2)))
				Expect(stats.TailscaleSuccesses).To(Equal(int64(2)))
			})
		})

		When("recording failed lookups", func() {
			It("should increment the lookups and failures counters", func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultFailure)
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
				Expect(stats.TailscaleFailures).To(Equal(int64(1)))
			})
		})

		When("recording not_tailnet lookups", func() {
			It("should increment the lookups and not_tailnet counters", func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultNotTailnet)
				stats := recorder.GetStats()
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(1)))
			})
		})
	})

	Describe("RecordHeaderExtraction", func() {
		When("recording extractions", func() {
			It("should increment the header extractions counter", func() {
				recorder := createRecorder()
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
				stats := recorder.GetStats()
				Expect(stats.HeaderExtractions).To(Equal(int64(3)))
			})
		})
	})

	Describe("Persist", func() {
		When("persisting metrics with data", func() {
			It("should write frontmatter with correct data", func() {
				mock := newMockPageReaderWriter()
				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPError()
				recorder.RecordGRPCRequest()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordHeaderExtraction()

				err := recorder.Persist()
				Expect(err).ToNot(HaveOccurred())

				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				Expect(fm).ToNot(BeNil())
				Expect(fm["identifier"]).To(Equal(observability.ObservabilityMetricsPage))
				Expect(fm["title"]).To(Equal("Observability Metrics"))

				obsData, ok := fm["observability"].(map[string]any)
				Expect(ok).To(BeTrue())

				httpData, ok := obsData["http"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(httpData["requests_total"]).To(Equal(int64(2)))
				Expect(httpData["errors_total"]).To(Equal(int64(1)))

				grpcData, ok := obsData["grpc"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(grpcData["requests_total"]).To(Equal(int64(1)))

				tsData, ok := obsData["tailscale"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(tsData["lookups_total"]).To(Equal(int64(1)))
				Expect(tsData["successes_total"]).To(Equal(int64(1)))
				Expect(tsData["header_extractions_total"]).To(Equal(int64(1)))
			})
		})

		When("persisting without page access configured", func() {
			It("should not return an error", func() {
				recorder := createRecorder()
				err := recorder.Persist()
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Persist creating new page", func() {
		When("page does not exist", func() {
			var mock *mockPageReaderWriter
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()
				recorder.RecordGRPCRequest()

				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should write markdown template with correct sections", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).ToNot(BeEmpty())
				Expect(md).To(ContainSubstring("# Observability Metrics"))
				Expect(md).To(ContainSubstring("## HTTP Metrics"))
				Expect(md).To(ContainSubstring("## gRPC Metrics"))
				Expect(md).To(ContainSubstring("## Tailscale Identity Metrics"))
			})

			It("should use template syntax for values", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("{{ .observability.http.requests_total }}"))
			})
		})

		When("page already exists", func() {
			var mock *mockPageReaderWriter
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				// Pre-populate the frontmatter to simulate existing page
				mock.frontmatter[observability.ObservabilityMetricsPage] = map[string]any{
					"title": "Existing Page",
				}
				mock.markdown[observability.ObservabilityMetricsPage] = "# Custom Content"

				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()

				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not overwrite the markdown", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(Equal("# Custom Content"))
			})

			It("should update the frontmatter", func() {
				fm := mock.frontmatter[observability.ObservabilityMetricsPage]
				Expect(fm["observability"]).ToNot(BeNil())
			})
		})
	})

	Describe("GetStats", func() {
		When("getting stats with no recorded data", func() {
			It("should return zero values", func() {
				recorder := createRecorder()
				stats := recorder.GetStats()
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
			It("should return correct values", func() {
				recorder := createRecorder()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPError()
				recorder.RecordGRPCRequest()
				recorder.RecordGRPCError()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordTailscaleLookup(observability.ResultFailure)
				recorder.RecordTailscaleLookup(observability.ResultNotTailnet)
				recorder.RecordHeaderExtraction()

				stats := recorder.GetStats()
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(2)))
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(1)))
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(1)))
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(1)))
				Expect(stats.TailscaleLookups).To(Equal(int64(3)))
				Expect(stats.TailscaleSuccesses).To(Equal(int64(1)))
				Expect(stats.TailscaleFailures).To(Equal(int64(1)))
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(1)))
				Expect(stats.HeaderExtractions).To(Equal(int64(1)))
			})
		})
	})
})
