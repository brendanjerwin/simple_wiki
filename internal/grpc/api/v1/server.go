// Package v1 provides the implementation of gRPC services for version 1 of the API
package v1

import (
	"context"
	"errors"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/filestore"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/pkg/chatbuffer"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/jcelliott/lumber"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	identifierKey                  = "identifier"
	pageNotFoundErrFmt             = "page not found: %s"
	failedToReadFrontmatterErrFmt  = "failed to read frontmatter: %v"
	failedToWriteFrontmatterErrFmt = "failed to write frontmatter: %v"
	failedToWriteMarkdownErrFmt    = "failed to write markdown: %v"
	pageNameRequiredErr            = "page_name is required"
	maxUniqueIdentifierAttempts    = 1000
	invalidTemplateErrFmt          = "invalid template in page content: %v"
)

// ChatBufferManager defines the interface for managing chat message buffers.
type ChatBufferManager interface {
	AddUserMessage(page, content, senderName string) (string, error)
	AddAssistantMessage(page, content, replyToID string) (string, error)
	EditMessage(messageID, newContent string, streaming bool) error
	AddReaction(messageID, emoji, reactor string) error
	GetMessages(page string) []*chatbuffer.Message
	SubscribeToPage(page string) (<-chan chatbuffer.Event, func())
	SubscribeToPageWithReplay(page string) ([]*chatbuffer.Message, <-chan chatbuffer.Event, func())
	SubscribeToPageChannel(page string) (<-chan *chatbuffer.Message, func())
	SubscribeToPageChannelWithReplay(page string) ([]*chatbuffer.Message, <-chan *chatbuffer.Message, func())
	HasPageChannelSubscriber(page string) bool
	RequestInstance(page string)
	SubscribeToInstanceRequests() (<-chan string, func())
	HasInstanceRequestSubscribers() bool
	IsInstanceRequested(page string) bool
	NotifyToolCall(page, messageID, toolCallID, title, toolStatus string)
	CancelPage(page string) bool
	SubscribeToCancellation(page string) (<-chan struct{}, func())
	RequestPermission(page, requestID, title, description string, options []chatbuffer.PermissionOption) string
	EmitPermissionRequest(page string, event *chatbuffer.PermissionRequestEvent)
	RespondToPermission(requestID, selectedOptionID string)
}

// BuildInfo contains version and build metadata for the server.
type BuildInfo struct {
	Commit    string
	BuildTime time.Time
}

// Server is the implementation of the gRPC services.
type Server struct {
	apiv1.UnimplementedSystemInfoServiceServer
	apiv1.UnimplementedFrontmatterServer
	apiv1.UnimplementedPageManagementServiceServer
	apiv1.UnimplementedSearchServiceServer
	apiv1.UnimplementedInventoryManagementServiceServer
	apiv1.UnimplementedPageImportServiceServer
	apiv1.UnimplementedFileStorageServiceServer
	apiv1.UnimplementedChatServiceServer
	commit                  string
	buildTime               time.Time
	pageReaderMutator       wikipage.PageReaderMutator
	bleveIndexQueryer       bleve.BleveIndexQueryer
	jobQueueCoordinator     jobs.JobCoordinator
	logger                  *lumber.ConsoleLogger
	markdownRenderer        wikipage.MarkdownToHTMLRenderer
	templateExecutor        wikipage.TemplateExecutor
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex
	fileStorer              filestore.FileStorer
	chatBufferManager       ChatBufferManager
	pageOpener              wikipage.PageOpener
}

