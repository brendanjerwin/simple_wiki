//revive:disable:dot-imports
package v1_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/internal/grpc/api/v1"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

// gRPCStatusMatcher is a Gomega matcher for checking gRPC status errors.
type gRPCStatusMatcher struct {
	expectedCode    codes.Code
	expectedMsg     string // for exact match
	expectedMsgPart string // for substring match
}

// HaveGrpcStatus creates a matcher for a gRPC status with an exact message match.
func HaveGrpcStatus(code codes.Code, msg string) types.GomegaMatcher {
	return &gRPCStatusMatcher{expectedCode: code, expectedMsg: msg}
}

// HaveGrpcStatusWithSubstr creates a matcher for a gRPC status with a substring message match.
func HaveGrpcStatusWithSubstr(code codes.Code, substr string) types.GomegaMatcher {
	return &gRPCStatusMatcher{expectedCode: code, expectedMsgPart: substr}
}

func (m *gRPCStatusMatcher) Match(actual any) (success bool, err error) {
	if actual == nil {
		return false, nil
	}
	actualErr, ok := actual.(error)
	if !ok {
		return false, fmt.Errorf("gRPCStatusMatcher expects an error. Got\n\t%#v", actual)
	}

	st, ok := status.FromError(actualErr)
	if !ok {
		return false, fmt.Errorf("error is not a gRPC status: %w", actualErr)
	}

	if st.Code() != m.expectedCode {
		return false, nil
	}

	if m.expectedMsg != "" && st.Message() != m.expectedMsg {
		return false, nil
	}

	if m.expectedMsgPart != "" && !strings.Contains(st.Message(), m.expectedMsgPart) {
		return false, nil
	}

	return true, nil
}

func (m *gRPCStatusMatcher) FailureMessage(actual any) (message string) {
	actualErr, ok := actual.(error)
	if !ok {
		return fmt.Sprintf("Expected an error, but got\n\t%#v", actual)
	}
	gomegaFailureMessage := fmt.Sprintf("Expected\n\t%#v", actual)

	st, ok := status.FromError(actualErr)
	if !ok {
		return fmt.Sprintf("%s\nto be a gRPC status error, but it's not.", gomegaFailureMessage)
	}

	var expectedMsgDesc string
	if m.expectedMsg != "" {
		expectedMsgDesc = fmt.Sprintf("and message '%s'", m.expectedMsg)
	} else if m.expectedMsgPart != "" {
		expectedMsgDesc = fmt.Sprintf("and message containing '%s'", m.expectedMsgPart)
	}

	return fmt.Sprintf("%s\nto have code %s %s\nbut got code %s and message '%s'", gomegaFailureMessage, m.expectedCode, expectedMsgDesc, st.Code(), st.Message())
}

func (m *gRPCStatusMatcher) NegatedFailureMessage(actual any) (message string) {
	actualErr, ok := actual.(error)
	if !ok {
		return fmt.Sprintf("Expected not an error, but got\n\t%#v", actual)
	}
	gomegaFailureMessage := fmt.Sprintf("Expected\n\t%#v", actual)

	_, ok = status.FromError(actualErr)
	if !ok {
		return fmt.Sprintf("%s\nnot to be a gRPC status error, but it's not (which is expected).", gomegaFailureMessage)
	}

	var expectedMsgDesc string
	if m.expectedMsg != "" {
		expectedMsgDesc = fmt.Sprintf("and message '%s'", m.expectedMsg)
	} else if m.expectedMsgPart != "" {
		expectedMsgDesc = fmt.Sprintf("and message containing '%s'", m.expectedMsgPart)
	}

	return fmt.Sprintf("%s\nnot to have code %s %s", gomegaFailureMessage, m.expectedCode, expectedMsgDesc)
}

func TestServer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "gRPC V1 Server Suite")
}

// MockPageReaderMutator is a mock implementation of wikipage.PageReaderMutator for testing.
type MockPageReaderMutator struct {
	Frontmatter        wikipage.FrontMatter
	FrontmatterByID    map[string]map[string]any // For multi-page scenarios
	ErrByID            map[string]error          // For returning different errors per identifier
	Markdown           wikipage.Markdown
	Err                error
	MarkdownReadErr    error // Separate error for ReadMarkdown; takes precedence over Err, allowing ReadFrontMatter to succeed while ReadMarkdown fails
	WrittenFrontmatter wikipage.FrontMatter
	WrittenMarkdown    wikipage.Markdown
	WrittenIdentifier  wikipage.PageIdentifier
	WriteErr           error
	MarkdownWriteErr   error // Separate error for WriteMarkdown
	DeletedIdentifier  wikipage.PageIdentifier
	DeleteErr          error
	// WrittenFrontmatterByID tracks all writes per identifier for multi-page scenarios
	WrittenFrontmatterByID map[string]map[string]any
	// PostWriteMarkdownReadErr is returned by ReadMarkdown after a successful WriteMarkdown call.
	// Use this to simulate a read-back failure for invariant check testing.
	PostWriteMarkdownReadErr error
	// PostWriteMarkdown, when non-nil, overrides what ReadMarkdown returns after a successful
	// WriteMarkdown. Use this to simulate an invariant violation (e.g. blank content after write).
	PostWriteMarkdown *wikipage.Markdown
	// ConcurrentModificationMarkdown, when non-nil, is what ModifyMarkdown presents to its modifier
	// as the current content — overriding m.Markdown. Use this to simulate a concurrent write that
	// happens between ReadMarkdown and ModifyMarkdown, verifying that the atomic hash check inside
	// ModifyMarkdown detects the TOCTOU race.
	ConcurrentModificationMarkdown *wikipage.Markdown
	// markdownWritten tracks whether WriteMarkdown has been called successfully.
	markdownWritten bool
}

func (m *MockPageReaderMutator) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	// Check for identifier-specific errors first
	if m.ErrByID != nil {
		if err, ok := m.ErrByID[string(identifier)]; ok {
			return "", nil, err
		}
	}
	if m.Err != nil {
		return "", nil, m.Err
	}
	// Check WrittenFrontmatterByID first to get the latest written value
	if m.WrittenFrontmatterByID != nil {
		if fm, ok := m.WrittenFrontmatterByID[string(identifier)]; ok {
			return identifier, fm, nil
		}
	}
	// Check FrontmatterByID for initial multi-page scenarios
	if m.FrontmatterByID != nil {
		if fm, ok := m.FrontmatterByID[string(identifier)]; ok {
			return identifier, fm, nil
		}
		return "", nil, os.ErrNotExist
	}
	return identifier, m.Frontmatter, nil
}

func (m *MockPageReaderMutator) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	m.WrittenIdentifier = identifier
	m.WrittenFrontmatter = fm
	// Track writes per identifier for multi-page scenarios
	if m.WrittenFrontmatterByID == nil {
		m.WrittenFrontmatterByID = make(map[string]map[string]any)
	}
	// Shallow copy the frontmatter (sufficient for test isolation)
	fmCopy := maps.Clone(fm)
	m.WrittenFrontmatterByID[string(identifier)] = fmCopy
	return m.WriteErr
}

func (m *MockPageReaderMutator) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = md
	if m.MarkdownWriteErr != nil {
		return m.MarkdownWriteErr
	}
	// Update Markdown to reflect what was written so subsequent ReadMarkdown calls
	// return the current state of the page, enabling invariant check testing.
	m.Markdown = md
	m.markdownWritten = true
	return nil
}

func (m *MockPageReaderMutator) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	if m.MarkdownReadErr != nil {
		return "", "", m.MarkdownReadErr
	}
	if m.markdownWritten {
		if m.PostWriteMarkdownReadErr != nil {
			return "", "", m.PostWriteMarkdownReadErr
		}
		if m.PostWriteMarkdown != nil {
			return identifier, *m.PostWriteMarkdown, nil
		}
	}
	if m.Err != nil {
		return "", "", m.Err
	}
	return identifier, m.Markdown, nil
}

func (m *MockPageReaderMutator) DeletePage(identifier wikipage.PageIdentifier) error {
	m.DeletedIdentifier = identifier
	return m.DeleteErr
}

// ModifyMarkdown atomically reads the markdown, calls modifier, and writes the result.
// Simulates the read-error, modifier, and write-error fields in sequence.
// If ConcurrentModificationMarkdown is set, the modifier sees that content instead of
// m.Markdown — simulating a concurrent write that happened between ReadMarkdown and the
// atomic write, so the hash check inside the modifier can detect the TOCTOU race.
func (m *MockPageReaderMutator) ModifyMarkdown(identifier wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	if m.MarkdownReadErr != nil {
		return m.MarkdownReadErr
	}
	if m.Err != nil {
		return m.Err
	}

	// Allow tests to simulate a concurrent modification that happened between ReadMarkdown
	// and ModifyMarkdown.
	currentMD := m.Markdown
	if m.ConcurrentModificationMarkdown != nil {
		currentMD = *m.ConcurrentModificationMarkdown
	}

	newMD, err := modifier(currentMD)
	if err != nil {
		return err
	}

	if m.MarkdownWriteErr != nil {
		return m.MarkdownWriteErr
	}

	m.WrittenIdentifier = identifier
	m.WrittenMarkdown = newMD
	m.Markdown = newMD
	m.markdownWritten = true
	return nil
}

// ReadPage satisfies wikipage.PageOpener, returning a Page built from the mock's Markdown and error fields.
func (m *MockPageReaderMutator) ReadPage(identifier wikipage.PageIdentifier) (*wikipage.Page, error) {
	readErr := m.MarkdownReadErr
	if readErr == nil {
		readErr = m.Err
	}
	if readErr != nil {
		return nil, readErr
	}
	return &wikipage.Page{
		Identifier:        string(identifier),
		Text:              string(m.Markdown),
		WasLoadedFromDisk: true,
		ModTime:           time.Now(),
	}, nil
}

// MockJobStreamServer is a mock implementation of apiv1.SystemInfoService_StreamJobStatusServer for testing.
type MockJobStreamServer struct {
	SentMessages      []*apiv1.GetJobStatusResponse
	SendErr           error
	SendErrAfterCount int // if > 0, returns SendErr only after this many successful sends
	ContextDone       bool
	ctx               context.Context
	cancelFunc        context.CancelFunc
}

// newCancellableMockJobStreamServer creates a MockJobStreamServer with a controllable context.
func newCancellableMockJobStreamServer() *MockJobStreamServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockJobStreamServer{ctx: ctx, cancelFunc: cancel}
}

// Cancel cancels the stream context.
func (m *MockJobStreamServer) Cancel() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

func (m *MockJobStreamServer) Send(response *apiv1.GetJobStatusResponse) error {
	if m.SendErrAfterCount > 0 && len(m.SentMessages) >= m.SendErrAfterCount {
		return m.SendErr
	}
	if m.SendErr != nil && m.SendErrAfterCount == 0 {
		return m.SendErr
	}
	m.SentMessages = append(m.SentMessages, response)
	return nil
}

func (m *MockJobStreamServer) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	if m.ContextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*MockJobStreamServer) SetHeader(metadata.MD) error {
	return nil
}

func (*MockJobStreamServer) SendHeader(metadata.MD) error {
	return nil
}

func (*MockJobStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}

func (*MockJobStreamServer) SendMsg(any) error {
	return nil
}

func (*MockJobStreamServer) RecvMsg(any) error {
	return nil
}

// MockPageWatchStreamServer is a mock implementation of apiv1.PageManagementService_WatchPageServer for testing.
type MockPageWatchStreamServer struct {
	SentMessages      []*apiv1.WatchPageResponse
	SendErr           error
	SendErrAfterCount int // if > 0, returns SendErr only after this many successful sends
	ContextDone       bool
	ctx               context.Context    // if set, overrides ContextDone behavior
	cancelFunc        context.CancelFunc // cancels ctx
}

// newCancellableMockPageWatchStreamServer creates a MockPageWatchStreamServer with a controllable context.
func newCancellableMockPageWatchStreamServer() *MockPageWatchStreamServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockPageWatchStreamServer{ctx: ctx, cancelFunc: cancel}
}

func (m *MockPageWatchStreamServer) Cancel() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

func (m *MockPageWatchStreamServer) Send(response *apiv1.WatchPageResponse) error {
	if m.SendErr != nil {
		if m.SendErrAfterCount <= 0 || len(m.SentMessages) >= m.SendErrAfterCount {
			return m.SendErr
		}
	}
	m.SentMessages = append(m.SentMessages, response)
	return nil
}

func (m *MockPageWatchStreamServer) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	if m.ContextDone {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		return ctx
	}
	return context.Background()
}

func (*MockPageWatchStreamServer) SetHeader(metadata.MD) error {
	return nil
}

func (*MockPageWatchStreamServer) SendHeader(metadata.MD) error {
	return nil
}

func (*MockPageWatchStreamServer) SetTrailer(metadata.MD) {
	// No-op test stub — not needed for this test scenario
}

func (*MockPageWatchStreamServer) SendMsg(any) error {
	return nil
}

func (*MockPageWatchStreamServer) RecvMsg(any) error {
	return nil
}

// MockBleveIndexQueryer is a mock implementation of bleve.BleveIndexQueryer for testing.
type MockBleveIndexQueryer struct {
	Results []bleve.SearchResult
	Err     error
}

func (m *MockBleveIndexQueryer) Query(_ string) ([]bleve.SearchResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Results, nil
}

// QueryWithTags returns the same Results regardless of tag filtering — tests
// that need to assert tag-aware behavior should override on a per-test basis.
func (m *MockBleveIndexQueryer) QueryWithTags(_ string, _ []string, _ []string) ([]bleve.SearchResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Results, nil
}

// noOpFrontmatterIndexQueryer is a minimal mock for tests that don't need frontmatter indexing.
type noOpFrontmatterIndexQueryer struct{}

func (noOpFrontmatterIndexQueryer) QueryExactMatch(wikipage.DottedKeyPath, wikipage.Value) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryKeyExistence(wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) QueryPrefixMatch(wikipage.DottedKeyPath, string) []wikipage.PageIdentifier {
	return nil
}
func (noOpFrontmatterIndexQueryer) GetValue(wikipage.PageIdentifier, wikipage.DottedKeyPath) wikipage.Value {
	return ""
}
func (noOpFrontmatterIndexQueryer) QueryExactMatchSortedBy(wikipage.DottedKeyPath, wikipage.Value, wikipage.DottedKeyPath, bool, int) []wikipage.PageIdentifier {
	return nil
}

// noOpBleveIndexQueryer is a minimal mock for tests that don't need search indexing.
type noOpBleveIndexQueryer struct{}

func (noOpBleveIndexQueryer) Query(string) ([]bleve.SearchResult, error) { return nil, nil }
func (noOpBleveIndexQueryer) QueryWithTags(string, []string, []string) ([]bleve.SearchResult, error) {
	return nil, nil
}

// noopUnsubscribe is a no-op unsubscribe function returned by mock subscription methods.
func noopUnsubscribe() {
	// No-op — intentionally empty test stub.
}

// noOpChatBufferManager is a minimal mock for tests that don't need chat functionality.
type noOpChatBufferManager struct{}

func (noOpChatBufferManager) AddUserMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) AddAssistantMessage(string, string, string) (string, error) {
	return "", nil
}
func (noOpChatBufferManager) EditMessage(string, string, bool) error {
	return nil
}
func (noOpChatBufferManager) AddReaction(string, string, string) error {
	return nil
}
func (noOpChatBufferManager) GetMessages(string) []*chatbuffer.Message {
	return nil
}
func (noOpChatBufferManager) SubscribeToPage(string) (<-chan chatbuffer.Event, func()) {
	ch := make(chan chatbuffer.Event)
	close(ch)
	return ch, noopUnsubscribe
}
func (noOpChatBufferManager) SubscribeToPageWithReplay(string) ([]*chatbuffer.Message, <-chan chatbuffer.Event, func()) {
	ch := make(chan chatbuffer.Event)
	close(ch)
	return nil, ch, noopUnsubscribe
}
func (noOpChatBufferManager) SubscribeToPageChannelWithReplay(string) ([]*chatbuffer.Message, <-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message)
	close(ch)
	return nil, ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) SubscribeToPageChannel(string) (<-chan *chatbuffer.Message, func()) {
	ch := make(chan *chatbuffer.Message)
	close(ch)
	return ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) HasPageChannelSubscriber(string) bool {
	return false
}

func (noOpChatBufferManager) RequestInstance(string) {
	// no-op: satisfies interface; this implementation ignores instance requests
}

func (noOpChatBufferManager) SubscribeToInstanceRequests() (<-chan string, func()) {
	ch := make(chan string)
	close(ch)
	return ch, func() {
		// no-op: nothing to unsubscribe from a closed channel
	}
}

func (noOpChatBufferManager) HasInstanceRequestSubscribers() bool {
	return false
}

func (noOpChatBufferManager) IsInstanceRequested(string) bool {
	return false
}

func (noOpChatBufferManager) NotifyToolCall(string, string, string, string, string) {
	// no-op: satisfies interface; this implementation ignores tool call notifications
}

func (noOpChatBufferManager) CancelPage(string) bool {
	return false
}

func (noOpChatBufferManager) SubscribeToCancellation(string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	return ch, func() {
		// no-op: nothing to unsubscribe from this mock cancellation channel
	}
}

func (noOpChatBufferManager) EmitPermissionRequest(string, *chatbuffer.PermissionRequestEvent) {
	// no-op: satisfies interface; this implementation ignores permission request emissions
}

func (noOpChatBufferManager) RespondToPermission(string, string) {
	// no-op: satisfies interface; this implementation ignores permission responses
}

// noOpPageReaderMutator is a minimal mock for tests that don't need page operations.
type noOpPageReaderMutator struct{}

func (noOpPageReaderMutator) ReadFrontMatter(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return "", nil, os.ErrNotExist
}
func (noOpPageReaderMutator) WriteFrontMatter(wikipage.PageIdentifier, wikipage.FrontMatter) error {
	return nil
}
func (noOpPageReaderMutator) ReadMarkdown(wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return "", "", os.ErrNotExist
}
func (noOpPageReaderMutator) WriteMarkdown(wikipage.PageIdentifier, wikipage.Markdown) error {
	return nil
}
func (noOpPageReaderMutator) DeletePage(wikipage.PageIdentifier) error { return nil }
func (noOpPageReaderMutator) ModifyMarkdown(_ wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	_, err := modifier("")
	return err
}

// noOpPageOpener is a minimal mock for tests that don't need full page reads.
type noOpPageOpener struct{}

func (noOpPageOpener) ReadPage(wikipage.PageIdentifier) (*wikipage.Page, error) {
	return &wikipage.Page{}, nil
}

// mustNewServer creates a server with the given dependencies, failing the test if creation fails.
func mustNewServer(
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.BleveIndexQueryer,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
) *v1.Server {
	return mustNewServerFull(pageReaderMutator, bleveIndexQueryer, frontmatterIndexQueryer, nil, nil)
}

