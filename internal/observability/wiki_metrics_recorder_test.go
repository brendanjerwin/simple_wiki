//revive:disable:dot-imports
package observability_test

import (
	"time"

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

// failingPageReaderWriter wraps mockPageReaderWriter and can simulate read/write failures.
type failingPageReaderWriter struct {
	*mockPageReaderWriter
	failFrontmatterWrite bool
	failMarkdownWrite    bool
	failFrontmatterRead  bool
	failMarkdownRead     bool
}

func (f *failingPageReaderWriter) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	if f.failFrontmatterRead {
		return "", nil, errFrontmatterReadFailed
	}
	return f.mockPageReaderWriter.ReadFrontMatter(identifier)
}

func (f *failingPageReaderWriter) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if f.failMarkdownRead {
		return "", "", errMarkdownReadFailed
	}
	return f.mockPageReaderWriter.ReadMarkdown(identifier)
}

func (f *failingPageReaderWriter) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	if f.failFrontmatterWrite {
		return errFrontmatterWriteFailed
	}
	return f.mockPageReaderWriter.WriteFrontMatter(identifier, fm)
}

func (f *failingPageReaderWriter) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	if f.failMarkdownWrite {
		return errMarkdownWriteFailed
	}
	return f.mockPageReaderWriter.WriteMarkdown(identifier, md)
}

var errFrontmatterWriteFailed = &testError{msg: "frontmatter write failed"}
var errMarkdownWriteFailed = &testError{msg: "markdown write failed"}
var errFrontmatterReadFailed = &testError{msg: "frontmatter read failed"}
var errMarkdownReadFailed = &testError{msg: "markdown read failed"}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// mockLogger captures log messages for testing.
type mockLogger struct {
	infoMessages  []string
	warnMessages  []string
	errorMessages []string
}

func (m *mockLogger) Info(format string, args ...any) {
	m.infoMessages = append(m.infoMessages, format)
}

func (m *mockLogger) Warn(format string, args ...any) {
	m.warnMessages = append(m.warnMessages, format)
}

func (m *mockLogger) Error(format string, args ...any) {
	m.errorMessages = append(m.errorMessages, format)
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

// createRecorderWithMockAndLogger is a helper to create a recorder with mock page access, job queue, and logger.
func createRecorderWithMockAndLogger(mock *mockPageReaderWriter, logger *mockLogger) *observability.WikiMetricsRecorder {
	coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
	recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, logger)
	return recorder
}