// NewServer creates a new gRPC server with the given dependencies.
// Required dependencies: pageReaderMutator, bleveIndexQueryer, frontmatterIndexQueryer, logger, chatBufferManager, pageOpener.
// Optional dependencies can be set via WithJobQueueCoordinator, WithMarkdownRenderer, WithTemplateExecutor, WithFileStorer.
func NewServer(
	buildInfo BuildInfo,
	pageReaderMutator wikipage.PageReaderMutator,
	bleveIndexQueryer bleve.BleveIndexQueryer,
	frontmatterIndexQueryer wikipage.IQueryFrontmatterIndex,
	logger *lumber.ConsoleLogger,
	chatBufferManager ChatBufferManager,
	pageOpener wikipage.PageOpener,
) (*Server, error) {
	if pageReaderMutator == nil {
		return nil, errors.New("pageReaderMutator is required")
	}
	if bleveIndexQueryer == nil {
		return nil, errors.New("bleveIndexQueryer is required")
	}
	if frontmatterIndexQueryer == nil {
		return nil, errors.New("frontmatterIndexQueryer is required")
	}
	if logger == nil {
		return nil, errors.New("logger is required")
	}
	if chatBufferManager == nil {
		return nil, errors.New("chatBufferManager is required")
	}
	if pageOpener == nil {
		return nil, errors.New("pageOpener is required")
	}
	return &Server{
		commit:                  buildInfo.Commit,
		buildTime:               buildInfo.BuildTime,
		pageReaderMutator:       pageReaderMutator,
		bleveIndexQueryer:       bleveIndexQueryer,
		frontmatterIndexQueryer: frontmatterIndexQueryer,
		logger:                  logger,
		chatBufferManager:       chatBufferManager,
		pageOpener:              pageOpener,
	}, nil
}

// WithJobQueueCoordinator sets the optional job queue coordinator and returns the server for chaining.
func (s *Server) WithJobQueueCoordinator(jqc jobs.JobCoordinator) *Server {
	s.jobQueueCoordinator = jqc
	return s
}

// WithMarkdownRenderer sets the optional markdown renderer and returns the server for chaining.
func (s *Server) WithMarkdownRenderer(r wikipage.MarkdownToHTMLRenderer) *Server {
	s.markdownRenderer = r
	return s
}

// WithTemplateExecutor sets the optional template executor and returns the server for chaining.
func (s *Server) WithTemplateExecutor(e wikipage.TemplateExecutor) *Server {
	s.templateExecutor = e
	return s
}

// WithFileStorer sets the optional file storer and returns the server for chaining.
func (s *Server) WithFileStorer(fs filestore.FileStorer) *Server {
	s.fileStorer = fs
	return s
}

// RegisterWithServer registers the gRPC services with the given gRPC server.
func (s *Server) RegisterWithServer(grpcServer *grpc.Server) {
	apiv1.RegisterSystemInfoServiceServer(grpcServer, s)
	apiv1.RegisterFrontmatterServer(grpcServer, s)
	apiv1.RegisterPageManagementServiceServer(grpcServer, s)
	apiv1.RegisterSearchServiceServer(grpcServer, s)
	apiv1.RegisterInventoryManagementServiceServer(grpcServer, s)
	apiv1.RegisterPageImportServiceServer(grpcServer, s)
	apiv1.RegisterFileStorageServiceServer(grpcServer, s)
	apiv1.RegisterChatServiceServer(grpcServer, s)
}

// LoggingInterceptor returns a gRPC unary interceptor for logging method calls.
func (s *Server) LoggingInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()

		// Call the method
		resp, err := handler(ctx, req)

		// Log the request in a format similar to Gin
		duration := time.Since(start)
		statusCode := codes.OK
		if err != nil {
			if st, ok := status.FromError(err); ok {
				statusCode = st.Code()
			}
		}

		// Get identity if available
		identity := tailscale.IdentityFromContext(ctx)
		identityStr := "anonymous"
		if !identity.IsAnonymous() {
			identityStr = identity.ForLog()
		}

		if s.logger != nil {
			s.logger.Warn("[GRPC] %s | %s | %v | %s",
				statusCode,
				duration,
				info.FullMethod,
				identityStr,
			)
		}

		return resp, err
	}
}