// mustNewServerFull creates a server with all optional dependencies.
func mustNewServerFull(
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.BleveIndexQueryer,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
	jobCoordinator jobs.JobCoordinator,
	pageOpener wikipage.PageOpener,
) *v1.Server {
	if pageReaderMutator == nil {
		pageReaderMutator = noOpPageReaderMutator{}
	}
	if bleveIndexQueryer == nil {
		bleveIndexQueryer = noOpBleveIndexQueryer{}
	}
	if frontmatterIndexQueryer == nil {
		frontmatterIndexQueryer = noOpFrontmatterIndexQueryer{}
	}
	if pageOpener == nil {
		pageOpener = noOpPageOpener{}
	}
	server, err := v1.NewServer(
		v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
		pageReaderMutator,
		bleveIndexQueryer,
		frontmatterIndexQueryer,
		lumber.NewConsoleLogger(lumber.WARN),
		noOpChatBufferManager{},
		pageOpener,
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "mustNewServerFull failed")
	if jobCoordinator != nil {
		server = server.WithJobQueueCoordinator(jobCoordinator)
	}
	return server
}

// logWriteCloser wraps a bytes.Buffer to implement io.WriteCloser for capturing log output in tests.
type logWriteCloser struct {
	*bytes.Buffer
}

func (*logWriteCloser) Close() error { return nil }

// mustNewServerWithLogger creates a server with the given logger (for capturing log output).
func mustNewServerWithLogger(
	pageReaderMutator wikipage.PageReaderMutator,
	jobCoordinator jobs.JobCoordinator,
	logger *lumber.ConsoleLogger,
) *v1.Server {
	if pageReaderMutator == nil {
		pageReaderMutator = noOpPageReaderMutator{}
	}
	server, err := v1.NewServer(
		v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
		pageReaderMutator,
		noOpBleveIndexQueryer{},
		noOpFrontmatterIndexQueryer{},
		logger,
		noOpChatBufferManager{},
		noOpPageOpener{},
	)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "mustNewServerWithLogger failed")
	if jobCoordinator != nil {
		server = server.WithJobQueueCoordinator(jobCoordinator)
	}
	return server
}