var _ = Describe("WikiMetricsRecorder", func() {
	Describe("NewWikiMetricsRecorder", func() {
		When("creating with all dependencies", func() {
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, err = observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a non-nil recorder", func() {
				Expect(recorder).ToNot(BeNil())
			})
		})

		When("creating without any dependencies (metrics-only mode)", func() {
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				recorder, err = observability.NewWikiMetricsRecorder(nil, nil, nil, nil)
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return a non-nil recorder", func() {
				Expect(recorder).ToNot(BeNil())
			})
		})

		When("creating with only pageWriter and pageReader (missing jobQueue)", func() {
			var err error

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				_, err = observability.NewWikiMetricsRecorder(mock, mock, nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate all dependencies must be provided together", func() {
				Expect(err).To(MatchError(ContainSubstring("all be provided together")))
			})
		})

		When("creating with only pageWriter", func() {
			var err error

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				_, err = observability.NewWikiMetricsRecorder(mock, nil, nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate all dependencies must be provided together", func() {
				Expect(err).To(MatchError(ContainSubstring("all be provided together")))
			})
		})

		When("creating with only pageReader", func() {
			var err error

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				_, err = observability.NewWikiMetricsRecorder(nil, mock, nil, nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate all dependencies must be provided together", func() {
				Expect(err).To(MatchError(ContainSubstring("all be provided together")))
			})
		})

		When("creating with only jobQueue", func() {
			var err error

			BeforeEach(func() {
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				_, err = observability.NewWikiMetricsRecorder(nil, nil, coordinator, nil)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should indicate all dependencies must be provided together", func() {
				Expect(err).To(MatchError(ContainSubstring("all be provided together")))
			})
		})
	})

	Describe("RecordHTTPRequest", func() {
		When("recording multiple requests", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				stats = recorder.GetStats()
			})

			It("should increment the HTTP requests counter", func() {
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(3)))
			})
		})
	})

	Describe("RecordHTTPError", func() {
		When("recording errors", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordHTTPError()
				recorder.RecordHTTPError()
				stats = recorder.GetStats()
			})

			It("should increment the HTTP errors counter", func() {
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCRequest", func() {
		When("recording multiple requests", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordGRPCRequest()
				recorder.RecordGRPCRequest()
				stats = recorder.GetStats()
			})

			It("should increment the gRPC requests counter", func() {
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(2)))
			})
		})
	})

	Describe("RecordGRPCError", func() {
		When("recording errors", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordGRPCError()
				stats = recorder.GetStats()
			})

			It("should increment the gRPC errors counter", func() {
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(1)))
			})
		})
	})

	Describe("RecordTailscaleLookup", func() {
		When("recording successful lookups", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				stats = recorder.GetStats()
			})

			It("should increment the lookups counter", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(2)))
			})

			It("should increment the successes counter", func() {
				Expect(stats.TailscaleSuccesses).To(Equal(int64(2)))
			})
		})

		When("recording failed lookups", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultFailure)
				stats = recorder.GetStats()
			})

			It("should increment the lookups counter", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
			})

			It("should increment the failures counter", func() {
				Expect(stats.TailscaleFailures).To(Equal(int64(1)))
			})
		})

		When("recording not_tailnet lookups", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordTailscaleLookup(observability.ResultNotTailnet)
				stats = recorder.GetStats()
			})

			It("should increment the lookups counter", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
			})

			It("should increment the not_tailnet counter", func() {
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(1)))
			})
		})

		When("recording an unknown result type", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				// Use an undefined result type value
				recorder.RecordTailscaleLookup(observability.IdentityLookupResult("unknown_result"))
				stats = recorder.GetStats()
			})

			It("should increment the lookups counter", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(1)))
			})

			It("should not increment the successes counter", func() {
				Expect(stats.TailscaleSuccesses).To(Equal(int64(0)))
			})

			It("should not increment the failures counter", func() {
				Expect(stats.TailscaleFailures).To(Equal(int64(0)))
			})

			It("should not increment the not_tailnet counter", func() {
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(0)))
			})
		})
	})

	Describe("RecordHeaderExtraction", func() {
		When("recording extractions", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
				recorder.RecordHeaderExtraction()
				stats = recorder.GetStats()
			})

			It("should increment the header extractions counter", func() {
				Expect(stats.HeaderExtractions).To(Equal(int64(3)))
			})
		})
	})

	Describe("Persist", func() {
		When("persisting metrics with data", func() {
			var fm map[string]any
			var obsData map[string]any
			var httpData map[string]any
			var grpcData map[string]any
			var tsData map[string]any
			var err error

			BeforeEach(func() {
				mock := newMockPageReaderWriter()
				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPRequest()
				recorder.RecordHTTPError()
				recorder.RecordGRPCRequest()
				recorder.RecordTailscaleLookup(observability.ResultSuccess)
				recorder.RecordHeaderExtraction()

				err = recorder.Persist()
				fm = mock.frontmatter[observability.ObservabilityMetricsPage]
				if fm != nil {
					obsData, _ = fm["observability"].(map[string]any)
					if obsData != nil {
						httpData, _ = obsData["http"].(map[string]any)
						grpcData, _ = obsData["grpc"].(map[string]any)
						tsData, _ = obsData["tailscale"].(map[string]any)
					}
				}
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should write frontmatter", func() {
				Expect(fm).ToNot(BeNil())
			})

			It("should set the correct identifier", func() {
				Expect(fm["identifier"]).To(Equal(observability.ObservabilityMetricsPage))
			})

			It("should set the correct title", func() {
				Expect(fm["title"]).To(Equal("Observability Metrics"))
			})

			It("should include observability data", func() {
				Expect(obsData).ToNot(BeNil())
			})

			It("should record HTTP requests count", func() {
				Expect(httpData["requests_total"]).To(Equal(int64(2)))
			})

			It("should record HTTP errors count", func() {
				Expect(httpData["errors_total"]).To(Equal(int64(1)))
			})

			It("should record gRPC requests count", func() {
				Expect(grpcData["requests_total"]).To(Equal(int64(1)))
			})

			It("should record Tailscale lookups count", func() {
				Expect(tsData["lookups_total"]).To(Equal(int64(1)))
			})

			It("should record Tailscale successes count", func() {
				Expect(tsData["successes_total"]).To(Equal(int64(1)))
			})

			It("should record header extractions count", func() {
				Expect(tsData["header_extractions_total"]).To(Equal(int64(1)))
			})
		})

		When("persisting without page access configured", func() {
			var err error

			BeforeEach(func() {
				recorder := createRecorder()
				err = recorder.Persist()
			})

			It("should not return an error", func() {
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
				Expect(md).To(ContainSubstring("{{ .Map.observability.http.requests_total }}"))
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

		When("page exists with only default template", func() {
			var mock *mockPageReaderWriter
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				// Use the shared constant for default template
				mock.markdown[observability.ObservabilityMetricsPage] = wikipage.DefaultPageTemplate

				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()

				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should replace the default template with observability template", func() {
				md := mock.markdown[observability.ObservabilityMetricsPage]
				Expect(md).To(ContainSubstring("# Observability Metrics"))
				Expect(md).To(ContainSubstring("## HTTP Metrics"))
			})
		})
	})

	Describe("Persist dirty flag", func() {
		When("persisting without any recorded metrics", func() {
			var mock *mockPageReaderWriter
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				recorder := createRecorderWithMock(mock)
				// Don't record anything - dirty flag should be false
				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not write frontmatter", func() {
				Expect(mock.frontmatter).To(BeEmpty())
			})

			It("should not write markdown", func() {
				Expect(mock.markdown).To(BeEmpty())
			})
		})

		When("persisting after recording metrics", func() {
			var mock *mockPageReaderWriter
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				recorder = createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should write frontmatter", func() {
				Expect(mock.frontmatter[observability.ObservabilityMetricsPage]).ToNot(BeNil())
			})

			When("persisting again without new metrics", func() {
				BeforeEach(func() {
					// Clear the mock to verify nothing is written
					mock.frontmatter = make(map[string]map[string]any)
					mock.markdown = make(map[string]string)
					err = recorder.Persist()
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should skip writing", func() {
					Expect(mock.frontmatter).To(BeEmpty())
				})
			})

			When("persisting after new metrics are recorded", func() {
				BeforeEach(func() {
					// Clear the mock
					mock.frontmatter = make(map[string]map[string]any)
					mock.markdown = make(map[string]string)
					recorder.RecordGRPCRequest()
					err = recorder.Persist()
				})

				It("should not return an error", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should write frontmatter again", func() {
					Expect(mock.frontmatter[observability.ObservabilityMetricsPage]).ToNot(BeNil())
				})
			})
		})
	})

	Describe("PersistAsync", func() {
		When("jobQueue is configured", func() {
			var mock *mockPageReaderWriter

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				recorder.RecordHTTPRequest()
				recorder.PersistAsync()
			})

			It("should enqueue a job that persists metrics", func() {
				// Give the job queue time to process
				Eventually(func() map[string]any {
					return mock.frontmatter[observability.ObservabilityMetricsPage]
				}, 5*time.Second, 100*time.Millisecond).ShouldNot(BeNil())
			})
		})

		When("jobQueue is not configured", func() {
			var recorder *observability.WikiMetricsRecorder

			BeforeEach(func() {
				recorder, _ = observability.NewWikiMetricsRecorder(nil, nil, nil, nil)
				recorder.RecordHTTPRequest()
			})

			It("should not panic", func() {
				Expect(func() {
					recorder.PersistAsync()
				}).ToNot(Panic())
			})
		})
	})

	Describe("Shutdown", func() {
		When("recorder has dirty metrics", func() {
			var mock *mockPageReaderWriter
			var fm map[string]any
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				recorder := createRecorderWithMock(mock)
				recorder.RecordHTTPRequest()
				recorder.RecordGRPCRequest()
				err = recorder.Shutdown()
				fm = mock.frontmatter[observability.ObservabilityMetricsPage]
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should write frontmatter", func() {
				Expect(fm).ToNot(BeNil())
			})

			It("should include observability data", func() {
				Expect(fm["observability"]).ToNot(BeNil())
			})
		})

		When("recorder has no dirty metrics", func() {
			var mock *mockPageReaderWriter
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				recorder = createRecorderWithMock(mock)
				// Don't record anything
				err = recorder.Shutdown()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should not write anything", func() {
				Expect(mock.frontmatter).To(BeEmpty())
			})
		})

		When("recorder has no page access", func() {
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				recorder, _ = observability.NewWikiMetricsRecorder(nil, nil, nil, nil)
				recorder.RecordHTTPRequest()
				err = recorder.Shutdown()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("Persist error handling", func() {
		When("frontmatter read fails", func() {
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failFrontmatterRead:  true,
				}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ = observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should contain the underlying error message", func() {
				Expect(err).To(MatchError(ContainSubstring("frontmatter read failed")))
			})
		})

		When("markdown read fails", func() {
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failMarkdownRead:     true,
				}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should contain the underlying error message", func() {
				Expect(err).To(MatchError(ContainSubstring("markdown read failed")))
			})
		})

		When("frontmatter write fails", func() {
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failFrontmatterWrite: true,
				}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should contain the underlying error message", func() {
				Expect(err).To(MatchError(ContainSubstring("frontmatter write failed")))
			})
		})

		When("markdown write fails", func() {
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failMarkdownWrite:    true,
				}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, nil)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should contain the underlying error message", func() {
				Expect(err).To(MatchError(ContainSubstring("markdown write failed")))
			})
		})
	})

	Describe("GetStats", func() {
		When("getting stats with no recorded data", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
				recorder := createRecorder()
				stats = recorder.GetStats()
			})

			It("should return zero HTTP requests", func() {
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(0)))
			})

			It("should return zero HTTP errors", func() {
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(0)))
			})

			It("should return zero gRPC requests", func() {
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(0)))
			})

			It("should return zero gRPC errors", func() {
				Expect(stats.GRPCErrorsTotal).To(Equal(int64(0)))
			})

			It("should return zero Tailscale lookups", func() {
				Expect(stats.TailscaleLookups).To(Equal(int64(0)))
			})

			It("should return zero Tailscale successes", func() {
				Expect(stats.TailscaleSuccesses).To(Equal(int64(0)))
			})

			It("should return zero Tailscale failures", func() {
				Expect(stats.TailscaleFailures).To(Equal(int64(0)))
			})

			It("should return zero Tailscale not_tailnet", func() {
				Expect(stats.TailscaleNotTailnet).To(Equal(int64(0)))
			})

			It("should return zero header extractions", func() {
				Expect(stats.HeaderExtractions).To(Equal(int64(0)))
			})
		})

		When("getting stats after recording various metrics", func() {
			var stats observability.WikiMetricsStats

			BeforeEach(func() {
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
				stats = recorder.GetStats()
			})

			It("should return correct HTTP requests count", func() {
				Expect(stats.HTTPRequestsTotal).To(Equal(int64(2)))
			})

			It("should return correct HTTP errors count", func() {
				Expect(stats.HTTPErrorsTotal).To(Equal(int64(1)))
			})

			It("should return correct gRPC requests count", func() {
				Expect(stats.GRPCRequestsTotal).To(Equal(int64(1)))
			})

			It("should return correct gRPC errors count", func() {
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

	Describe("Logging", func() {
		When("persisting with a logger configured", func() {
			var mock *mockPageReaderWriter
			var logger *mockLogger
			var recorder *observability.WikiMetricsRecorder
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				logger = &mockLogger{}
				recorder = createRecorderWithMockAndLogger(mock, logger)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should log success message", func() {
				Expect(logger.infoMessages).To(ContainElement(ContainSubstring("Persisted observability metrics")))
			})
		})

		When("creating a new page with a logger configured", func() {
			var mock *mockPageReaderWriter
			var logger *mockLogger
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				logger = &mockLogger{}
				recorder := createRecorderWithMockAndLogger(mock, logger)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should log template creation", func() {
				Expect(logger.infoMessages).To(ContainElement(ContainSubstring("Created observability metrics page")))
			})
		})

		When("shutdown is called with a logger configured", func() {
			var mock *mockPageReaderWriter
			var logger *mockLogger
			var err error

			BeforeEach(func() {
				mock = newMockPageReaderWriter()
				logger = &mockLogger{}
				recorder := createRecorderWithMockAndLogger(mock, logger)
				recorder.RecordHTTPRequest()
				err = recorder.Shutdown()
			})

			It("should not return an error", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should log shutdown message", func() {
				Expect(logger.infoMessages).To(ContainElement(ContainSubstring("Persisting final metrics before shutdown")))
			})
		})

		When("frontmatter write fails with a logger configured", func() {
			var logger *mockLogger
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failFrontmatterWrite: true,
				}
				logger = &mockLogger{}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, logger)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should log the error", func() {
				Expect(logger.errorMessages).To(ContainElement(ContainSubstring("Failed to persist wiki metrics")))
			})
		})

		When("markdown write fails with a logger configured", func() {
			var logger *mockLogger
			var err error

			BeforeEach(func() {
				mock := &failingPageReaderWriter{
					mockPageReaderWriter: newMockPageReaderWriter(),
					failMarkdownWrite:    true,
				}
				logger = &mockLogger{}
				coordinator := jobs.NewJobQueueCoordinator(lumber.NewConsoleLogger(lumber.FATAL))
				recorder, _ := observability.NewWikiMetricsRecorder(mock, mock, coordinator, logger)
				recorder.RecordHTTPRequest()
				err = recorder.Persist()
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})

			It("should log the error", func() {
				Expect(logger.errorMessages).To(ContainElement(ContainSubstring("Failed to write metrics page template")))
			})
		})
	})
})