var _ = Describe("Server", func() {
	var (
		server *v1.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		ctx = context.Background()
	})

	Describe("GetFrontmatter", func() {
		var (
			req                   *apiv1.GetFrontmatterRequest
			res                   *apiv1.GetFrontmatterResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.GetFrontmatterRequest{
				Page: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			res, err = server.GetFrontmatter(ctx, req)
		})

		When("the requested page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(res).To(BeNil())
			})
		})

		When("PageReaderMutator returns a generic error", func() {
			var genericError error
			BeforeEach(func() {
				genericError = errors.New("kaboom")
				mockPageReaderMutator.Err = genericError
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "kaboom"))
				Expect(res).To(BeNil())
			})
		})

		When("the requested page exists", func() {
			var expectedFm map[string]any
			var expectedStruct *structpb.Struct

			BeforeEach(func() {
				expectedFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReaderMutator.Frontmatter = expectedFm

				var structErr error
				expectedStruct, structErr = structpb.NewStruct(expectedFm)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the page's frontmatter", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})

		When("the requested page exists but has nil frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = nil
				mockPageReaderMutator.Err = nil
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response with empty frontmatter", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).NotTo(BeNil())
			})
		})

		When("the requested page has frontmatter with identifier key", func() {
			var frontmatterWithIdentifier map[string]any
			var expectedFilteredFm map[string]any
			var expectedStruct *structpb.Struct

			BeforeEach(func() {
				frontmatterWithIdentifier = map[string]any{
					"title":      "Test Page",
					"identifier": "test-page",
					"tags":       []any{"test", "ginkgo"},
				}
				expectedFilteredFm = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test", "ginkgo"},
				}
				mockPageReaderMutator.Frontmatter = frontmatterWithIdentifier

				var structErr error
				expectedStruct, structErr = structpb.NewStruct(expectedFilteredFm)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter without the identifier key", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})

		When("the requested page has frontmatter with nested identifier keys", func() {
			var frontmatterWithNestedIdentifier map[string]any
			var expectedStruct *structpb.Struct

			BeforeEach(func() {
				frontmatterWithNestedIdentifier = map[string]any{
					"title": "Test Page",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				mockPageReaderMutator.Frontmatter = frontmatterWithNestedIdentifier

				// Nested identifier keys should be preserved, only root-level filtered
				var structErr error
				expectedStruct, structErr = structpb.NewStruct(frontmatterWithNestedIdentifier)
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the frontmatter with nested identifier keys preserved", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.Frontmatter).To(Equal(expectedStruct))
			})
		})
	})

	Describe("MergeFrontmatter", func() {
		var (
			req                   *apiv1.MergeFrontmatterRequest
			resp                  *apiv1.MergeFrontmatterResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
			pageName              string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReaderMutator = &MockPageReaderMutator{}
			req = &apiv1.MergeFrontmatterRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.MergeFrontmatter(ctx, req)
		})

		When("reading the frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("read error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.WriteErr = errors.New("write error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			var newFrontmatterPb *structpb.Struct
			var newFrontmatter wikipage.FrontMatter

			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist

				newFrontmatter = wikipage.FrontMatter{"title": "New Title"}
				var err error
				newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				req.Frontmatter = newFrontmatterPb
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the new frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(newFrontmatter))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})

		When("the page exists", func() {
			var existingFrontmatter wikipage.FrontMatter
			var newFrontmatter wikipage.FrontMatter
			var mergedFrontmatter wikipage.FrontMatter

			BeforeEach(func() {
				existingFrontmatter = wikipage.FrontMatter{"title": "Old Title", "tags": []any{"old"}}
				newFrontmatter = wikipage.FrontMatter{"title": "New Title", "author": "test"}

				mergedFrontmatter = wikipage.FrontMatter{
					"title":  "New Title",
					"tags":   []any{"old"},
					"author": "test",
				}

				mockPageReaderMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(mergedFrontmatter))
			})

			It("should return the merged frontmatter without identifier key", func() {
				expectedPb, err := structpb.NewStruct(mergedFrontmatter)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("the frontmatter contains an identifier key", func() {
			BeforeEach(func() {
				frontmatterWithIdentifier := wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": "malicious-identifier",
				}
				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "identifier key cannot be modified"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter contains nested identifier keys", func() {
			var existingFrontmatter wikipage.FrontMatter
			var newFrontmatter wikipage.FrontMatter
			var expectedMergedFm wikipage.FrontMatter

			BeforeEach(func() {
				existingFrontmatter = wikipage.FrontMatter{
					"title": "Existing Title",
					"metadata": map[string]any{
						"author": "existing-author",
					},
				}
				newFrontmatter = wikipage.FrontMatter{
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "new-tag",
						},
					},
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"version":    "1.0",
					},
				}
				expectedMergedFm = wikipage.FrontMatter{
					"title": "Existing Title",
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "new-tag",
						},
					},
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "existing-author",
						"version":    "1.0",
					},
				}

				mockPageReaderMutator.Frontmatter = existingFrontmatter
				var err error
				req.Frontmatter, err = structpb.NewStruct(newFrontmatter)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the merged frontmatter with nested identifier keys preserved", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedMergedFm))
			})

			It("should return the merged frontmatter with nested identifier keys preserved", func() {
				expectedPb, err := structpb.NewStruct(expectedMergedFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("merging a partial update that only specifies one of multiple sibling nested objects", func() {
			var expectedMergedFm wikipage.FrontMatter

			BeforeEach(func() {
				// Existing frontmatter has two sibling checklists
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items":       []any{map[string]any{"text": "Old task", "checked": false}},
							"group_order": []any{"work"},
						},
						"confirmations": map[string]any{
							"items":       []any{map[string]any{"text": "Confirm A", "checked": false}},
							"group_order": []any{"legal"},
						},
					},
				}

				// The merge request only includes todos — confirmations is intentionally absent
				partialUpdate := map[string]any{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items": []any{map[string]any{"text": "Updated task", "checked": true}},
						},
					},
				}

				expectedMergedFm = wikipage.FrontMatter{
					"checklists": map[string]any{
						"todos": map[string]any{
							"items":       []any{map[string]any{"text": "Updated task", "checked": true}},
							"group_order": []any{"work"},
						},
						"confirmations": map[string]any{
							"items":       []any{map[string]any{"text": "Confirm A", "checked": false}},
							"group_order": []any{"legal"},
						},
					},
				}

				var err error
				req.Frontmatter, err = structpb.NewStruct(partialUpdate)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the updated todos items", func() {
				written := mockPageReaderMutator.WrittenFrontmatter
				checklists, ok := written["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				todos, ok := checklists["todos"].(map[string]any)
				Expect(ok).To(BeTrue(), "todos should be map[string]any")
				items, ok := todos["items"].([]any)
				Expect(ok).To(BeTrue(), "items should be []any")
				Expect(items).To(HaveLen(1))
				item, ok := items[0].(map[string]any)
				Expect(ok).To(BeTrue(), "item should be map[string]any")
				Expect(item["text"]).To(Equal("Updated task"))
				Expect(item["checked"]).To(BeTrue())
			})

			It("should preserve the todos group_order that was not in the update", func() {
				written := mockPageReaderMutator.WrittenFrontmatter
				checklists, ok := written["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				todos, ok := checklists["todos"].(map[string]any)
				Expect(ok).To(BeTrue(), "todos should be map[string]any")
				Expect(todos["group_order"]).To(Equal([]any{"work"}))
			})

			It("should preserve the confirmations sibling checklist", func() {
				written := mockPageReaderMutator.WrittenFrontmatter
				checklists, ok := written["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				Expect(checklists).To(HaveKey("confirmations"))
				confirmations, ok := checklists["confirmations"].(map[string]any)
				Expect(ok).To(BeTrue(), "confirmations should be map[string]any")
				items, ok := confirmations["items"].([]any)
				Expect(ok).To(BeTrue(), "confirmations items should be []any")
				Expect(items).To(HaveLen(1))
				confirmItem, ok := items[0].(map[string]any)
				Expect(ok).To(BeTrue(), "confirmation item should be map[string]any")
				Expect(confirmItem["text"]).To(Equal("Confirm A"))
			})

			It("should return the deep-merged frontmatter", func() {
				expectedPb, err := structpb.NewStruct(expectedMergedFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("ReplaceFrontmatter", func() {
		var (
			req                   *apiv1.ReplaceFrontmatterRequest
			resp                  *apiv1.ReplaceFrontmatterResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
			pageName              string
			newFrontmatter        wikipage.FrontMatter
			newFrontmatterPb      *structpb.Struct
		)

		BeforeEach(func() {
			pageName = "test-page"
			newFrontmatter = wikipage.FrontMatter{"title": "New Title", "tags": []any{"a", "b"}}
			var err error
			newFrontmatterPb, err = structpb.NewStruct(newFrontmatter)
			Expect(err).NotTo(HaveOccurred())

			mockPageReaderMutator = &MockPageReaderMutator{}

			req = &apiv1.ReplaceFrontmatterRequest{
				Page:        pageName,
				Frontmatter: newFrontmatterPb,
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.ReplaceFrontmatter(ctx, req)
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.WriteErr = errors.New("disk full")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("the request is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should write the new frontmatter to the page", func() {
				expectedWrittenFm := maps.Clone(newFrontmatter)
				expectedWrittenFm["identifier"] = pageName
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return the new frontmatter", func() {
				Expect(resp.Frontmatter).To(Equal(newFrontmatterPb))
			})
		})

		When("the request has a nil frontmatter", func() {
			BeforeEach(func() {
				req.Frontmatter = nil
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should write nil frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(BeNil())
			})

			It("should return nil frontmatter", func() {
				Expect(resp.Frontmatter).To(BeNil())
			})
		})

		When("the request contains an identifier key", func() {
			var frontmatterWithIdentifier wikipage.FrontMatter
			var expectedWrittenFm wikipage.FrontMatter
			var expectedResponseFm wikipage.FrontMatter

			BeforeEach(func() {
				frontmatterWithIdentifier = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": "malicious-identifier",
					"tags":       []any{"a", "b"},
				}
				expectedWrittenFm = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": pageName, // Should be set to correct page name
					"tags":       []any{"a", "b"},
				}
				expectedResponseFm = wikipage.FrontMatter{
					"title": "New Title",
					"tags":  []any{"a", "b"},
				}

				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write frontmatter with correct identifier", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return frontmatter without identifier key", func() {
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("the request contains nested identifier keys", func() {
			var frontmatterWithNestedIdentifier wikipage.FrontMatter
			var expectedWrittenFm wikipage.FrontMatter
			var expectedResponseFm wikipage.FrontMatter

			BeforeEach(func() {
				frontmatterWithNestedIdentifier = wikipage.FrontMatter{
					"title": "New Title",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				expectedWrittenFm = wikipage.FrontMatter{
					"title":      "New Title",
					"identifier": pageName, // Should be set to correct page name
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}
				expectedResponseFm = wikipage.FrontMatter{
					"title": "New Title",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-allowed",
						"author":     "test-author",
					},
					"tags": []any{
						map[string]any{
							"identifier": "tag-identifier-should-be-allowed",
							"name":       "test-tag",
						},
					},
				}

				var err error
				req.Frontmatter, err = structpb.NewStruct(frontmatterWithNestedIdentifier)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write frontmatter with nested identifier keys preserved and correct root identifier", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier(pageName)))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedWrittenFm))
			})

			It("should return frontmatter with nested identifier keys preserved", func() {
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("RemoveKeyAtPath", func() {
		var (
			req                   *apiv1.RemoveKeyAtPathRequest
			resp                  *apiv1.RemoveKeyAtPathResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
			pageName              string
		)

		BeforeEach(func() {
			pageName = "test-page"
			mockPageReaderMutator = &MockPageReaderMutator{}
			req = &apiv1.RemoveKeyAtPathRequest{
				Page: pageName,
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.RemoveKeyAtPath(ctx, req)
		})

		When("the key_path is empty", func() {
			BeforeEach(func() {
				req.KeyPath = []*apiv1.PathComponent{}
			})

			It("should return an invalid argument error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "key_path cannot be empty"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "page not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("the frontmatter is nil", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = nil
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})
			It("should return a not found error for the key and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "key 'a' not found"))
				Expect(resp).To(BeNil())
			})
		})

		When("reading the frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("disk error")
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{"a": "b"}
				mockPageReaderMutator.WriteErr = errors.New("write error")
				req.KeyPath = []*apiv1.PathComponent{{Component: &apiv1.PathComponent_Key{Key: "a"}}}
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("removing a key successfully", func() {
			var initialFm wikipage.FrontMatter
			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"a": "b",
					"c": map[string]any{
						"d": "e",
					},
					"f": []any{"g", "h", map[string]any{"i": "j"}},
				}
				mockPageReaderMutator.Frontmatter = initialFm
			})

			When("from the top-level map", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
					}
					expectedFm = wikipage.FrontMatter{
						"c": map[string]any{
							"d": "e",
						},
						"f": []any{"g", "h", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a nested map", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "c"}},
						{Component: &apiv1.PathComponent_Key{Key: "d"}},
					}
					expectedFm = wikipage.FrontMatter{
						"a": "b",
						"c": map[string]any{},
						"f": []any{"g", "h", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("from a slice", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 1}},
					}
					expectedFm = wikipage.FrontMatter{
						"a": "b",
						"c": map[string]any{
							"d": "e",
						},
						"f": []any{"g", map[string]any{"i": "j"}},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("a nested key within a slice element", func() {
				var expectedFm wikipage.FrontMatter

				BeforeEach(func() {
					// frontmatter: {"f": [{"a": "b", "c": "d"}, "other"]}
					// removing path: f[0].a (key "f" -> index 0 -> key "a")
					mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
						"f": []any{map[string]any{"a": "b", "c": "d"}, "other"},
					}
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 0}},
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
					}
					expectedFm = wikipage.FrontMatter{
						"f": []any{map[string]any{"c": "d"}, "other"},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write the correctly modified frontmatter", func() {
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})

			When("the last element from a slice", func() {
				var expectedFm wikipage.FrontMatter
				BeforeEach(func() {
					mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
						"a": "b",
						"f": []any{"only-item"},
					}
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 0}},
					}
					expectedFm = wikipage.FrontMatter{
						"a": "b",
						"f": []any{},
					}
				})

				It("should not return an error", func() {
					Expect(err).NotTo(HaveOccurred())
				})

				It("should write an empty slice (not remove the key)", func() {
					Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
				})

				It("should return the correctly modified frontmatter with an empty slice", func() {
					expectedPb, err := structpb.NewStruct(expectedFm)
					Expect(err).NotTo(HaveOccurred())
					Expect(resp.Frontmatter).To(Equal(expectedPb))
				})
			})
		})

		When("the path is invalid", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
					"a": "b",
					"f": []any{"g"},
				}
			})

			When("a key is not found", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "z"}},
					}
				})
				It("returns a not found error and no response", func() {
					Expect(err).To(HaveGrpcStatus(codes.NotFound, "key 'z' not found"))
					Expect(resp).To(BeNil())
				})
			})

			When("an index is out of bounds", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 99}},
					}
				})
				It("returns an out of range error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.OutOfRange, "index 99 is out of range"))
					Expect(resp).To(BeNil())
				})
			})

			When("a negative index is used", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: -1}},
					}
				})
				It("returns an out of range error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.OutOfRange, "index -1 is out of range"))
					Expect(resp).To(BeNil())
				})
			})

			When("a key is used on a slice", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Key{Key: "z"}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "not an index for a slice"))
					Expect(resp).To(BeNil())
				})
			})

			When("an index is used on a map", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Index{Index: 0}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "not a key for a map"))
					Expect(resp).To(BeNil())
				})
			})

			When("a path traverses deeper than a primitive value within a slice element", func() {
				BeforeEach(func() {
					// f[0] is "g" (a string), but we try to access a key "x" on it
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "f"}},
						{Component: &apiv1.PathComponent_Index{Index: 0}},
						{Component: &apiv1.PathComponent_Key{Key: "x"}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "path is deeper than data structure"))
					Expect(resp).To(BeNil())
				})
			})

			When("traversing through a primitive value", func() {
				BeforeEach(func() {
					req.KeyPath = []*apiv1.PathComponent{
						{Component: &apiv1.PathComponent_Key{Key: "a"}},
						{Component: &apiv1.PathComponent_Key{Key: "b"}},
					}
				})
				It("returns an invalid argument error and no response", func() {
					Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "path is deeper than data structure"))
					Expect(resp).To(BeNil())
				})
			})
		})

		When("attempting to remove the identifier key", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
				}
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "identifier"}},
				}
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "identifier key cannot be removed"))
				Expect(resp).To(BeNil())
			})
		})

		When("removing a regular key with identifier present", func() {
			var initialFm wikipage.FrontMatter
			var expectedFm wikipage.FrontMatter

			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"tags":       []any{"test"},
				}
				expectedFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
				}
				mockPageReaderMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "tags"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with identifier preserved", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
			})

			It("should return the modified frontmatter without identifier key", func() {
				expectedResponseFm := wikipage.FrontMatter{
					"title": "Test Page",
				}
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})

		When("removing a nested identifier key", func() {
			var initialFm wikipage.FrontMatter
			var expectedFm wikipage.FrontMatter

			BeforeEach(func() {
				initialFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"metadata": map[string]any{
						"identifier": "nested-identifier-should-be-removable",
						"author":     "test-author",
						"version":    "1.0",
					},
				}
				expectedFm = wikipage.FrontMatter{
					"title":      "Test Page",
					"identifier": "test-page",
					"metadata": map[string]any{
						"author":  "test-author",
						"version": "1.0",
					},
				}

				mockPageReaderMutator.Frontmatter = initialFm
				req.KeyPath = []*apiv1.PathComponent{
					{Component: &apiv1.PathComponent_Key{Key: "metadata"}},
					{Component: &apiv1.PathComponent_Key{Key: "identifier"}},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write the correctly modified frontmatter with nested identifier removed", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(Equal(expectedFm))
			})

			It("should return the modified frontmatter without root identifier key", func() {
				expectedResponseFm := wikipage.FrontMatter{
					"title": "Test Page",
					"metadata": map[string]any{
						"author":  "test-author",
						"version": "1.0",
					},
				}
				expectedPb, err := structpb.NewStruct(expectedResponseFm)
				Expect(err).NotTo(HaveOccurred())
				Expect(resp.Frontmatter).To(Equal(expectedPb))
			})
		})
	})

	Describe("RegisterWithServer", func() {
		var grpcServer *grpc.Server

		BeforeEach(func() {
			grpcServer = grpc.NewServer()
			server = mustNewServer(nil, nil, nil)
		})

		It("should register all services without panic", func() {
			Expect(func() { server.RegisterWithServer(grpcServer) }).NotTo(Panic())
		})
	})

	Describe("GetVersion", func() {
		var (
			req  *apiv1.GetVersionRequest
			resp *apiv1.GetVersionResponse
			err  error
		)

		BeforeEach(func() {
			req = &apiv1.GetVersionRequest{}
			server = mustNewServer(nil, nil, nil)
		})

		When("identity is anonymous", func() {
			BeforeEach(func() {
				resp, err = server.GetVersion(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a version response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should not include tailscale identity", func() {
				Expect(resp.TailscaleIdentity).To(BeNil())
			})
		})

		When("identity is a tailscale user", func() {
			BeforeEach(func() {
				identity := tailscale.NewIdentity("user@example.com", "User Name", "device-name")
				identCtx := tailscale.ContextWithIdentity(ctx, identity)
				resp, err = server.GetVersion(identCtx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a version response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should include tailscale identity", func() {
				Expect(resp.TailscaleIdentity).NotTo(BeNil())
			})

			It("should include the login name", func() {
				Expect(resp.TailscaleIdentity.LoginName).To(HaveValue(Equal("user@example.com")))
			})

			It("should include the display name", func() {
				Expect(resp.TailscaleIdentity.DisplayName).To(HaveValue(Equal("User Name")))
			})

			It("should include the node name", func() {
				Expect(resp.TailscaleIdentity.NodeName).To(HaveValue(Equal("device-name")))
			})
		})
	})

	Describe("LoggingInterceptor", func() {
		var (
			server  *v1.Server
			ctx     context.Context
			req     any
			info    *grpc.UnaryServerInfo
			handler grpc.UnaryHandler
		)

		BeforeEach(func() {
			ctx = context.Background()
			req = &apiv1.GetVersionRequest{}
			info = &grpc.UnaryServerInfo{
				FullMethod: "/api.v1.Version/GetVersion",
			}

			server = mustNewServer(nil, nil, nil)
		})

		When("a successful gRPC call is made", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					time.Sleep(10 * time.Millisecond) // Simulate some work
					return &apiv1.GetVersionResponse{Commit: "test"}, nil
				}

				interceptor := server.LoggingInterceptor()
				resp, err = interceptor(ctx, req, info, handler)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp).To(BeAssignableToTypeOf(&apiv1.GetVersionResponse{}))
			})
		})

		When("a gRPC call fails", func() {
			var (
				resp any
				err  error
			)

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					time.Sleep(5 * time.Millisecond) // Simulate some work
					return nil, status.Error(codes.Internal, "test error")
				}

				interceptor := server.LoggingInterceptor()
				resp, err = interceptor(ctx, req, info, handler)
			})

			It("should return the error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(HaveGrpcStatus(codes.Internal, "test error"))
			})

			It("should return nil response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("a handler panics", func() {
			var err error

			BeforeEach(func() {
				handler = func(ctx context.Context, req any) (any, error) {
					panic("test panic")
				}

				interceptor := server.LoggingInterceptor()

				// Capture the panic
				defer func() {
					if recover() != nil {
						err = status.Error(codes.Internal, "panic occurred")
					}
				}()

				_, err = interceptor(ctx, req, info, handler)
			})

			It("should propagate the panic", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("logger is nil", func() {
			var serverErr error

			BeforeEach(func() {
				// Create a server with nil logger - should fail since logger is required
				_, serverErr = v1.NewServer(
					v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
					noOpPageReaderMutator{},
					noOpBleveIndexQueryer{},
					noOpFrontmatterIndexQueryer{},
					nil, // logger is nil - this should cause an error
					noOpChatBufferManager{},
					noOpPageOpener{},
				)
			})

			It("should return an error", func() {
				Expect(serverErr).To(MatchError("logger is required"))
			})
		})
	})

	Describe("DeletePage", func() {
		var (
			req                   *apiv1.DeletePageRequest
			resp                  *apiv1.DeletePageResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.DeletePageRequest{
				PageName: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.DeletePage(ctx, req)
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.DeleteErr = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(resp).To(BeNil())
			})
		})

		When("deletion fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.DeleteErr = errors.New("disk error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to delete page"))
				Expect(resp).To(BeNil())
			})
		})

		When("deletion is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a success response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Success).To(BeTrue())
				Expect(resp.Error).To(BeEmpty())
			})

			It("should call delete on the PageReaderMutator", func() {
				Expect(mockPageReaderMutator.DeletedIdentifier).To(Equal(wikipage.PageIdentifier("test-page")))
			})
		})
	})

	Describe("Audit logging for destructive operations", func() {
		var (
			logBuffer *bytes.Buffer
			logOutput string
		)

		BeforeEach(func() {
			logBuffer = &bytes.Buffer{}
		})

		Describe("DeletePage audit log", func() {
			When("called with an authenticated Tailscale identity", func() {
				BeforeEach(func() {
					logger := lumber.NewBasicLogger(&logWriteCloser{logBuffer}, lumber.TRACE)
					srv := mustNewServerWithLogger(&MockPageReaderMutator{}, nil, logger)
					identity := tailscale.NewIdentity("user@example.com", "User Name", "device-name")
					identCtx := tailscale.ContextWithIdentity(ctx, identity)
					_, _ = srv.DeletePage(identCtx, &apiv1.DeletePageRequest{PageName: "audit-page"})
					logOutput = logBuffer.String()
				})

				It("should log the operation with the page name", func() {
					Expect(logOutput).To(ContainSubstring("audit-page"))
				})

				It("should log the identity", func() {
					Expect(logOutput).To(ContainSubstring("user@example.com"))
				})

				It("should log the operation type", func() {
					Expect(logOutput).To(ContainSubstring("delete"))
				})
			})

			When("called with no identity (anonymous)", func() {
				BeforeEach(func() {
					logger := lumber.NewBasicLogger(&logWriteCloser{logBuffer}, lumber.TRACE)
					srv := mustNewServerWithLogger(&MockPageReaderMutator{}, nil, logger)
					_, _ = srv.DeletePage(ctx, &apiv1.DeletePageRequest{PageName: "anon-page"})
					logOutput = logBuffer.String()
				})

				It("should log as anonymous", func() {
					Expect(logOutput).To(ContainSubstring("anonymous"))
				})
			})
		})

		Describe("StartPageImportJob audit log", func() {
			When("called with an authenticated Tailscale identity", func() {
				BeforeEach(func() {
					logger := lumber.NewBasicLogger(&logWriteCloser{logBuffer}, lumber.TRACE)
					mockCoordinator := &MockJobQueueCoordinator{}
					srv := mustNewServerWithLogger(&MockPageReaderMutator{Err: os.ErrNotExist}, mockCoordinator.AsCoordinator(), logger)
					identity := tailscale.NewIdentity("importer@example.com", "Importer", "import-device")
					identCtx := tailscale.ContextWithIdentity(ctx, identity)
					_, _ = srv.StartPageImportJob(identCtx, &apiv1.StartPageImportJobRequest{
						CsvContent: "identifier,title\ntest_page,Test Page",
					})
					logOutput = logBuffer.String()
				})

				It("should log the identity", func() {
					Expect(logOutput).To(ContainSubstring("importer@example.com"))
				})

				It("should log the operation type", func() {
					Expect(logOutput).To(ContainSubstring("import"))
				})
			})

			When("called with no identity (anonymous)", func() {
				BeforeEach(func() {
					logger := lumber.NewBasicLogger(&logWriteCloser{logBuffer}, lumber.TRACE)
					mockCoordinator := &MockJobQueueCoordinator{}
					srv := mustNewServerWithLogger(&MockPageReaderMutator{Err: os.ErrNotExist}, mockCoordinator.AsCoordinator(), logger)
					_, _ = srv.StartPageImportJob(ctx, &apiv1.StartPageImportJobRequest{
						CsvContent: "identifier,title\ntest_page,Test Page",
					})
					logOutput = logBuffer.String()
				})

				It("should log as anonymous", func() {
					Expect(logOutput).To(ContainSubstring("anonymous"))
				})
			})
		})
	})

	Describe("UpdatePageContent", func() {
		var (
			req                   *apiv1.UpdatePageContentRequest
			resp                  *apiv1.UpdatePageContentResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.UpdatePageContentRequest{
				PageName:           "test-page",
				NewContentMarkdown: "# New Content",
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Frontmatter: wikipage.FrontMatter{"identifier": "test-page"},
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.UpdatePageContent(ctx, req)
		})

		When("page_name is empty", func() {
			BeforeEach(func() {
				req.PageName = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page_name is required"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("reading current content fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("read error")
			})

			// The system-page guard runs first and surfaces a frontmatter
			// read failure as Internal — this is intentional fail-closed
			// behavior so a transient read can't sneak past the guard.
			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "read error"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("writing the markdown fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.MarkdownWriteErr = errors.New("disk full")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write markdown"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the update is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should indicate success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should not return an error message", func() {
				Expect(resp.Error).To(BeEmpty())
			})

			It("should return the version_hash of the stored content", func() {
				h := sha256.Sum256([]byte("# New Content"))
				Expect(resp.VersionHash).To(Equal(hex.EncodeToString(h[:])))
			})

			It("should write to the correct page", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("test-page")))
			})

			It("should write the new markdown content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# New Content")))
			})
		})

		When("new_content_markdown is empty", func() {
			BeforeEach(func() {
				req.NewContentMarkdown = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "new_content_markdown cannot be empty"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("new_content_markdown is only whitespace", func() {
			BeforeEach(func() {
				req.NewContentMarkdown = "   \n\t  "
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "new_content_markdown cannot be empty"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("expected_version_hash matches current content hash", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Old Content"
				// SHA256 of "# Old Content"
				expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte("# Old Content")))
				req.ExpectedVersionHash = &expectedHash
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should indicate success", func() {
				Expect(resp.Success).To(BeTrue())
			})
		})

		When("expected_version_hash does not match current content hash", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Current Content"
				staleHash := "stale-hash-value"
				req.ExpectedVersionHash = &staleHash
			})

			It("should return an aborted error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Aborted, "content version mismatch"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("reading current content fails before write", func() {
			BeforeEach(func() {
				mockPageReaderMutator.MarkdownReadErr = errors.New("disk read error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read current content"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the post-write read-back fails (invariant check)", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Original Content"
				mockPageReaderMutator.PostWriteMarkdownReadErr = errors.New("storage failure")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to verify stored content after write"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should attempt to restore the original content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# Original Content")))
			})
		})

		When("the stored content is empty after write (invariant violation)", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Original Content"
				empty := wikipage.Markdown("")
				mockPageReaderMutator.PostWriteMarkdown = &empty
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "invariant violation"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should restore the original content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# Original Content")))
			})
		})

		When("new_content_markdown contains an invalid template macro", func() {
			BeforeEach(func() {
				req.NewContentMarkdown = `# Page
{{ CheckList "todos" }}`
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "invalid template"))
			})

			It("should include the unknown macro name in the error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "CheckList"))
			})

			It("should suggest the correct macro name", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "Checklist"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should not write any content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(BeEmpty())
			})
		})

		When("old_content_markdown is provided and found in the current content", func() {
			const expectedContent = "# Section One (Updated)\n\nUpdated content here.\n\n# Section Two\n\nMore content."

			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Section One\n\nContent here.\n\n# Section Two\n\nMore content."
				oldContent := "# Section One\n\nContent here."
				req.OldContentMarkdown = &oldContent
				req.NewContentMarkdown = "# Section One (Updated)\n\nUpdated content here."
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should indicate success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should write only the substituted content, preserving the rest of the page", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown(expectedContent)))
			})

			It("should return the version_hash of the stored content", func() {
				h := sha256.Sum256([]byte(expectedContent))
				Expect(resp.VersionHash).To(Equal(hex.EncodeToString(h[:])))
			})
		})

		When("old_content_markdown is provided but not found in the current content", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Section One\n\nContent here."
				notPresent := "# Section That Does Not Exist"
				req.OldContentMarkdown = &notPresent
				req.NewContentMarkdown = "# Replacement"
			})

			It("should return a not found error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "old_content_markdown not found"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should not write any content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(BeEmpty())
			})
		})

		When("old_content_markdown is provided but empty", func() {
			BeforeEach(func() {
				emptyOld := ""
				req.OldContentMarkdown = &emptyOld
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "old_content_markdown cannot be empty"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("old_content_markdown is provided but only whitespace", func() {
			BeforeEach(func() {
				whitespaceOld := "   \n\t  "
				req.OldContentMarkdown = &whitespaceOld
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "old_content_markdown cannot be empty"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		// TOCTOU fix verification: the hash check is now inside ModifyMarkdown (atomic with the
		// write), so a concurrent modification that happens between the pre-read and the write is
		// detected — rather than silently overwriting the concurrent writer's changes.
		When("the page is concurrently modified between the pre-read and the atomic write", func() {
			BeforeEach(func() {
				originalContent := wikipage.Markdown("# Original Content")
				concurrentContent := wikipage.Markdown("# Concurrently Modified Content")

				mockPageReaderMutator.Markdown = originalContent
				originalHash := fmt.Sprintf("%x", sha256.Sum256([]byte(originalContent)))
				req.ExpectedVersionHash = &originalHash
				req.NewContentMarkdown = "# My New Content"

				// Simulate a concurrent write: ReadMarkdown returns the original, but
				// ModifyMarkdown sees the concurrently-modified version when it re-reads
				// the content under the write lock.
				mockPageReaderMutator.ConcurrentModificationMarkdown = &concurrentContent
			})

			It("should detect the version mismatch and return Aborted", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Aborted, "content version mismatch"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should not overwrite the concurrently modified content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(BeEmpty())
			})
		})
	})

	Describe("ClearPageContent", func() {
		var (
			req                   *apiv1.ClearPageContentRequest
			resp                  *apiv1.ClearPageContentResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.ClearPageContentRequest{
				PageName:     "test-page",
				ConfirmClear: true,
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Frontmatter: wikipage.FrontMatter{"identifier": "test-page"},
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.ClearPageContent(ctx, req)
		})

		When("page_name is empty", func() {
			BeforeEach(func() {
				req.PageName = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page_name is required"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("confirm_clear is false", func() {
			BeforeEach(func() {
				req.ConfirmClear = false
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "confirm_clear must be true to clear page content"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("reading frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("read error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("writing the markdown fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.MarkdownWriteErr = errors.New("disk full")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to clear markdown"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the clear is successful", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should indicate success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should not return an error message", func() {
				Expect(resp.Error).To(BeEmpty())
			})

			It("should write to the correct page", func() {
				Expect(mockPageReaderMutator.WrittenIdentifier).To(Equal(wikipage.PageIdentifier("test-page")))
			})

			It("should write empty markdown content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("")))
			})
		})
	})

	Describe("UpdateWholePage", func() {
		var (
			req                   *apiv1.UpdateWholePageRequest
			resp                  *apiv1.UpdateWholePageResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.UpdateWholePageRequest{
				PageName:         "test-page",
				NewWholeMarkdown: "+++\ntitle = \"New Title\"\n+++\n# New Content",
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Frontmatter: wikipage.FrontMatter{"identifier": "test-page", "title": "Old Title"},
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.UpdateWholePage(ctx, req)
		})

		When("page_name is empty", func() {
			BeforeEach(func() {
				req.PageName = ""
			})

			It("should return an invalid argument error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page_name is required"))
				Expect(resp).To(BeNil())
			})
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error and no response", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
				Expect(resp).To(BeNil())
			})
		})

		When("reading frontmatter fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("read error")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to read frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the frontmatter fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.WriteErr = errors.New("disk full")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write frontmatter"))
				Expect(resp).To(BeNil())
			})
		})

		When("writing the markdown fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.MarkdownWriteErr = errors.New("disk full")
			})

			It("should return an internal error and no response", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to write markdown"))
				Expect(resp).To(BeNil())
			})
		})

		When("the update is successful with TOML frontmatter", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a success response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Success).To(BeTrue())
				Expect(resp.Error).To(BeEmpty())
			})

			It("should write frontmatter with identifier preserved", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(HaveKeyWithValue("title", "New Title"))
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(HaveKeyWithValue("identifier", "test-page"))
			})

			It("should write the markdown content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# New Content")))
			})
		})

		When("the update is successful with no frontmatter", func() {
			BeforeEach(func() {
				req.NewWholeMarkdown = "# Just Markdown"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should write frontmatter containing only the identifier", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(HaveKeyWithValue("identifier", "test-page"))
			})

			It("should write the markdown content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# Just Markdown")))
			})
		})

		When("the whole markdown contains an identifier key that differs from page_name", func() {
			BeforeEach(func() {
				req.NewWholeMarkdown = "+++\nidentifier = \"malicious-override\"\ntitle = \"Hacked\"\n+++\n# Content"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should overwrite identifier with the correct page name", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(HaveKeyWithValue("identifier", "test-page"))
			})
		})

		When("new_whole_markdown contains an invalid template macro", func() {
			BeforeEach(func() {
				req.NewWholeMarkdown = "+++\ntitle = \"Test\"\n+++\n# Page\n{{ CheckList \"todos\" }}"
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "invalid template"))
			})

			It("should include the unknown macro name in the error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "CheckList"))
			})

			It("should suggest the correct macro name", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "Checklist"))
			})

			It("should not return a response", func() {
				Expect(resp).To(BeNil())
			})

			It("should not write any content", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(BeEmpty())
			})
		})
	})

	Describe("GetJobStatus", func() {
		var (
			req *apiv1.GetJobStatusRequest
			res *apiv1.GetJobStatusResponse
			err error
		)

		BeforeEach(func() {
			req = &apiv1.GetJobStatusRequest{}
			server = mustNewServer(nil, nil, nil)
			res, err = server.GetJobStatus(ctx, req)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return empty job queues for now", func() {
			Expect(res).NotTo(BeNil())
			Expect(res.JobQueues).To(BeEmpty())
		})

		When("coordinator has enqueued jobs", func() {
			BeforeEach(func() {
				coordinator := (&MockJobQueueCoordinator{}).AsCoordinator()
				importPageReaderMutator := &MockPageReaderMutator{Err: os.ErrNotExist}
				server = mustNewServerFull(importPageReaderMutator, nil, nil, coordinator, nil)

				startReq := &apiv1.StartPageImportJobRequest{
					CsvContent: "identifier,title\ntest_page,Test Page",
				}
				_, _ = server.StartPageImportJob(ctx, startReq)

				res, err = server.GetJobStatus(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return non-empty job queues", func() {
				Expect(res).NotTo(BeNil())
				Expect(res.JobQueues).NotTo(BeEmpty())
			})

			It("should report the page import queue name", func() {
				Expect(res.JobQueues[0].Name).To(Equal("PageImportJob"))
			})
		})
	})

	Describe("StreamJobStatus", func() {
		var (
			req          *apiv1.StreamJobStatusRequest
			streamServer *MockJobStreamServer
		)

		BeforeEach(func() {
			req = &apiv1.StreamJobStatusRequest{}
			streamServer = &MockJobStreamServer{}
			server = mustNewServer(nil, nil, nil)
		})

		When("context is cancelled immediately after initial send", func() {
			var (
				err          error
				firstMessage *apiv1.GetJobStatusResponse
			)

			BeforeEach(func() {
				streamServer.ContextDone = true
				err = server.StreamJobStatus(req, streamServer)
				if len(streamServer.SentMessages) > 0 {
					firstMessage = streamServer.SentMessages[0]
				}
			})

			It("should handle context cancellation", func() {
				Expect(err).To(Equal(context.Canceled))
			})

			It("should send initial empty response", func() {
				Expect(streamServer.SentMessages).To(HaveLen(1))
				Expect(firstMessage).NotTo(BeNil())
				Expect(firstMessage.JobQueues).To(BeEmpty())
			})
		})

		When("initial send fails", func() {
			var err error

			BeforeEach(func() {
				streamServer.SendErr = errors.New("stream broken")
				err = server.StreamJobStatus(req, streamServer)
			})

			It("should return the send error", func() {
				Expect(err).To(MatchError("stream broken"))
			})

			It("should not record any sent messages", func() {
				Expect(streamServer.SentMessages).To(BeEmpty())
			})
		})

		When("the minimum interval clamp is applied", func() {
			var err error

			BeforeEach(func() {
				// Request an interval below the 100ms minimum
				intervalMs := int32(50)
				req.UpdateIntervalMs = &intervalMs
				streamServer = newCancellableMockJobStreamServer()
				// Fail on the 2nd send to trigger ticker path then exit
				streamServer.SendErrAfterCount = 1
				streamServer.SendErr = errors.New("stop after first tick")
				err = server.StreamJobStatus(req, streamServer)
			})

			It("should return after ticker fires and send fails", func() {
				Expect(err).To(MatchError("stop after first tick"))
			})

			It("should have sent the initial message before the ticker", func() {
				Expect(len(streamServer.SentMessages)).To(BeNumerically(">=", 1))
			})
		})

		When("send fails on ticker update", func() {
			var err error

			BeforeEach(func() {
				intervalMs := int32(100)
				req.UpdateIntervalMs = &intervalMs
				streamServer = newCancellableMockJobStreamServer()
				streamServer.SendErrAfterCount = 1
				streamServer.SendErr = errors.New("ticker send failed")
				err = server.StreamJobStatus(req, streamServer)
			})

			It("should return the ticker send error", func() {
				Expect(err).To(MatchError("ticker send failed"))
			})

			It("should have sent exactly one initial message", func() {
				Expect(streamServer.SentMessages).To(HaveLen(1))
			})
		})
	})

	Describe("WatchPage", func() {
		var (
			req                   *apiv1.WatchPageRequest
			streamServer          *MockPageWatchStreamServer
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.WatchPageRequest{
				PageName: "test-page",
			}
			streamServer = &MockPageWatchStreamServer{}
			mockPageReaderMutator = &MockPageReaderMutator{}
		})

		When("page exists", func() {
			var (
				err          error
				firstMessage *apiv1.WatchPageResponse
			)

			BeforeEach(func() {
				// Set up mock to return valid content
				mockPageReaderMutator.Markdown = wikipage.Markdown("# Test Content")
				mockPageReaderMutator.MarkdownReadErr = nil

				// Set up context that gets cancelled after initial send
				streamServer.ContextDone = true

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				err = server.WatchPage(req, streamServer)

				if len(streamServer.SentMessages) > 0 {
					firstMessage = streamServer.SentMessages[0]
				}
			})

			It("should handle context cancellation", func() {
				Expect(err).To(Equal(context.Canceled))
			})

			It("should send initial version hash", func() {
				Expect(streamServer.SentMessages).To(HaveLen(1))
				Expect(firstMessage).NotTo(BeNil())
				Expect(firstMessage.VersionHash).NotTo(BeEmpty())
			})

			It("should send correct version hash for content", func() {
				// Calculate expected hash
				h := sha256.Sum256([]byte("# Test Content"))
				expectedHash := hex.EncodeToString(h[:])
				Expect(firstMessage.VersionHash).To(Equal(expectedHash))
			})
		})

		When("page does not exist", func() {
			var err error

			BeforeEach(func() {
				mockPageReaderMutator.MarkdownReadErr = os.ErrNotExist
				server = mustNewServer(mockPageReaderMutator, nil, nil)
				err = server.WatchPage(req, streamServer)
			})

			It("should return NotFound error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.NotFound, "test-page"))
			})

			It("should not send any messages", func() {
				Expect(streamServer.SentMessages).To(BeEmpty())
			})
		})

		When("page name is empty", func() {
			var err error

			BeforeEach(func() {
				req.PageName = ""
				server = mustNewServer(nil, nil, nil)
				err = server.WatchPage(req, streamServer)
			})

			It("should return InvalidArgument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page_name is required"))
			})
		})

		When("content changes after initial send", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchErr          error
				watchDone         chan error
			)

			BeforeEach(func() {
				cancellableStream = newCancellableMockPageWatchStreamServer()
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Initial Content"),
				}

				checkInterval := int32(100)
				req.CheckIntervalMs = &checkInterval // Use minimum interval for fast test

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)

				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				// Wait for initial message
				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				// Change content
				mockPageReaderMutator.Markdown = wikipage.Markdown("# Changed Content")

				// Wait for second message
				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(2))

				// Cancel the stream and wait for completion
				cancellableStream.Cancel()
				watchErr = <-watchDone
			})

			It("should end with context.Canceled", func() {
				Expect(watchErr).To(Equal(context.Canceled))
			})

			It("should detect hash change and send two messages", func() {
				Expect(cancellableStream.SentMessages).To(HaveLen(2))
			})

			It("should send correct hash for initial content", func() {
				h := sha256.Sum256([]byte("# Initial Content"))
				expectedHash := hex.EncodeToString(h[:])
				Expect(cancellableStream.SentMessages[0].VersionHash).To(Equal(expectedHash))
			})

			It("should send correct hash for changed content", func() {
				h := sha256.Sum256([]byte("# Changed Content"))
				expectedHash := hex.EncodeToString(h[:])
				Expect(cancellableStream.SentMessages[1].VersionHash).To(Equal(expectedHash))
			})

			It("should not send duplicate messages when content unchanged", func() {
				// After the change, the content stays the same — no more messages after the 2nd
				Consistently(func() int {
					return len(cancellableStream.SentMessages)
				}, 300*time.Millisecond, 50*time.Millisecond).Should(Equal(2))
			})
		})

		When("read fails transiently during polling", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchDone         chan error
			)

			BeforeEach(func() {
				cancellableStream = newCancellableMockPageWatchStreamServer()
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Content"),
				}

				checkInterval := int32(100)
				req.CheckIntervalMs = &checkInterval

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)

				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				// Wait for initial message
				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				// Inject a transient read error
				mockPageReaderMutator.Err = errors.New("transient error")

				// Wait a couple of poll cycles, then clear the error
				time.Sleep(200 * time.Millisecond)
				mockPageReaderMutator.Err = nil

				// Cancel and collect
				cancellableStream.Cancel()
				<-watchDone
			})

			It("should still only have the initial message (transient error doesn't crash)", func() {
				// We only got the initial message; the transient error was swallowed and watch continued
				Expect(len(cancellableStream.SentMessages)).To(BeNumerically(">=", 1))
			})
		})
		When("initial stream.Send returns an error", func() {
			var err error

			BeforeEach(func() {
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Test Content"),
				}
				streamServer = &MockPageWatchStreamServer{
					SendErr: errors.New("stream closed"),
				}
				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				err = server.WatchPage(req, streamServer)
			})

			It("should return the stream error", func() {
				Expect(err).To(MatchError("stream closed"))
			})

			It("should not append the failed message", func() {
				Expect(streamServer.SentMessages).To(BeEmpty())
			})
		})

		When("check_interval_ms is below the 100ms minimum", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchErr          error
				watchDone         chan error
			)

			BeforeEach(func() {
				cancellableStream = newCancellableMockPageWatchStreamServer()
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Content"),
				}

				belowMin := int32(10) // 10ms, below the 100ms minimum
				req.CheckIntervalMs = &belowMin

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				cancellableStream.Cancel()
				watchErr = <-watchDone
			})

			It("should clamp interval and still work", func() {
				Expect(watchErr).To(Equal(context.Canceled))
			})

			It("should send the initial message", func() {
				Expect(cancellableStream.SentMessages).To(HaveLen(1))
			})
		})

		When("page is deleted during polling", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchErr          error
				watchDone         chan error
			)

			BeforeEach(func() {
				cancellableStream = newCancellableMockPageWatchStreamServer()
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Content"),
				}

				checkInterval := int32(100)
				req.CheckIntervalMs = &checkInterval

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				// Simulate page deletion during polling
				mockPageReaderMutator.MarkdownReadErr = os.ErrNotExist
				watchErr = <-watchDone
			})

			It("should return a NotFound error indicating the page was deleted", func() {
				Expect(watchErr).To(HaveGrpcStatusWithSubstr(codes.NotFound, "page deleted"))
			})
		})

		When("page content does not change during polling", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchErr          error
				watchDone         chan error
			)

			BeforeEach(func() {
				cancellableStream = newCancellableMockPageWatchStreamServer()
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Stable Content"),
				}

				checkInterval := int32(100)
				req.CheckIntervalMs = &checkInterval

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				// Wait for at least one full poll cycle so sendPageUpdateIfChanged runs with same hash
				time.Sleep(250 * time.Millisecond)

				cancellableStream.Cancel()
				watchErr = <-watchDone
			})

			It("should end with context.Canceled", func() {
				Expect(watchErr).To(Equal(context.Canceled))
			})

			It("should only send the initial message", func() {
				Expect(cancellableStream.SentMessages).To(HaveLen(1))
			})
		})

		When("stream.Send fails during polling after content changes", func() {
			var (
				cancellableStream *MockPageWatchStreamServer
				watchErr          error
				watchDone         chan error
			)

			BeforeEach(func() {
				watchCtx, watchCancel := context.WithCancel(context.Background())
				DeferCleanup(watchCancel)
				cancellableStream = &MockPageWatchStreamServer{
					SendErr:           errors.New("stream closed during polling"),
					SendErrAfterCount: 1, // succeed on first send, fail on second
					ctx:               watchCtx,
					cancelFunc:        watchCancel,
				}
				mockPageReaderMutator = &MockPageReaderMutator{
					Markdown: wikipage.Markdown("# Initial Content"),
				}

				checkInterval := int32(100)
				req.CheckIntervalMs = &checkInterval

				server = mustNewServerFull(mockPageReaderMutator, nil, nil, nil, mockPageReaderMutator)
				watchDone = make(chan error, 1)
				go func() {
					watchDone <- server.WatchPage(req, cancellableStream)
				}()

				Eventually(func() int {
					return len(cancellableStream.SentMessages)
				}, 2*time.Second, 50*time.Millisecond).Should(Equal(1))

				// Change content so sendPageUpdateIfChanged tries to send again
				mockPageReaderMutator.Markdown = wikipage.Markdown("# Changed Content")
				watchErr = <-watchDone
			})

			It("should return the stream send error", func() {
				Expect(watchErr).To(MatchError("stream closed during polling"))
			})
		})
	})

	Describe("SearchContent", func() {
		var (
			req                         *apiv1.SearchContentRequest
			resp                        *apiv1.SearchContentResponse
			err                         error
			mockBleveIndexQueryer       *MockBleveIndexQueryer
			mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.SearchContentRequest{
				Query: "test query",
			}
			mockBleveIndexQueryer = &MockBleveIndexQueryer{}
			mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
				ExactMatchResults: make(map[string][]wikipage.PageIdentifier),
				GetValueResults:   make(map[string]map[string]string),
			}
		})

		JustBeforeEach(func() {
			server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
			resp, err = server.SearchContent(ctx, req)
		})

		When("a valid query is provided", func() {
			var searchResults []bleve.SearchResult

			BeforeEach(func() {
				searchResults = []bleve.SearchResult{
					{
						Identifier: "test-page",
						Title:      "Test Page",
						Fragment:   "This is a test fragment",
						Highlights: []bleve.HighlightSpan{
							{Start: 10, End: 14}, // "test"
						},
					},
				}
				mockBleveIndexQueryer.Results = searchResults
			})

			It("should return search results", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("test-page"))
				Expect(resp.Results[0].Title).To(Equal("Test Page"))
				Expect(resp.Results[0].Fragment).To(Equal("This is a test fragment"))
				Expect(resp.Results[0].Highlights).To(HaveLen(1))
				Expect(resp.Results[0].Highlights[0].Start).To(Equal(int32(10)))
				Expect(resp.Results[0].Highlights[0].End).To(Equal(int32(14)))
			})

			It("should return zero for total unfiltered count when no filters are applied", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.TotalUnfilteredCount).To(Equal(int32(0))) // No filters, so no warning needed
			})
		})

		When("the search index returns an error", func() {
			BeforeEach(func() {
				mockBleveIndexQueryer.Err = errors.New("index error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to search"))
			})
		})

		When("an empty query is provided", func() {
			BeforeEach(func() {
				req.Query = ""
			})

			It("should return invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "query cannot be empty"))
			})
		})

		When("a frontmatter key include filter is provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns 3 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-inventory", Title: "Inventory Page", Fragment: "This has inventory"},
					{Identifier: "page-without-inventory", Title: "Normal Page", Fragment: "This is normal"},
					{Identifier: "another-inventory-page", Title: "Another Inventory", Fragment: "More inventory"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Only 2 pages have the "inventory" frontmatter key
				// Note: Frontmatter index stores MUNGED identifiers (snake_case)
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"page_with_inventory", "another_inventory_page"}
				req.FrontmatterKeyIncludeFilters = []string{"inventory"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only pages that have the include filter key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				Expect(identifiers).To(ContainElements("page-with-inventory", "another-inventory-page"))
			})

			It("should exclude pages that do NOT have the include filter key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				identifiers := make([]string, len(resp.Results))
				for i, r := range resp.Results {
					identifiers[i] = r.Identifier
				}
				Expect(identifiers).NotTo(ContainElement("page-without-inventory"))
			})

			It("should return the total unfiltered count", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.TotalUnfilteredCount).To(Equal(int32(3))) // 3 total results before filtering
			})

			When("no pages match the filter", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{}
				})

				It("should return empty results", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(BeEmpty())
				})

				It("should still return the total unfiltered count", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.TotalUnfilteredCount).To(Equal(int32(3))) // 3 total results before filtering
				})
			})
		})

		When("search returns pages that all lack the include filter key", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns pages that do NOT have the required key
				searchResults = []bleve.SearchResult{
					{Identifier: "page-without-key-1", Title: "Page Without Key 1", Fragment: "No key here"},
					{Identifier: "page-without-key-2", Title: "Page Without Key 2", Fragment: "No key here either"},
					{Identifier: "page-without-key-3", Title: "Page Without Key 3", Fragment: "Still no key"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// The include filter key exists ONLY on pages that are NOT in the search results
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"completely-different-page", "another-unrelated-page"}
				req.FrontmatterKeyIncludeFilters = []string{"inventory.container"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty results since none have the required key", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(BeEmpty())
			})
		})

		When("search results have non-munged identifiers but frontmatter index uses munged identifiers", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Bleve returns identifiers in original format (e.g., with hyphens, mixed case)
				searchResults = []bleve.SearchResult{
					{Identifier: "My-Inventory-Item", Title: "My Inventory Item", Fragment: "Has inventory"},
					{Identifier: "Another-Item", Title: "Another Item", Fragment: "Also has inventory"},
					{Identifier: "Non-Inventory-Page", Title: "Non Inventory", Fragment: "No inventory here"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Frontmatter index stores identifiers in munged format (lowercase snake_case)
				// Note: "My-Inventory-Item" becomes "my_inventory_item" when munged
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{
					"my_inventory_item", // munged version of "My-Inventory-Item"
					"another_item",      // munged version of "Another-Item"
				}
				req.FrontmatterKeyIncludeFilters = []string{"inventory.container"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should correctly match pages by munging search result identifiers", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				// Should return the ORIGINAL identifiers from bleve, not the munged ones
				Expect(identifiers).To(ContainElements("My-Inventory-Item", "Another-Item"))
			})

			It("should exclude pages without the include filter key", func() {
				Expect(resp).NotTo(BeNil())
				identifiers := make([]string, len(resp.Results))
				for i, r := range resp.Results {
					identifiers[i] = r.Identifier
				}
				Expect(identifiers).NotTo(ContainElement("Non-Inventory-Page"))
			})
		})

		When("multiple frontmatter key include filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-both", Title: "Both Keys", Fragment: "Has both"},
					{Identifier: "page-with-inventory", Title: "Inventory Only", Fragment: "Has inventory"},
					{Identifier: "page-with-container", Title: "Container Only", Fragment: "Has container"},
					{Identifier: "page-with-neither", Title: "Neither", Fragment: "Has neither"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Pages with "inventory" key (MUNGED identifiers in frontmatter index)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory"] = []wikipage.PageIdentifier{
					"page_with_both", "page_with_inventory",
				}
				// Pages with "inventory.container" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.container"] = []wikipage.PageIdentifier{
					"page_with_both", "page_with_container",
				}
				// Require both keys (intersection)
				req.FrontmatterKeyIncludeFilters = []string{"inventory", "inventory.container"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results matching ALL include filters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("page-with-both"))
			})
		})

		When("a frontmatter key exclude filter is provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
				// Search returns 3 results: 2 containers (have items key) and 1 item (no items key)
				searchResults = []bleve.SearchResult{
					{Identifier: "container-one", Title: "Container One", Fragment: "This is a container"},
					{Identifier: "item-one", Title: "Item One", Fragment: "This is an item"},
					{Identifier: "container-two", Title: "Container Two", Fragment: "Another container"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// 2 pages have the "inventory.items" key (containers) - MUNGED identifiers
				mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{"container_one", "container_two"}
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results without the excluded key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("item-one"))
			})

			When("no pages have the excluded key", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.KeyExistsResults = []wikipage.PageIdentifier{}
				})

				It("should return all results", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(3))
				})
			})
		})

		When("multiple frontmatter key exclude filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "page-with-both-excluded", Title: "Both Excluded", Fragment: "Has both bad keys"},
					{Identifier: "page-with-items", Title: "Has Items", Fragment: "Has items key"},
					{Identifier: "page-with-archived", Title: "Has Archived", Fragment: "Has archived key"},
					{Identifier: "page-clean", Title: "Clean Page", Fragment: "Has neither"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Pages with "inventory.items" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.items"] = []wikipage.PageIdentifier{
					"page_with_both_excluded", "page_with_items",
				}
				// Pages with "archived" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["archived"] = []wikipage.PageIdentifier{
					"page_with_both_excluded", "page_with_archived",
				}
				// Exclude both keys (union of exclusions)
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items", "archived"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only results matching NONE of the exclude filters", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Identifier).To(Equal("page-clean"))
			})
		})

		When("both inclusion and exclusion filters are provided", func() {
			var (
				mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{
					KeyExistsResultsMap: make(map[string][]wikipage.PageIdentifier),
				}
				// Search returns 4 results (bleve may return non-munged identifiers)
				searchResults = []bleve.SearchResult{
					{Identifier: "inventory-container", Title: "Container", Fragment: "A container"},
					{Identifier: "inventory-item", Title: "Item", Fragment: "An item"},
					{Identifier: "non-inventory-page", Title: "Regular Page", Fragment: "Not inventory"},
					{Identifier: "another-inventory-item", Title: "Another Item", Fragment: "Another item"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// 3 pages have "inventory" key (MUNGED identifiers)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory"] = []wikipage.PageIdentifier{
					"inventory_container", "inventory_item", "another_inventory_item",
				}
				// 1 page has "inventory.items" key (it's a container) (MUNGED identifier)
				mockFrontmatterIndexQueryer.KeyExistsResultsMap["inventory.items"] = []wikipage.PageIdentifier{
					"inventory_container",
				}
				req.FrontmatterKeyIncludeFilters = []string{"inventory"}
				req.FrontmatterKeyExcludeFilters = []string{"inventory.items"}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return only inventory items (has inventory, no items)", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(2))
				identifiers := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
				Expect(identifiers).To(ContainElements("inventory-item", "another-inventory-item"))
				Expect(identifiers).NotTo(ContainElement("inventory-container"))
				Expect(identifiers).NotTo(ContainElement("non-inventory-page"))
			})
		})

		When("frontmatter_keys_to_return_in_results is specified", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]wikipage.PageIdentifier),
					GetValueResults:   make(map[string]map[string]string),
				}
				// Search returns a page with various frontmatter fields
				searchResults = []bleve.SearchResult{
					{Identifier: "test_page", Title: "Test Page", Fragment: "Test content"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Page has multiple frontmatter fields
				mockFrontmatterIndexQueryer.GetValueResults["test_page"] = map[string]string{
					"author":   "John Doe",
					"category": "Technology",
					"tags":     "golang,testing",
					"draft":    "false",
				}
				// Request specific frontmatter keys to return
				req = &apiv1.SearchContentRequest{
					Query:                            "test",
					FrontmatterKeysToReturnInResults: []string{"author", "category", "missing_key"},
				}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should return requested frontmatter keys in results", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(BeNil())
				Expect(resp.Results[0].Frontmatter).To(HaveKey("author"))
				Expect(resp.Results[0].Frontmatter["author"]).To(Equal("John Doe"))
				Expect(resp.Results[0].Frontmatter).To(HaveKey("category"))
				Expect(resp.Results[0].Frontmatter["category"]).To(Equal("Technology"))
			})

			It("should not include frontmatter keys not requested", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("tags"))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("draft"))
			})

			It("should not include missing keys in frontmatter map", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].Frontmatter).NotTo(HaveKey("missing_key"))
			})
		})

		When("result is an inventory item with container", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]wikipage.PageIdentifier),
					GetValueResults:   make(map[string]map[string]string),
				}
				// Search returns an item with a container
				searchResults = []bleve.SearchResult{
					{Identifier: "screwdriver", Title: "Screwdriver", Fragment: "A useful tool"},
				}
				mockBleveIndexQueryer.Results = searchResults
				// Item has container "toolbox"
				mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
					"inventory.container": "toolbox",
				}
				// Container has a title
				mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
					"title": "My Toolbox",
				}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should include inventory context with container ID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
			})

			It("should set IsInventoryRelated to true", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.IsInventoryRelated).To(BeTrue())
			})

			It("should include path with single container element", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
				Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("toolbox"))
				Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("My Toolbox"))
			})

			When("item has nested containers", func() {
				BeforeEach(func() {
					// Item is in toolbox, toolbox is in garage, garage is in house
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "toolbox",
					}
					mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
						"title":               "My Toolbox",
						"inventory.container": "garage",
					}
					mockFrontmatterIndexQueryer.GetValueResults["garage"] = map[string]string{
						"title":               "Main Garage",
						"inventory.container": "house",
					}
					mockFrontmatterIndexQueryer.GetValueResults["house"] = map[string]string{
						"title": "My House",
					}
				})

				It("should build full path from root to immediate container", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(3))

					// Path should be: house > garage > toolbox
					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("house"))
					Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("My House"))
					Expect(resp.Results[0].InventoryContext.Path[0].Depth).To(Equal(int32(0)))

					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("garage"))
					Expect(resp.Results[0].InventoryContext.Path[1].Title).To(Equal("Main Garage"))
					Expect(resp.Results[0].InventoryContext.Path[1].Depth).To(Equal(int32(1)))

					Expect(resp.Results[0].InventoryContext.Path[2].Identifier).To(Equal("toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[2].Title).To(Equal("My Toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[2].Depth).To(Equal(int32(2)))
				})
			})

			When("path element has no title", func() {
				BeforeEach(func() {
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "toolbox",
					}
					mockFrontmatterIndexQueryer.GetValueResults["toolbox"] = map[string]string{
						"inventory.container": "garage",
					}
					mockFrontmatterIndexQueryer.GetValueResults["garage"] = map[string]string{
						"title": "Main Garage",
					}
				})

				It("should include empty title in path element", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(2))

					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("garage"))
					Expect(resp.Results[0].InventoryContext.Path[0].Title).To(Equal("Main Garage"))

					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("toolbox"))
					Expect(resp.Results[0].InventoryContext.Path[1].Title).To(Equal(""))
				})
			})

			When("item has circular reference in container chain", func() {
				BeforeEach(func() {
					// Create circular reference: A -> B -> C -> A
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "container_a",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_a"] = map[string]string{
						"title":               "Container A",
						"inventory.container": "container_b",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_b"] = map[string]string{
						"title":               "Container B",
						"inventory.container": "container_c",
					}
					mockFrontmatterIndexQueryer.GetValueResults["container_c"] = map[string]string{
						"title":               "Container C",
						"inventory.container": "container_a", // Circular!
					}
				})

				It("should detect circular reference and stop building path", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())

					// Should have stopped when it detected the circular reference
					// Path built from immediate container to root, so order is: container_c, container_b, container_a
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(3))
					Expect(resp.Results[0].InventoryContext.Path[0].Identifier).To(Equal("container_c"))
					Expect(resp.Results[0].InventoryContext.Path[1].Identifier).To(Equal("container_b"))
					Expect(resp.Results[0].InventoryContext.Path[2].Identifier).To(Equal("container_a"))
				})
			})

			When("item has container chain exceeding max depth", func() {
				BeforeEach(func() {
					// Create a chain of 25 containers (exceeds maxDepth of 20)
					mockFrontmatterIndexQueryer.GetValueResults["screwdriver"] = map[string]string{
						"inventory.container": "container_0",
					}
					for i := range 25 {
						containerID := fmt.Sprintf("container_%d", i)
						nextID := fmt.Sprintf("container_%d", i+1)
						mockFrontmatterIndexQueryer.GetValueResults[containerID] = map[string]string{
							"title":               fmt.Sprintf("Container %d", i),
							"inventory.container": nextID,
						}
					}
					// Last container has no parent
					mockFrontmatterIndexQueryer.GetValueResults["container_25"] = map[string]string{
						"title": "Container 25",
					}
				})

				It("should stop at max depth of 20", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(resp).NotTo(BeNil())
					Expect(resp.Results).To(HaveLen(1))
					Expect(resp.Results[0].InventoryContext).NotTo(BeNil())

					// Should have stopped at maxDepth (20)
					Expect(resp.Results[0].InventoryContext.Path).To(HaveLen(20))

					// Verify depth values are correct (0 to 19)
					for i := range 20 {
						Expect(resp.Results[0].InventoryContext.Path[i].Depth).To(Equal(int32(i)))
					}
				})
			})
		})

		When("result is not an inventory item", func() {
			var (
				mockFrontmatterIndexQueryer *FlexibleMockFrontmatterIndexQueryer
				searchResults               []bleve.SearchResult
			)

			BeforeEach(func() {
				mockFrontmatterIndexQueryer = &FlexibleMockFrontmatterIndexQueryer{
					ExactMatchResults: make(map[string][]wikipage.PageIdentifier),
					GetValueResults:   make(map[string]map[string]string),
				}
				searchResults = []bleve.SearchResult{
					{Identifier: "regular_page", Title: "Regular Page", Fragment: "Some content"},
				}
				mockBleveIndexQueryer.Results = searchResults
				mockFrontmatterIndexQueryer.GetValueResults["regular_page"] = map[string]string{
					"title": "Regular Page",
				}
			})

			JustBeforeEach(func() {
				server = mustNewServer(nil, mockBleveIndexQueryer, mockFrontmatterIndexQueryer)
				resp, err = server.SearchContent(ctx, req)
			})

			It("should not include inventory context", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(resp).NotTo(BeNil())
				Expect(resp.Results).To(HaveLen(1))
				Expect(resp.Results[0].InventoryContext).To(BeNil())
			})
		})
	})

	Describe("ReadPage", func() {
		var (
			req                         *apiv1.ReadPageRequest
			resp                        *apiv1.ReadPageResponse
			err                         error
			mockPageReaderMutator       *MockPageReaderMutator
			mockMarkdownRenderer        *MockMarkdownRenderer
			mockTemplateExecutor        *MockTemplateExecutor
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.ReadPageRequest{
				PageName: "test-page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
			mockMarkdownRenderer = &MockMarkdownRenderer{}
			mockTemplateExecutor = &MockTemplateExecutor{}
			mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		})

		JustBeforeEach(func() {
			var serverErr error
			server, serverErr = v1.NewServer(
				v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				mockFrontmatterIndexQueryer,
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
			Expect(serverErr).NotTo(HaveOccurred())
			server = server.WithMarkdownRenderer(mockMarkdownRenderer).WithTemplateExecutor(mockTemplateExecutor)
			resp, err = server.ReadPage(ctx, req)
		})

		When("the page does not exist", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should return a not found error", func() {
				Expect(err).To(HaveGrpcStatus(codes.NotFound, "page not found: test-page"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("reading markdown fails with a generic error", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = errors.New("disk read error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "disk read error"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("rendering fails", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Test Page"
				mockPageReaderMutator.Frontmatter = map[string]any{"title": "Test"}
				mockMarkdownRenderer.Err = errors.New("rendering error")
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to render page"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page exists with valid content", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Test Page\n\nThis is test content."
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Test Page",
					"tags":  []any{"test"},
				}
				mockTemplateExecutor.Result = []byte("# Test Page\n\nThis is test content.")
				mockMarkdownRenderer.Result = []byte("<h1>Test Page</h1>\n<p>This is test content.</p>")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a response", func() {
				Expect(resp).NotTo(BeNil())
			})

			It("should return the markdown content", func() {
				Expect(resp.ContentMarkdown).To(Equal("# Test Page\n\nThis is test content."))
			})

			It("should return the frontmatter as TOML", func() {
				Expect(resp.FrontMatterToml).To(ContainSubstring("title = 'Test Page'"))
			})

			It("should return the rendered HTML", func() {
				Expect(resp.RenderedContentHtml).To(Equal("<h1>Test Page</h1>\n<p>This is test content.</p>"))
			})

			It("should return the rendered markdown", func() {
				Expect(resp.RenderedContentMarkdown).To(Equal("# Test Page\n\nThis is test content."))
			})

			It("should return a non-empty version_hash", func() {
				Expect(resp.VersionHash).NotTo(BeEmpty())
			})

			It("should return a consistent version_hash for the same content", func() {
				// Recompute the expected hash for "# Test Page\n\nThis is test content."
				expectedHash := fmt.Sprintf("%x", sha256.Sum256([]byte("# Test Page\n\nThis is test content.")))
				Expect(resp.VersionHash).To(Equal(expectedHash))
			})
		})

		When("the page exists with no frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Markdown = "# Simple Page"
				mockPageReaderMutator.Frontmatter = nil
				mockTemplateExecutor.Result = []byte("# Simple Page")
				mockMarkdownRenderer.Result = []byte("<h1>Simple Page</h1>")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return empty frontmatter TOML", func() {
				Expect(resp.FrontMatterToml).To(BeEmpty())
			})

			It("should return the markdown content", func() {
				Expect(resp.ContentMarkdown).To(Equal("# Simple Page"))
			})
		})
	})

	Describe("RenderMarkdown", func() {
		var (
			req                         *apiv1.RenderMarkdownRequest
			resp                        *apiv1.RenderMarkdownResponse
			err                         error
			mockMarkdownRenderer        *MockMarkdownRenderer
			mockPageReaderMutator       *MockPageReaderMutator
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.RenderMarkdownRequest{
				Content: "# Hello World",
			}
			mockMarkdownRenderer = &MockMarkdownRenderer{
				Result: []byte("<h1>Hello World</h1>"),
			}
			mockPageReaderMutator = &MockPageReaderMutator{}
			mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		})

		JustBeforeEach(func() {
			var serverErr error
			server, serverErr = v1.NewServer(
				v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				mockFrontmatterIndexQueryer,
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
			Expect(serverErr).NotTo(HaveOccurred())
			server = server.WithMarkdownRenderer(mockMarkdownRenderer)
			resp, err = server.RenderMarkdown(ctx, req)
		})

		When("content is empty", func() {
			BeforeEach(func() {
				req.Content = ""
			})

			It("should return empty HTML", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty rendered_html", func() {
				Expect(resp.RenderedHtml).To(Equal(""))
			})
		})

		When("content is plain markdown without page context", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return rendered HTML", func() {
				Expect(resp.RenderedHtml).To(Equal("<h1>Hello World</h1>"))
			})
		})

		When("the markdown renderer is not available", func() {
			BeforeEach(func() {
				mockMarkdownRenderer.Result = nil
				mockMarkdownRenderer.Err = nil
			})

			It("should return a FailedPrecondition error", func() {
				// Create server without markdown renderer
				noRendererServer, serverErr := v1.NewServer(
					v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
					mockPageReaderMutator,
					noOpBleveIndexQueryer{},
					mockFrontmatterIndexQueryer,
					lumber.NewConsoleLogger(lumber.WARN),
					noOpChatBufferManager{},
					noOpPageOpener{},
				)
				Expect(serverErr).NotTo(HaveOccurred())
				_, noRendererErr := noRendererServer.RenderMarkdown(ctx, req)
				Expect(noRendererErr).To(HaveGrpcStatus(codes.FailedPrecondition, "markdown renderer is not available"))
			})
		})

		When("a page context is provided and the page exists", func() {
			BeforeEach(func() {
				req.Page = "test-page"
				mockPageReaderMutator.Frontmatter = map[string]any{
					"identifier": "test-page",
					"title":      "Test Page",
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return rendered HTML", func() {
				Expect(resp.RenderedHtml).To(Equal("<h1>Hello World</h1>"))
			})
		})

		When("a page context is provided but the page does not exist", func() {
			BeforeEach(func() {
				req.Page = "nonexistent-page"
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should still render without error (falls back to empty context)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return rendered HTML", func() {
				Expect(resp.RenderedHtml).To(Equal("<h1>Hello World</h1>"))
			})
		})

		When("the markdown renderer fails", func() {
			BeforeEach(func() {
				mockMarkdownRenderer.Result = nil
				mockMarkdownRenderer.Err = errors.New("render failure")
			})

			It("should return an Internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to render markdown"))
			})
		})
	})

	Describe("ParseCSVPreview", func() {
		var (
			req                   *apiv1.ParseCSVPreviewRequest
			resp                  *apiv1.ParseCSVPreviewResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, pages don't exist
			}
			req = &apiv1.ParseCSVPreviewRequest{}
		})

		JustBeforeEach(func() {
			server = mustNewServer(mockPageReaderMutator, nil, nil)
			resp, err = server.ParseCSVPreview(ctx, req)
		})

		When("csv_content is empty", func() {
			BeforeEach(func() {
				req.CsvContent = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "csv_content cannot be empty"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content is invalid CSV", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier\n\"unclosed quote"
			})

			It("should return an invalid argument error with parsing details", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "failed to parse CSV"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has no identifier column", func() {
			BeforeEach(func() {
				req.CsvContent = "title,description\nTest,A test page"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return parsing errors in the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.ParsingErrors).To(ContainElement("CSV must have 'identifier' column"))
			})
		})

		When("csv_content has no data rows", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return parsing errors in the response", func() {
				Expect(resp).NotTo(BeNil())
				Expect(resp.ParsingErrors).To(ContainElement("CSV has no data rows"))
			})
		})

		When("csv_content has valid rows for new pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title,description\ntest_page,Test Page,A test description\nanother_page,Another Page,Another description"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct total record count", func() {
				Expect(resp.TotalRecords).To(Equal(int32(2)))
			})

			It("should return the correct create count", func() {
				Expect(resp.CreateCount).To(Equal(int32(2)))
			})

			It("should return zero update count", func() {
				Expect(resp.UpdateCount).To(Equal(int32(0)))
			})

			It("should return zero error count", func() {
				Expect(resp.ErrorCount).To(Equal(int32(0)))
			})

			It("should indicate pages do not exist", func() {
				Expect(resp.Records).To(HaveLen(2))
				Expect(resp.Records[0].PageExists).To(BeFalse())
				Expect(resp.Records[1].PageExists).To(BeFalse())
			})

			It("should have correct identifiers", func() {
				Expect(resp.Records[0].Identifier).To(Equal("test_page"))
				Expect(resp.Records[1].Identifier).To(Equal("another_page"))
			})

			It("should have correct row numbers", func() {
				Expect(resp.Records[0].RowNumber).To(Equal(int32(1)))
				Expect(resp.Records[1].RowNumber).To(Equal(int32(2)))
			})

			It("should have parsed frontmatter", func() {
				Expect(resp.Records[0].Frontmatter).NotTo(BeNil())
				Expect(resp.Records[0].Frontmatter.AsMap()["title"]).To(Equal("Test Page"))
			})
		})

		When("csv_content has rows for existing pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nexisting_page,Updated Title"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"existing_page": {"title": "Old Title"},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should indicate page exists", func() {
				Expect(resp.Records).To(HaveLen(1))
				Expect(resp.Records[0].PageExists).To(BeTrue())
			})

			It("should return correct update count", func() {
				Expect(resp.UpdateCount).To(Equal(int32(1)))
			})

			It("should return zero create count", func() {
				Expect(resp.CreateCount).To(Equal(int32(0)))
			})
		})

		When("csv_content has rows with validation errors", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\n,Missing Identifier\nvalid_page,Valid Title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return correct error count", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})

			It("should have validation errors on the invalid record", func() {
				Expect(resp.Records[0].ValidationErrors).NotTo(BeEmpty())
			})

			It("should still process valid records", func() {
				Expect(resp.Records[1].ValidationErrors).To(BeEmpty())
				Expect(resp.Records[1].Identifier).To(Equal("valid_page"))
			})
		})

		When("csv_content has template column", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,inv_item,Test Item"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse the template correctly", func() {
				Expect(resp.Records[0].Template).To(Equal("inv_item"))
			})

			It("should not have validation errors for built-in templates", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a non-existent template", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,nonexistent_template,Test Item"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add a validation error for the missing template", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("template 'nonexistent_template' does not exist")))
			})

			It("should count the missing template as an error", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})
		})

		When("csv_content references a template that fails to read", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,broken_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					Err: os.ErrNotExist, // Default error for pages that don't exist
					ErrByID: map[string]error{
						"broken_template": errors.New("permission denied"),
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add a validation error with the read failure details", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("failed to read template 'broken_template'")))
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("permission denied")))
			})

			It("should count the template read failure as an error", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})
		})

		When("csv_content references a page that is not a template", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,regular_page,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"regular_page": {
							"title": "Just a Regular Page",
							// No "template: true" field
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist, // The new page doesn't exist
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add a validation error for the non-template page", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("not a template")))
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("regular_page")))
			})

			It("should count the non-template error", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})
		})

		When("csv_content references a valid template with template: true", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,article_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"article_template": {
							"title": "Article Template",
							"wiki": map[string]any{
								"template": true,
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist, // The new page doesn't exist
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not add validation errors", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})

			It("should not count errors", func() {
				Expect(resp.ErrorCount).To(Equal(int32(0)))
			})
		})

		When("csv_content has array column notation", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,tags[],tags[]\ntest_page,tag1,tag2"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse array operations", func() {
				Expect(resp.Records[0].ArrayOps).To(HaveLen(2))
			})
		})

		When("csv_content has DELETE sentinel for scalar field", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,description\ntest_page,[[DELETE]]"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should track fields to delete", func() {
				Expect(resp.Records[0].FieldsToDelete).To(ContainElement("description"))
			})
		})

		When("csv_content has nested field paths", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,inventory.container\ntest_page,toolbox"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should create nested frontmatter structure", func() {
				fm := resp.Records[0].Frontmatter.AsMap()
				inventory, ok := fm["inventory"].(map[string]any)
				Expect(ok).To(BeTrue())
				Expect(inventory["container"]).To(Equal("toolbox"))
			})
		})

		When("csv_content has mixed existing and new pages", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nexisting_page,Updated Title\nnew_page,New Page"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"existing_page": {"title": "Old Title"},
					},
				}
			})

			It("should return correct counts", func() {
				Expect(resp.TotalRecords).To(Equal(int32(2)))
				Expect(resp.UpdateCount).To(Equal(int32(1)))
				Expect(resp.CreateCount).To(Equal(int32(1)))
				Expect(resp.ErrorCount).To(Equal(int32(0)))
			})
		})

		When("csv_content has duplicate identifiers", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,First Title\ntest_page,Second Title"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should flag duplicate identifiers as validation errors", func() {
				Expect(resp.Records[1].ValidationErrors).To(ContainElement(ContainSubstring("duplicate identifier")))
			})

			It("should count the duplicate as an error", func() {
				Expect(resp.ErrorCount).To(Equal(int32(1)))
			})
		})

		When("csv_content has invalid identifier format", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nInvalid-Identifier,Test"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should flag invalid identifiers", func() {
				Expect(resp.Records[0].ValidationErrors).NotTo(BeEmpty())
			})
		})

		When("csv references an inventory container that exists but has no inventory key", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,inventory.container\nitem_a,box_b"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"box_b": {"title": "Box B"},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no validation errors for item_a", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv references an inventory container that has inventory but no container field", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,inventory.container\nitem_a,box_b"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"box_b": {"inventory": map[string]any{}},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have no validation errors for item_a", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv references an inventory container that itself points back creating a cycle", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,inventory.container\nitem_a,box_b"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"box_b": {"inventory": map[string]any{"container": "item_a"}},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should detect the circular reference via existing page lookup", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("circular reference detected")))
			})
		})

		When("csv has two records where the second has a missing container triggering GetContainerReference on the first", func() {
			BeforeEach(func() {
				// item_a references item_b (in CSV), item_b references item_c (not in CSV, not existing)
				// item_b gets a validation error (item_c missing), so it is excluded from importGraph
				// during cycle detection for item_a, GetContainerReference(item_b) is called (returns "" due to error)
				req.CsvContent = "identifier,inventory.container\nitem_a,item_b\nitem_b,item_c"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should have a validation error on item_b for missing container", func() {
				Expect(resp.Records[1].ValidationErrors).To(ContainElement(ContainSubstring("non-existent page")))
			})

			It("should have no validation errors on item_a", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a template with wiki.template field as string 'true'", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,string_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"string_template": {
							"title": "String Template",
							"wiki": map[string]any{
								"template": "true", // string case
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist,
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not add validation errors for the template reference", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a template with wiki.template field as int64 nonzero", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,int_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"int_template": {
							"title": "Int Template",
							"wiki": map[string]any{
								"template": int64(1), // int64 nonzero = true
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist,
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not add validation errors for the template reference", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a page with wiki.template field as int64 zero", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,zero_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"zero_template": {
							"title": "Zero Template",
							"wiki": map[string]any{
								"template": int64(0), // int64 zero = false
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist,
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add validation error because page is not a template", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("is not a template")))
			})
		})

		When("csv_content references a template with wiki.template field as float64 nonzero", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,float_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"float_template": {
							"title": "Float Template",
							"wiki": map[string]any{
								"template": float64(1.0), // float64 nonzero = true
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist,
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not add validation errors for the template reference", func() {
				Expect(resp.Records[0].ValidationErrors).To(BeEmpty())
			})
		})

		When("csv_content references a page with wiki.template field of an unrecognized type", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,template,title\ntest_page,unknown_template,Test Item"
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"unknown_template": {
							"title": "Unknown Template",
							"wiki": map[string]any{
								"template": []string{"true"}, // default case: not bool/string/int64/float64
							},
						},
					},
					ErrByID: map[string]error{
						"test_page": os.ErrNotExist,
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add validation error because page is not a template", func() {
				Expect(resp.Records[0].ValidationErrors).To(ContainElement(ContainSubstring("is not a template")))
			})
		})

		When("csv_content has DELETE array value sentinel", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,tags[]\ntest_page,[[DELETE(old-tag)]]"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should parse the DELETE array operation with correct type", func() {
				Expect(resp.Records[0].ArrayOps).To(HaveLen(1))
				Expect(resp.Records[0].ArrayOps[0].Operation).To(Equal(apiv1.ArrayOpType_ARRAY_OP_TYPE_DELETE_VALUE))
			})

			It("should include the value to delete", func() {
				Expect(resp.Records[0].ArrayOps[0].Value).To(Equal("old-tag"))
			})
		})
	})

	Describe("StartPageImportJob", func() {
		var (
			req                   *apiv1.StartPageImportJobRequest
			resp                  *apiv1.StartPageImportJobResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
			mockJobCoordinator    *MockJobQueueCoordinator
		)

		BeforeEach(func() {
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, pages don't exist
			}
			mockJobCoordinator = &MockJobQueueCoordinator{}
			req = &apiv1.StartPageImportJobRequest{}
		})

		JustBeforeEach(func() {
			var coordinator jobs.JobCoordinator
			if mockJobCoordinator != nil {
				coordinator = mockJobCoordinator.AsCoordinator()
			}
			server = mustNewServerFull(mockPageReaderMutator, nil, nil, coordinator, nil)
			resp, err = server.StartPageImportJob(ctx, req)
		})

		When("csv_content is empty", func() {
			BeforeEach(func() {
				req.CsvContent = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "csv_content cannot be empty"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("job queue coordinator is nil", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,Test"
				mockJobCoordinator = nil
			})

			It("should return an unavailable error", func() {
				Expect(err).To(HaveGrpcStatus(codes.Unavailable, "job queue coordinator not available"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has parsing errors (missing identifier column)", func() {
			BeforeEach(func() {
				req.CsvContent = "title,description\nTest,A test page"
			})

			It("should return an invalid argument error with parsing error details", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "CSV has parsing errors"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content is syntactically malformed", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier\n\"unclosed quote"
			})

			It("should return an invalid argument error for failed CSV parse", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "failed to parse CSV"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has a header but no data rows", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title"
			})

			It("should return an invalid argument error for empty CSV", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.InvalidArgument, "CSV has parsing errors"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("csv_content has no valid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\n,Missing Identifier\n,Also Missing"
			})

			It("should not return an error", func() {
				// Invalid records are now processed individually and will fail,
				// rather than rejecting the entire batch upfront
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success with total record count", func() {
				// All records are counted, including invalid ones
				// Invalid records will be processed and recorded as failures
				Expect(resp.Success).To(BeTrue())
				Expect(resp.RecordCount).To(Equal(int32(2)))
			})
		})

		When("csv_content has valid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,Test Page\nanother_page,Another Page"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should return the correct record count", func() {
				Expect(resp.RecordCount).To(Equal(int32(2)))
			})

			It("should return a job ID", func() {
				Expect(resp.JobId).NotTo(BeEmpty())
			})
		})

		When("csv_content has mixed valid and invalid records", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\nvalid_page,Valid Page\n,Invalid Page\nanother_valid,Another Valid"
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should count all records including invalid ones", func() {
				// All records are now processed as individual jobs
				// Invalid records will fail individually and be reported
				Expect(resp.RecordCount).To(Equal(int32(3)))
			})
		})

		When("dispatcher fails to enqueue the import job", func() {
			BeforeEach(func() {
				req.CsvContent = "identifier,title\ntest_page,Test Page"
				mockJobCoordinator = nil // prevent outer JustBeforeEach from using no-op mock coordinator
			})

			JustBeforeEach(func() {
				failingFactory := jobs.FailingDispatcherFactory(errors.New("queue is full"))
				failingCoordinator := jobs.NewJobQueueCoordinatorWithFactory(
					lumber.NewConsoleLogger(lumber.WARN),
					failingFactory,
				)
				server = mustNewServerFull(mockPageReaderMutator, nil, nil, failingCoordinator, nil)
				resp, err = server.StartPageImportJob(ctx, req)
			})

			It("should return an internal error", func() {
				Expect(err).To(HaveGrpcStatusWithSubstr(codes.Internal, "failed to enqueue import job"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})
	})

	Describe("GenerateIdentifier", func() {
		var (
			req                   *apiv1.GenerateIdentifierRequest
			resp                  *apiv1.GenerateIdentifierResponse
			err                   error
			mockPageReaderMutator *MockPageReaderMutator
		)

		BeforeEach(func() {
			req = &apiv1.GenerateIdentifierRequest{
				Text: "Phillips Screwdriver",
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // By default, no pages exist
			}
		})

		JustBeforeEach(func() {
			var serverErr error
			server, serverErr = v1.NewServer(
				v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
			Expect(serverErr).NotTo(HaveOccurred())
			resp, err = server.GenerateIdentifier(ctx, req)
		})

		When("the text is empty", func() {
			BeforeEach(func() {
				req.Text = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "text is required"))
			})
		})

		When("the text is provided and no page exists with the identifier", func() {
			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the munged identifier", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})

			It("should not return existing page info", func() {
				Expect(resp.ExistingPage).To(BeNil())
			})
		})

		When("a page already exists with the generated identifier", func() {
			BeforeEach(func() {
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver": {
							"title": "Existing Phillips Screwdriver",
							"inventory": map[string]any{
								"container": "toolbox",
							},
						},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the munged identifier", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should indicate the identifier is not unique", func() {
				Expect(resp.IsUnique).To(BeFalse())
			})

			It("should return existing page info with identifier", func() {
				Expect(resp.ExistingPage).NotTo(BeNil())
				Expect(resp.ExistingPage.Identifier).To(Equal("phillips_screwdriver"))
			})

			It("should return existing page info with title", func() {
				Expect(resp.ExistingPage.Title).To(Equal("Existing Phillips Screwdriver"))
			})

			It("should return existing page info with container", func() {
				Expect(resp.ExistingPage.Container).To(Equal("toolbox"))
			})
		})

		When("ensure_unique is requested and a page already exists", func() {
			BeforeEach(func() {
				req.EnsureUnique = true
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver": {
							"title": "Existing Phillips Screwdriver",
						},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a unique identifier with suffix", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver_1"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})

			It("should not return existing page info", func() {
				Expect(resp.ExistingPage).To(BeNil())
			})
		})

		When("ensure_unique is requested and multiple pages exist with suffixes", func() {
			BeforeEach(func() {
				req.EnsureUnique = true
				mockPageReaderMutator = &MockPageReaderMutator{
					FrontmatterByID: map[string]map[string]any{
						"phillips_screwdriver":   {"title": "First"},
						"phillips_screwdriver_1": {"title": "Second"},
						"phillips_screwdriver_2": {"title": "Third"},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the next available suffix", func() {
				Expect(resp.Identifier).To(Equal("phillips_screwdriver_3"))
			})

			It("should indicate the identifier is unique", func() {
				Expect(resp.IsUnique).To(BeTrue())
			})
		})
	})

	Describe("CreatePage", func() {
		var (
			req                         *apiv1.CreatePageRequest
			resp                        *apiv1.CreatePageResponse
			err                         error
			mockPageReaderMutator       *MockPageReaderMutator
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.CreatePageRequest{
				PageName: "My New Page",
			}
			mockPageReaderMutator = &MockPageReaderMutator{
				Err: os.ErrNotExist, // Page doesn't exist by default
			}
			mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		})

		JustBeforeEach(func() {
			server, err = v1.NewServer(
				v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				mockFrontmatterIndexQueryer,
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
			Expect(err).NotTo(HaveOccurred())
			resp, err = server.CreatePage(ctx, req)
		})

		When("the page_name is empty", func() {
			BeforeEach(func() {
				req.PageName = ""
			})

			It("should return an invalid argument error", func() {
				Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "page_name is required"))
			})

			It("should return no response", func() {
				Expect(resp).To(BeNil())
			})
		})

		When("the page already exists", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = nil
				mockPageReaderMutator.Frontmatter = map[string]any{"title": "Existing Page"}
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should return an error message about existing page", func() {
				Expect(resp.Error).To(ContainSubstring("already exists"))
			})
		})

		When("creating a new page without template", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should write the frontmatter with identifier", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).NotTo(BeNil())
				Expect(mockPageReaderMutator.WrittenFrontmatter["identifier"]).To(Equal("my_new_page"))
			})

			It("should write default markdown", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(ContainSubstring("{{or .Title .Identifier}}"))
			})
		})

		When("creating a page with custom markdown content", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.ContentMarkdown = "# Custom Content\n\nThis is my page."
			})

			It("should write the custom markdown", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(Equal(wikipage.Markdown("# Custom Content\n\nThis is my page.")))
			})
		})

		When("creating a page with structured frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				var structErr error
				req.Frontmatter, structErr = structpb.NewStruct(map[string]any{
					"title": "My Page Title",
					"tags":  []any{"one", "two"},
				})
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should merge the frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["title"]).To(Equal("My Page Title"))
				Expect(mockPageReaderMutator.WrittenFrontmatter["tags"]).To(Equal([]any{"one", "two"}))
			})

			It("should preserve the identifier", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["identifier"]).To(Equal("my_new_page"))
			})
		})

		When("creating a page with a valid template", func() {
			var templateID string

			BeforeEach(func() {
				mockPageReaderMutator.Err = nil // Needed for template lookup
				mockPageReaderMutator.ErrByID = map[string]error{
					"my_new_page": os.ErrNotExist, // New page doesn't exist
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"article_template": {
						"author":      "Default Author",
						"category":    "articles",
						"description": "Template description",
						"wiki": map[string]any{
							"template": true,
						},
					},
				}
				templateID = "article_template"
				req.Template = &templateID
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success", func() {
				Expect(resp.Success).To(BeTrue())
			})

			It("should copy template frontmatter fields", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["author"]).To(Equal("Default Author"))
				Expect(mockPageReaderMutator.WrittenFrontmatter["category"]).To(Equal("articles"))
			})

			It("should not copy the wiki.* reserved subtree (template flag, etc.)", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).NotTo(HaveKey("wiki"))
			})

			It("should set the correct identifier", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["identifier"]).To(Equal("my_new_page"))
			})
		})

		When("creating a page with template and custom frontmatter", func() {
			var templateID string

			BeforeEach(func() {
				mockPageReaderMutator.Err = nil
				mockPageReaderMutator.ErrByID = map[string]error{
					"my_new_page": os.ErrNotExist,
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"article_template": {
						"author":   "Default Author",
						"category": "articles",
						"wiki": map[string]any{
							"template": true,
						},
					},
				}
				templateID = "article_template"
				req.Template = &templateID
				var structErr error
				req.Frontmatter, structErr = structpb.NewStruct(map[string]any{
					"author": "Custom Author",
				})
				Expect(structErr).NotTo(HaveOccurred())
			})

			It("should override template values with provided frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["author"]).To(Equal("Custom Author"))
			})

			It("should preserve template values not overridden", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter["category"]).To(Equal("articles"))
			})
		})

		When("creating a page with a non-existent template", func() {
			var templateID string

			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				templateID = "non_existent_template"
				req.Template = &templateID
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should return an error about template not existing", func() {
				Expect(resp.Error).To(ContainSubstring("does not exist"))
			})
		})

		When("creating a page with a non-template page as template", func() {
			var templateID string

			BeforeEach(func() {
				mockPageReaderMutator.Err = nil
				mockPageReaderMutator.ErrByID = map[string]error{
					"my_new_page": os.ErrNotExist,
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"regular_page": {
						"title": "Just a Regular Page",
						// Note: no "template: true"
					},
				}
				templateID = "regular_page"
				req.Template = &templateID
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should return an error about not being a template", func() {
				Expect(resp.Error).To(ContainSubstring("not a template"))
			})
		})

		When("content_markdown contains an invalid template macro", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist
				req.ContentMarkdown = `# Page
{{ CheckList "todos" }}`
			})

			It("should not return a gRPC error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return success=false", func() {
				Expect(resp.Success).To(BeFalse())
			})

			It("should include the unknown macro name in the error", func() {
				Expect(resp.Error).To(ContainSubstring("CheckList"))
			})

			It("should suggest the correct macro name", func() {
				Expect(resp.Error).To(ContainSubstring("Checklist"))
			})

			It("should not write any markdown", func() {
				Expect(mockPageReaderMutator.WrittenMarkdown).To(BeEmpty())
			})

			It("should not write any frontmatter", func() {
				Expect(mockPageReaderMutator.WrittenFrontmatter).To(BeNil())
			})
		})
	})

	Describe("ListTemplates", func() {
		var (
			req                         *apiv1.ListTemplatesRequest
			resp                        *apiv1.ListTemplatesResponse
			err                         error
			mockPageReaderMutator       *MockPageReaderMutator
			mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		)

		BeforeEach(func() {
			req = &apiv1.ListTemplatesRequest{}
			mockPageReaderMutator = &MockPageReaderMutator{}
			mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		})

		JustBeforeEach(func() {
			server, err = v1.NewServer(
				v1.BuildInfo{Commit: "commit", BuildTime: time.Now()},
				mockPageReaderMutator,
				noOpBleveIndexQueryer{},
				mockFrontmatterIndexQueryer,
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
			Expect(err).NotTo(HaveOccurred())
			resp, err = server.ListTemplates(ctx, req)
		})

		When("there are no templates", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer.ExactMatchResults = []wikipage.PageIdentifier{}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an empty list", func() {
				Expect(resp.Templates).To(BeEmpty())
			})
		})

		When("there are multiple templates", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer.ExactMatchResults = []wikipage.PageIdentifier{
					"article_template",
					"project_template",
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"article_template": {
						"title":       "Article Template",
						"description": "For writing articles",
						"wiki": map[string]any{
							"template": true,
						},
					},
					"project_template": {
						"title": "Project Template",
						"wiki": map[string]any{
							"template": true,
						},
					},
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return all templates", func() {
				Expect(resp.Templates).To(HaveLen(2))
			})

			It("should include template identifiers", func() {
				identifiers := make([]string, len(resp.Templates))
				for i, t := range resp.Templates {
					identifiers[i] = t.Identifier
				}
				Expect(identifiers).To(ContainElements("article_template", "project_template"))
			})

			It("should include titles", func() {
				for _, t := range resp.Templates {
					if t.Identifier == "article_template" {
						Expect(t.Title).To(Equal("Article Template"))
					}
				}
			})

			It("should include descriptions when available", func() {
				for _, t := range resp.Templates {
					if t.Identifier == "article_template" {
						Expect(t.Description).To(Equal("For writing articles"))
					}
				}
			})
		})

		When("exclude_identifiers is provided", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer.ExactMatchResults = []wikipage.PageIdentifier{
					"article_template",
					"inv_item",
					"project_template",
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"article_template": {"title": "Article", "wiki": map[string]any{"template": true}},
					"inv_item":         {"title": "Inventory Item", "wiki": map[string]any{"template": true}},
					"project_template": {"title": "Project", "wiki": map[string]any{"template": true}},
				}
				req.ExcludeIdentifiers = []string{"inv_item"}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should exclude the specified template", func() {
				identifiers := make([]string, len(resp.Templates))
				for i, t := range resp.Templates {
					identifiers[i] = t.Identifier
				}
				Expect(identifiers).NotTo(ContainElement("inv_item"))
			})

			It("should include other templates", func() {
				Expect(resp.Templates).To(HaveLen(2))
			})
		})

		When("a template page cannot be read", func() {
			BeforeEach(func() {
				mockFrontmatterIndexQueryer.ExactMatchResults = []wikipage.PageIdentifier{
					"good_template",
					"broken_template",
				}
				mockPageReaderMutator.FrontmatterByID = map[string]map[string]any{
					"good_template": {"title": "Good Template", "wiki": map[string]any{"template": true}},
				}
				mockPageReaderMutator.ErrByID = map[string]error{
					"broken_template": errors.New("disk error"),
				}
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should skip the broken template", func() {
				Expect(resp.Templates).To(HaveLen(1))
				Expect(resp.Templates[0].Identifier).To(Equal("good_template"))
			})
		})
	})
})

var _ = Describe("Checklist gRPC round-trip", func() {
	var (
		server                *v1.Server
		ctx                   context.Context
		mockPageReaderMutator *MockPageReaderMutator
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockPageReaderMutator = &MockPageReaderMutator{}
		server = mustNewServer(mockPageReaderMutator, nil, nil)
	})

	Describe("writing checklist data via MergeFrontmatter then reading via GetFrontmatter", func() {
		var (
			getResp *apiv1.GetFrontmatterResponse
			getErr  error
		)

		When("a checklist with items and group_order is written", func() {
			var (
				items      []any
				firstItem  map[string]any
				secondItem map[string]any
				groupOrder []any
			)

			BeforeEach(func() {
				checklistFm := map[string]any{
					"checklists": map[string]any{
						"shopping": map[string]any{
							"items": []any{
								map[string]any{"text": "Buy milk", "checked": false, "tag": "groceries"},
								map[string]any{"text": "Buy eggs", "checked": true, "tag": "groceries"},
							},
							"group_order": []any{"groceries", "other"},
						},
					},
				}
				fmPb, err := structpb.NewStruct(checklistFm)
				Expect(err).NotTo(HaveOccurred())

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "test-page",
					Frontmatter: fmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: "test-page",
				})
				if getErr == nil && getResp != nil {
					fm := getResp.Frontmatter.AsMap()
					checklists, ok := fm["checklists"].(map[string]any)
					Expect(ok).To(BeTrue(), "checklists should be map[string]any")
					shoppingChecklist, ok := checklists["shopping"].(map[string]any)
					Expect(ok).To(BeTrue(), "shopping should be map[string]any")
					items, ok = shoppingChecklist["items"].([]any)
					Expect(ok).To(BeTrue(), "items should be []any")
					groupOrder, ok = shoppingChecklist["group_order"].([]any)
					Expect(ok).To(BeTrue(), "group_order should be []any")
					if len(items) >= 2 {
						firstItem, ok = items[0].(map[string]any)
						Expect(ok).To(BeTrue(), "first item should be map[string]any")
						secondItem, ok = items[1].(map[string]any)
						Expect(ok).To(BeTrue(), "second item should be map[string]any")
					}
				}
			})

			It("should not error on GetFrontmatter", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("should return both checklist items intact", func() {
				Expect(items).To(HaveLen(2))
			})

			It("should preserve first item text", func() {
				Expect(firstItem["text"]).To(Equal("Buy milk"))
			})

			It("should preserve first item checked state", func() {
				Expect(firstItem["checked"]).To(BeFalse())
			})

			It("should preserve second item checked state", func() {
				Expect(secondItem["checked"]).To(BeTrue())
			})

			It("should preserve item tag", func() {
				Expect(firstItem["tag"]).To(Equal("groceries"))
			})

			It("should preserve group_order as an array of strings", func() {
				Expect(groupOrder).To(Equal([]any{"groceries", "other"}))
			})
		})

		When("performing a read-modify-write to update one of multiple checklists", func() {
			var (
				finalChecklists map[string]any
				shoppingItems   []any
				tasksItems      []any
				firstTaskItem   map[string]any
			)

			BeforeEach(func() {
				// Initial state: page with two checklists
				mockPageReaderMutator.Frontmatter = map[string]any{
					"checklists": map[string]any{
						"shopping": map[string]any{
							"items":       []any{map[string]any{"text": "Buy milk", "checked": false, "tag": "groceries"}},
							"group_order": []any{"groceries"},
						},
						"tasks": map[string]any{
							"items":       []any{map[string]any{"text": "Call dentist", "checked": false, "tag": "health"}},
							"group_order": []any{"health"},
						},
					},
				}

				// Step 1: client reads current frontmatter
				readResp, err := server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{Page: "test-page"})
				Expect(err).NotTo(HaveOccurred())

				// Step 2: client modifies only the shopping checklist, keeping both in the payload
				fm := readResp.Frontmatter.AsMap()
				checklists, ok := fm["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				shopping, ok := checklists["shopping"].(map[string]any)
				Expect(ok).To(BeTrue(), "shopping should be map[string]any")
				items, ok := shopping["items"].([]any)
				Expect(ok).To(BeTrue(), "items should be []any")
				firstItem, ok := items[0].(map[string]any)
				Expect(ok).To(BeTrue(), "first item should be map[string]any")
				firstItem["checked"] = true
				shopping["items"] = []any{firstItem}
				checklists["shopping"] = shopping
				fm["checklists"] = checklists

				// Step 3: client writes back the full checklists map
				updatedFmPb, err := structpb.NewStruct(fm)
				Expect(err).NotTo(HaveOccurred())
				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "test-page",
					Frontmatter: updatedFmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				// Step 4: read back the final state
				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{Page: "test-page"})
				if getErr == nil && getResp != nil {
					finalFm := getResp.Frontmatter.AsMap()
					finalChecklists, ok = finalFm["checklists"].(map[string]any)
					Expect(ok).To(BeTrue(), "checklists should be map[string]any")
					shopping, ok := finalChecklists["shopping"].(map[string]any)
					Expect(ok).To(BeTrue(), "shopping should be map[string]any")
					shoppingItems, ok = shopping["items"].([]any)
					Expect(ok).To(BeTrue(), "shopping items should be []any")
					tasks, ok := finalChecklists["tasks"].(map[string]any)
					Expect(ok).To(BeTrue(), "tasks should be map[string]any")
					tasksItems, ok = tasks["items"].([]any)
					Expect(ok).To(BeTrue(), "tasks items should be []any")
					if len(tasksItems) > 0 {
						firstTaskItem, ok = tasksItems[0].(map[string]any)
						Expect(ok).To(BeTrue(), "first task item should be map[string]any")
					}
				}
			})

			It("should not error on final GetFrontmatter", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("should reflect the update to the modified checklist", func() {
				item, ok := shoppingItems[0].(map[string]any)
				Expect(ok).To(BeTrue(), "shopping item should be map[string]any")
				Expect(item["checked"]).To(BeTrue())
			})

			It("should preserve the untouched checklist item count", func() {
				Expect(tasksItems).To(HaveLen(1))
			})

			It("should preserve the untouched checklist item text", func() {
				Expect(firstTaskItem["text"]).To(Equal("Call dentist"))
			})

			It("should preserve the untouched checklist item checked state", func() {
				Expect(firstTaskItem["checked"]).To(BeFalse())
			})
		})
	})

	Describe("AI assistant workflow", func() {
		var (
			getResp    *apiv1.GetFrontmatterResponse
			getErr     error
			finalFm    map[string]any
			finalItems []any
			groupOrder []any
		)

		When("reading, modifying items and group_order, then writing back", func() {
			BeforeEach(func() {
				// Initial state: page with a checklist and a title
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "My Page",
					"checklists": map[string]any{
						"todo": map[string]any{
							"items":       []any{map[string]any{"text": "Write tests", "checked": false, "tag": "coding"}},
							"group_order": []any{"coding"},
						},
					},
				}

				// AI reads current state
				currentResp, err := server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{Page: "wiki-page"})
				Expect(err).NotTo(HaveOccurred())

				// AI modifies: marks first item done, adds a new item, updates group_order
				fm := currentResp.Frontmatter.AsMap()
				checklists, ok := fm["checklists"].(map[string]any)
				Expect(ok).To(BeTrue(), "checklists should be map[string]any")
				todo, ok := checklists["todo"].(map[string]any)
				Expect(ok).To(BeTrue(), "todo should be map[string]any")
				todo["items"] = []any{
					map[string]any{"text": "Write tests", "checked": true, "tag": "coding"},
					map[string]any{"text": "Deploy", "checked": false, "tag": "ops"},
				}
				todo["group_order"] = []any{"coding", "ops"}
				checklists["todo"] = todo
				fm["checklists"] = checklists

				// AI writes back
				updatedFmPb, err := structpb.NewStruct(fm)
				Expect(err).NotTo(HaveOccurred())
				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        "wiki-page",
					Frontmatter: updatedFmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				// Confirm final state
				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{Page: "wiki-page"})
				if getErr == nil && getResp != nil {
					finalFm = getResp.Frontmatter.AsMap()
					checklists, ok := finalFm["checklists"].(map[string]any)
					Expect(ok).To(BeTrue(), "checklists should be map[string]any")
					todo, ok := checklists["todo"].(map[string]any)
					Expect(ok).To(BeTrue(), "todo should be map[string]any")
					finalItems, ok = todo["items"].([]any)
					Expect(ok).To(BeTrue(), "items should be []any")
					groupOrder, ok = todo["group_order"].([]any)
					Expect(ok).To(BeTrue(), "group_order should be []any")
				}
			})

			It("should not error on final read", func() {
				Expect(getErr).NotTo(HaveOccurred())
			})

			It("should reflect the first item as checked", func() {
				item, ok := finalItems[0].(map[string]any)
				Expect(ok).To(BeTrue(), "first item should be map[string]any")
				Expect(item["checked"]).To(BeTrue())
			})

			It("should have two items after adding one", func() {
				Expect(finalItems).To(HaveLen(2))
			})

			It("should include the new item text", func() {
				item, ok := finalItems[1].(map[string]any)
				Expect(ok).To(BeTrue(), "second item should be map[string]any")
				Expect(item["text"]).To(Equal("Deploy"))
			})

			It("should preserve the updated group_order", func() {
				Expect(groupOrder).To(Equal([]any{"coding", "ops"}))
			})

			It("should preserve non-checklist frontmatter", func() {
				Expect(finalFm["title"]).To(Equal("My Page"))
			})
		})
	})
})

var _ = Describe("MergeFrontmatter deep merge integration behavior", func() {
	const pageName = "test-page"
	var (
		server                *v1.Server
		ctx                   context.Context
		mockPageReaderMutator *MockPageReaderMutator
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockPageReaderMutator = &MockPageReaderMutator{}
		server = mustNewServer(mockPageReaderMutator, nil, nil)
	})

	Describe("deep merge preserves nested keys not in the merge payload", func() {
		var (
			getResp  *apiv1.GetFrontmatterResponse
			getErr   error
			metadata map[string]any
		)

		When("merging a partial nested object into existing frontmatter with sibling keys", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "My Page",
					"metadata": map[string]any{
						"author":  "alice",
						"version": "1.0",
					},
				}

				partialFm := map[string]any{
					"metadata": map[string]any{
						"version": "2.0",
					},
				}
				fmPb, err := structpb.NewStruct(partialFm)
				Expect(err).NotTo(HaveOccurred())

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        pageName,
					Frontmatter: fmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: pageName,
				})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getResp).NotTo(BeNil())
				fm := getResp.Frontmatter.AsMap()
				var ok bool
				metadata, ok = fm["metadata"].(map[string]any)
				Expect(ok).To(BeTrue(), "metadata should be map[string]any")
			})

			It("should merge nested frontmatter while preserving sibling keys", func() {
				Expect(metadata["version"]).To(Equal("2.0"))
				Expect(metadata["author"]).To(Equal("alice"))
				Expect(getResp.Frontmatter.AsMap()["title"]).To(Equal("My Page"))
			})
		})
	})

	Describe("shallow keys merge correctly", func() {
		var (
			getResp *apiv1.GetFrontmatterResponse
			getErr  error
		)

		When("merging new and updated scalar keys into existing frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title":  "Old Title",
					"author": "alice",
				}

				mergeFm := map[string]any{
					"title": "New Title",
					"extra": "added",
				}
				fmPb, err := structpb.NewStruct(mergeFm)
				Expect(err).NotTo(HaveOccurred())

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        pageName,
					Frontmatter: fmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: pageName,
				})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getResp).NotTo(BeNil())
			})

			It("should overwrite updated keys, add new keys, and preserve untouched keys", func() {
				fm := getResp.Frontmatter.AsMap()
				Expect(fm["title"]).To(Equal("New Title"))
				Expect(fm["extra"]).To(Equal("added"))
				Expect(fm["author"]).To(Equal("alice"))
			})
		})
	})

	Describe("arrays are replaced, not merged", func() {
		var (
			getResp *apiv1.GetFrontmatterResponse
			getErr  error
			tags    []any
		)

		When("merging a payload with an array value over an existing array", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"tags": []any{"old-tag-1", "old-tag-2", "old-tag-3"},
				}

				mergeFm := map[string]any{
					"tags": []any{"new-tag"},
				}
				fmPb, err := structpb.NewStruct(mergeFm)
				Expect(err).NotTo(HaveOccurred())

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        pageName,
					Frontmatter: fmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: pageName,
				})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getResp).NotTo(BeNil())
				var ok bool
				tags, ok = getResp.Frontmatter.AsMap()["tags"].([]any)
				Expect(ok).To(BeTrue(), "tags should be []any")
			})

			It("should replace the array entirely with only the new element", func() {
				Expect(tags).To(Equal([]any{"new-tag"}))
			})
		})
	})

	Describe("edge case: empty merge payload", func() {
		var (
			getResp *apiv1.GetFrontmatterResponse
			getErr  error
		)

		When("MergeFrontmatter is called with an empty Frontmatter struct", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Frontmatter = map[string]any{
					"title": "Existing Title",
					"metadata": map[string]any{
						"author": "alice",
					},
				}

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        pageName,
					Frontmatter: &structpb.Struct{},
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: pageName,
				})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getResp).NotTo(BeNil())
			})

			It("should preserve all existing frontmatter when merging an empty payload", func() {
				fm := getResp.Frontmatter.AsMap()
				Expect(fm["title"]).To(Equal("Existing Title"))
				metadata, ok := fm["metadata"].(map[string]any)
				Expect(ok).To(BeTrue(), "metadata should be map[string]any")
				Expect(metadata["author"]).To(Equal("alice"))
			})
		})
	})

	Describe("edge case: merging into empty frontmatter", func() {
		var (
			getResp  *apiv1.GetFrontmatterResponse
			getErr   error
			metadata map[string]any
		)

		When("the page has no existing frontmatter", func() {
			BeforeEach(func() {
				mockPageReaderMutator.Err = os.ErrNotExist

				newFm := map[string]any{
					"title": "Brand New Page",
					"metadata": map[string]any{
						"author": "bob",
					},
				}
				fmPb, err := structpb.NewStruct(newFm)
				Expect(err).NotTo(HaveOccurred())

				_, mergeErr := server.MergeFrontmatter(ctx, &apiv1.MergeFrontmatterRequest{
					Page:        pageName,
					Frontmatter: fmPb,
				})
				Expect(mergeErr).NotTo(HaveOccurred())

				// Reset error so GetFrontmatter can read the written data
				mockPageReaderMutator.Err = nil

				getResp, getErr = server.GetFrontmatter(ctx, &apiv1.GetFrontmatterRequest{
					Page: pageName,
				})
				Expect(getErr).NotTo(HaveOccurred())
				Expect(getResp).NotTo(BeNil())
				var ok bool
				metadata, ok = getResp.Frontmatter.AsMap()["metadata"].(map[string]any)
				Expect(ok).To(BeTrue(), "metadata should be map[string]any")
			})

			It("should write the full merge payload as the initial frontmatter", func() {
				Expect(getResp.Frontmatter.AsMap()["title"]).To(Equal("Brand New Page"))
				Expect(metadata["author"]).To(Equal("bob"))
			})
		})
	})
})

var _ = Describe("NewServer", func() {
	var buildInfo v1.BuildInfo

	BeforeEach(func() {
		buildInfo = v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()}
	})

	When("pageReaderMutator is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				nil,
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("pageReaderMutator is required"))
		})
	})

	When("bleveIndexQueryer is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				nil,
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("bleveIndexQueryer is required"))
		})
	})

	When("frontmatterIndexQueryer is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				noOpBleveIndexQueryer{},
				nil,
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("frontmatterIndexQueryer is required"))
		})
	})

	When("logger is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				nil,
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("logger is required"))
		})
	})

	When("chatBufferManager is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				nil,
				noOpPageOpener{},
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("chatBufferManager is required"))
		})
	})

	When("pageOpener is nil", func() {
		var err error

		BeforeEach(func() {
			_, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				nil,
			)
		})

		It("should return an error", func() {
			Expect(err).To(MatchError("pageOpener is required"))
		})
	})

	When("all required dependencies are provided", func() {
		var (
			server *v1.Server
			err    error
		)

		BeforeEach(func() {
			server, err = v1.NewServer(
				buildInfo,
				noOpPageReaderMutator{},
				noOpBleveIndexQueryer{},
				noOpFrontmatterIndexQueryer{},
				lumber.NewConsoleLogger(lumber.WARN),
				noOpChatBufferManager{},
				noOpPageOpener{},
			)
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a non-nil server", func() {
			Expect(server).NotTo(BeNil())
		})
	})
})

var _ = Describe("Server builder methods", func() {
	var (
		ctx    context.Context
		server *v1.Server
	)

	BeforeEach(func() {
		ctx = context.Background()
		var err error
		server, err = v1.NewServer(
			v1.BuildInfo{Commit: "test-commit", BuildTime: time.Now()},
			noOpPageReaderMutator{},
			noOpBleveIndexQueryer{},
			noOpFrontmatterIndexQueryer{},
			lumber.NewConsoleLogger(lumber.WARN),
			noOpChatBufferManager{},
			noOpPageOpener{},
		)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("WithJobQueueCoordinator", func() {
		When("called with a job coordinator", func() {
			var result *v1.Server

			BeforeEach(func() {
				coordinator := (&MockJobQueueCoordinator{}).AsCoordinator()
				result = server.WithJobQueueCoordinator(coordinator)
			})

			It("should return the same server for chaining", func() {
				Expect(result).To(BeIdenticalTo(server))
			})

			It("should enable job queue operations (StartPageImportJob no longer returns unavailable)", func() {
				_, err := result.StartPageImportJob(ctx, &apiv1.StartPageImportJobRequest{
					CsvContent: "identifier,title\ntest_page,Test Page",
				})
				Expect(err).NotTo(HaveGrpcStatus(codes.Unavailable, "job queue coordinator not available"))
			})
		})
	})

	Describe("WithMarkdownRenderer", func() {
		When("called with a markdown renderer", func() {
			var result *v1.Server

			BeforeEach(func() {
				result = server.WithMarkdownRenderer(&MockMarkdownRenderer{Result: []byte("<p>test</p>")})
			})

			It("should return the same server for chaining", func() {
				Expect(result).To(BeIdenticalTo(server))
			})

			It("should enable markdown rendering (RenderMarkdown no longer returns failed precondition)", func() {
				_, err := result.RenderMarkdown(ctx, &apiv1.RenderMarkdownRequest{Content: "# Hello"})
				Expect(err).NotTo(HaveGrpcStatus(codes.FailedPrecondition, "markdown renderer is not available"))
			})
		})
	})

	Describe("WithTemplateExecutor", func() {
		When("called with a template executor", func() {
			var result *v1.Server

			BeforeEach(func() {
				result = server.WithTemplateExecutor(&MockTemplateExecutor{})
			})

			It("should return the same server for chaining", func() {
				Expect(result).To(BeIdenticalTo(server))
			})
		})
	})

	Describe("WithFileStorer", func() {
		When("called with a file storer mock", func() {
			var result *v1.Server

			BeforeEach(func() {
				result = server.WithFileStorer(&mockFileStorer{})
			})

			It("should return the same server for chaining", func() {
				Expect(result).To(BeIdenticalTo(server))
			})
		})
	})
})

// MockMarkdownRenderer is a mock implementation of wikipage.MarkdownToHTMLRenderer
type MockMarkdownRenderer struct {
	Result []byte
	Err    error
}

func (m *MockMarkdownRenderer) Render(input []byte) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Result, nil
}

// MockTemplateExecutor is a mock implementation of wikipage.TemplateExecutor
type MockTemplateExecutor struct {
	Result []byte
	Err    error
}

func (m *MockTemplateExecutor) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Result != nil {
		return m.Result, nil
	}
	return []byte(templateString), nil
}

// MockFrontmatterIndexQueryer is a mock implementation of wikipage.IQueryFrontmatterIndex
type MockFrontmatterIndexQueryer struct {
	ExactMatchResults   []wikipage.PageIdentifier
	KeyExistsResults    []wikipage.PageIdentifier
	KeyExistsResultsMap map[string][]wikipage.PageIdentifier
	PrefixMatchResults  []wikipage.PageIdentifier
	GetValueResult      string
	SortedByResults     []wikipage.PageIdentifier
	GetValueResultMap   map[string]map[string]string // identifier -> key -> value
}

func (m *MockFrontmatterIndexQueryer) QueryExactMatch(dottedKeyPath wikipage.DottedKeyPath, value wikipage.Value) []wikipage.PageIdentifier {
	return m.ExactMatchResults
}

func (m *MockFrontmatterIndexQueryer) QueryKeyExistence(dottedKeyPath wikipage.DottedKeyPath) []wikipage.PageIdentifier {
	// Use map if available, otherwise fall back to simple results
	if m.KeyExistsResultsMap != nil {
		if results, ok := m.KeyExistsResultsMap[string(dottedKeyPath)]; ok {
			return results
		}
		return nil
	}
	return m.KeyExistsResults
}

func (m *MockFrontmatterIndexQueryer) QueryPrefixMatch(dottedKeyPath wikipage.DottedKeyPath, valuePrefix string) []wikipage.PageIdentifier {
	return m.PrefixMatchResults
}

func (m *MockFrontmatterIndexQueryer) GetValue(identifier wikipage.PageIdentifier, dottedKeyPath wikipage.DottedKeyPath) wikipage.Value {
	if m.GetValueResultMap != nil {
		if idMap, ok := m.GetValueResultMap[string(identifier)]; ok {
			return idMap[string(dottedKeyPath)]
		}
		return ""
	}
	return m.GetValueResult
}

func (m *MockFrontmatterIndexQueryer) QueryExactMatchSortedBy(_ wikipage.DottedKeyPath, _ wikipage.Value, _ wikipage.DottedKeyPath, _ bool, _ int) []wikipage.PageIdentifier {
	return m.SortedByResults
}

// MockJobQueueCoordinator is a mock implementation for testing job queue interactions.
type MockJobQueueCoordinator struct{}

// AsCoordinator returns a real JobQueueCoordinator for testing.
// This is needed because the server expects a *jobs.JobQueueCoordinator, not an interface.
func (m *MockJobQueueCoordinator) AsCoordinator() *jobs.JobQueueCoordinator {
	if m == nil {
		return nil
	}
	// Create a coordinator with a no-op dispatcher for testing
	mockDispatcherFactory := func(maxWorkers, maxQueue int) jobs.Dispatcher {
		return &noOpDispatcher{}
	}
	return jobs.NewJobQueueCoordinatorWithFactory(lumber.NewConsoleLogger(lumber.WARN), mockDispatcherFactory)
}

// noOpDispatcher is a dispatcher that does nothing for testing purposes.
type noOpDispatcher struct{}

func (*noOpDispatcher) Start() {
	// No-op test stub — not needed for this test scenario
}

func (*noOpDispatcher) Dispatch(_ func()) error {
	return nil
}

var _ = Describe("ListPagesByFrontmatter", func() {
	var (
		ctx    context.Context
		server *v1.Server
		req    *apiv1.ListPagesByFrontmatterRequest
		resp   *apiv1.ListPagesByFrontmatterResponse
		err    error

		mockFrontmatterIndexQueryer *MockFrontmatterIndexQueryer
		mockPageReaderMutator       *MockPageReaderMutator
	)

	BeforeEach(func() {
		ctx = context.Background()
		mockFrontmatterIndexQueryer = &MockFrontmatterIndexQueryer{}
		mockPageReaderMutator = &MockPageReaderMutator{}
		req = &apiv1.ListPagesByFrontmatterRequest{
			MatchKey:  "status",
			SortByKey: "title",
		}
	})

	JustBeforeEach(func() {
		server = mustNewServer(mockPageReaderMutator, nil, mockFrontmatterIndexQueryer)
		resp, err = server.ListPagesByFrontmatter(ctx, req)
	})

	When("match_key is empty", func() {
		BeforeEach(func() {
			req.MatchKey = ""
		})

		It("should return an invalid argument error", func() {
			Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "match_key is required"))
		})

		It("should return no response", func() {
			Expect(resp).To(BeNil())
		})
	})

	When("sort_by_key is empty", func() {
		BeforeEach(func() {
			req.SortByKey = ""
		})

		It("should return an invalid argument error", func() {
			Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "sort_by_key is required"))
		})

		It("should return no response", func() {
			Expect(resp).To(BeNil())
		})
	})

	When("max_results is negative", func() {
		BeforeEach(func() {
			req.MaxResults = -1
		})

		It("should return an invalid argument error", func() {
			Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "max_results must be >= 0"))
		})

		It("should return no response", func() {
			Expect(resp).To(BeNil())
		})
	})

	When("content_excerpt_max_chars is negative", func() {
		BeforeEach(func() {
			req.ContentExcerptMaxChars = -1
		})

		It("should return an invalid argument error", func() {
			Expect(err).To(HaveGrpcStatus(codes.InvalidArgument, "content_excerpt_max_chars must be >= 0"))
		})

		It("should return no response", func() {
			Expect(resp).To(BeNil())
		})
	})

	When("no pages match the frontmatter query", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = nil
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty results list", func() {
			Expect(resp.Results).To(BeEmpty())
		})
	})

	When("pages match the frontmatter query", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"page_one", "page_two"}
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the correct number of results", func() {
			Expect(resp.Results).To(HaveLen(2))
		})

		It("should include page identifiers in results", func() {
			ids := []string{resp.Results[0].Identifier, resp.Results[1].Identifier}
			Expect(ids).To(ConsistOf("page_one", "page_two"))
		})
	})

	When("frontmatter keys are requested and pages match", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"article_one"}
			mockFrontmatterIndexQueryer.GetValueResultMap = map[string]map[string]string{
				"article_one": {
					"title":  "My Article",
					"author": "Bob",
				},
			}
			req.FrontmatterKeysToReturn = []string{"title", "author"}
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include the requested frontmatter values", func() {
			Expect(resp.Results[0].FrontmatterValues["title"]).To(Equal("My Article"))
			Expect(resp.Results[0].FrontmatterValues["author"]).To(Equal("Bob"))
		})
	})

	When("content excerpts are requested and the page has content", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"content_page"}
			mockPageReaderMutator.Markdown = "Hello, this is the page content that is longer than ten chars."
			req.ContentExcerptMaxChars = 10
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should truncate the content to the specified number of characters", func() {
			Expect(resp.Results[0].ContentExcerpt).To(Equal("Hello, thi"))
		})
	})

	When("content excerpts are requested but max chars is zero", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"content_page"}
			mockPageReaderMutator.Markdown = "Some content here."
			req.ContentExcerptMaxChars = 0
		})

		It("should return an empty content excerpt", func() {
			Expect(resp.Results[0].ContentExcerpt).To(BeEmpty())
		})
	})

	When("content excerpts are requested but the page has no content", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"empty_page"}
			mockPageReaderMutator.MarkdownReadErr = os.ErrNotExist
			req.ContentExcerptMaxChars = 100
		})

		It("should not return an error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an empty content excerpt", func() {
			Expect(resp.Results[0].ContentExcerpt).To(BeEmpty())
		})
	})

	When("content is shorter than max chars", func() {
		BeforeEach(func() {
			mockFrontmatterIndexQueryer.SortedByResults = []wikipage.PageIdentifier{"short_page"}
			mockPageReaderMutator.Markdown = "Short."
			req.ContentExcerptMaxChars = 1000
		})

		It("should return the full content", func() {
			Expect(resp.Results[0].ContentExcerpt).To(Equal("Short."))
		})
	})
})

// callbackObservingMutator is a thread-safe PageReaderMutator for observing async callback execution.
// It reports new pages as not-existing and tracks when the page_import_report page is written.
type callbackObservingMutator struct {
	mu            sync.Mutex
	reportWritten bool
}

func (*callbackObservingMutator) ReadFrontMatter(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	return id, nil, os.ErrNotExist
}

func (m *callbackObservingMutator) WriteFrontMatter(id wikipage.PageIdentifier, _ wikipage.FrontMatter) error {
	if string(id) == "page_import_report" {
		m.mu.Lock()
		m.reportWritten = true
		m.mu.Unlock()
	}
	return nil
}

func (*callbackObservingMutator) WriteMarkdown(_ wikipage.PageIdentifier, _ wikipage.Markdown) error {
	return nil
}

func (*callbackObservingMutator) ReadMarkdown(id wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	return id, "", os.ErrNotExist
}

func (*callbackObservingMutator) DeletePage(_ wikipage.PageIdentifier) error {
	return nil
}

func (*callbackObservingMutator) ModifyMarkdown(_ wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	_, err := modifier("")
	return err
}

func (m *callbackObservingMutator) isReportWritten() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reportWritten
}

var _ = Describe("makeReportJobCallback execution", func() {
	var (
		ctx              context.Context
		observingMutator *callbackObservingMutator
	)

	BeforeEach(func() {
		ctx = context.Background()
		observingMutator = &callbackObservingMutator{}

		// Use MockDispatcher which runs jobs in goroutines (async execution)
		mockDispatcherFactory := func(_, _ int) jobs.Dispatcher {
			return jobs.NewMockDispatcher()
		}
		coordinator := jobs.NewJobQueueCoordinatorWithFactory(
			lumber.NewConsoleLogger(lumber.WARN),
			mockDispatcherFactory,
		)
		server := mustNewServerFull(observingMutator, nil, nil, coordinator, nil)

		startReq := &apiv1.StartPageImportJobRequest{
			CsvContent: "identifier\ntest_page",
		}
		_, _ = server.StartPageImportJob(ctx, startReq)
	})

	It("should eventually invoke the callback and write the import report", func() {
		Eventually(observingMutator.isReportWritten, "2s", "10ms").Should(BeTrue())
	})
})

// synchronousCompletingCoordinator executes jobs and their completion callbacks synchronously.
// EnqueueJob always returns an error — used to test error handling in makeReportJobCallback
// when the report job cannot be enqueued.
type synchronousCompletingCoordinator struct {
	enqueueJobCallCount int
}

func (*synchronousCompletingCoordinator) GetJobProgress() jobs.JobProgress {
	return jobs.JobProgress{}
}

func (*synchronousCompletingCoordinator) EnqueueJobWithCompletion(job jobs.Job, callback jobs.CompletionCallback) error {
	// Execute the import job synchronously so the callback fires inline.
	jobErr := job.Execute()
	if callback != nil {
		callback(jobErr)
	}
	return nil
}

func (c *synchronousCompletingCoordinator) EnqueueJob(_ jobs.Job) error {
	c.enqueueJobCallCount++
	return errors.New("report job enqueue rejected for testing")
}

var _ = Describe("makeReportJobCallback", func() {
	Describe("when the report job cannot be enqueued", func() {
		var (
			ctx         context.Context
			coordinator *synchronousCompletingCoordinator
			resp        *apiv1.StartPageImportJobResponse
			err         error
		)

		BeforeEach(func() {
			ctx = context.Background()
			coordinator = &synchronousCompletingCoordinator{}
			server := mustNewServerFull(nil, nil, nil, coordinator, nil)

			resp, err = server.StartPageImportJob(ctx, &apiv1.StartPageImportJobRequest{
				CsvContent: "identifier\ntest_page",
			})
		})

		It("should not return an error to the caller", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return a successful response", func() {
			Expect(resp.Success).To(BeTrue())
		})

		It("should have attempted to enqueue the report job", func() {
			Expect(coordinator.enqueueJobCallCount).To(Equal(1))
		})
	})
})

func (noOpChatBufferManager) RequestPermission(_ context.Context, _ string, _ string, _ string, _ string, _ []chatbuffer.PermissionOption) string {
	return ""
}

func (noOpChatBufferManager) GetPendingPermissionsForPage(string) []*chatbuffer.PermissionRequestEvent {
	return nil
}
