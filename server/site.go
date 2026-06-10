package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/brendanjerwin/simple_wiki/filestore"
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/migrations/canonicalize"
	"github.com/brendanjerwin/simple_wiki/migrations/eager"
	"github.com/brendanjerwin/simple_wiki/pkg/jobs"
	"github.com/brendanjerwin/simple_wiki/pkg/ulid"
	"github.com/brendanjerwin/simple_wiki/server/pagestore"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/brendanjerwin/simple_wiki/utils/goldmarkrenderer"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jcelliott/lumber"
	"github.com/pelletier/go-toml/v2"
)

// MarkdownToHTMLRenderer is a type alias for the rendering interface
type MarkdownToHTMLRenderer = wikipage.MarkdownToHTMLRenderer

// TemplateExecutor implements the wikipage.TemplateExecutor interface using the templating package
type TemplateExecutor struct{}

// ExecuteTemplate executes a template using the templating package
func (TemplateExecutor) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	return templating.ExecuteTemplate(templateString, fm, reader, query)
}

// ChatTemplateExecutor implements the wikipage.TemplateExecutor interface with a
// restricted set of macros suitable for chat messages. Excludes interactive widget
// macros (Checklist, Blog) that render web components inappropriate for chat bubbles.
type ChatTemplateExecutor struct{}

// ExecuteTemplate executes a template using the chat-safe macro set.
func (ChatTemplateExecutor) ExecuteTemplate(templateString string, fm wikipage.FrontMatter, reader wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	return templating.ExecuteChatTemplate(templateString, fm, reader, query)
}

// Site represents the wiki site.
type Site struct {
	PathToData               string
	CSS                      []byte
	DefaultPage              string
	Debounce                 int
	ChatPersona              string
	SessionStore             cookie.Store
	Fileuploads              bool
	MaxUploadSize            uint
	MaxDocumentSize          uint // in runes; about a 10mb limit by default
	FileStorer               filestore.FileStorer
	Logger                   *lumber.ConsoleLogger
	MarkdownRenderer         MarkdownToHTMLRenderer
	IndexCoordinator         *index.IndexCoordinator
	JobQueueCoordinator      *jobs.JobQueueCoordinator
	CronScheduler            *jobs.CronScheduler
	FrontmatterIndexQueryer  frontmatter.IQueryFrontmatterIndex
	BleveIndexQueryer        bleve.BleveIndexQueryer
	AgentScheduleStore       *AgentScheduleStore
	AgentChatContextStore    *AgentChatContextStore
	AgentScheduler           *AgentScheduler
	ScheduledTurnDispatcher  *ScheduledTurnDispatcher
	AgentScheduleConcurrency int
	AgentScheduleQueueCap    int
	AgentTurnHardTimeout     time.Duration
	// caldavServer dispatches CalDAV-shaped HTTP traffic. The
	// caldavGateway middleware (registered before route matching)
	// forwards qualifying requests here; everything else falls through
	// to the regular Gin routes. nil disables CalDAV routing — used in
	// tests that don't need the CalDAV surface.
	caldavServer http.Handler

	// store is the disk-backed storage primitive. All page I/O and per-page
	// locking lives in *pagestore.Store; Site delegates here and adds the
	// indexing / scheduling side effects that don't belong on the storage
	// primitive itself.
	store *pagestore.Store

	// reader wraps store with the read-side canonicalization decorator so
	// reads always return canonical bytes regardless of on-disk state.
	// Tracked separately from `store` because pure-storage tests
	// (eager-backfill ScanJob, etc.) want the bare *Store, while
	// user-facing reads want canonicalized output.
	reader pagestore.Reader

	// storeInitMu guards lazy initialization of `store` and `reader` in
	// ensureStore so parallel goroutines (and parallel Ginkgo specs) don't
	// race on the field mutation. Production-path Site is initialized
	// completely in NewSite before any handler runs; this mutex protects
	// only the test-time lazy-init path.
	storeInitMu sync.Mutex
}

// SetCalDAVServer installs the CalDAV HTTP handler the caldavGateway
// middleware will dispatch CalDAV-shaped requests to. Pass nil to
// disable CalDAV routing (the gateway becomes a no-op pass-through).
func (s *Site) SetCalDAVServer(h http.Handler) {
	s.caldavServer = h
}

// LoadCustomCSS reads custom CSS from the given file path and assigns it to s.CSS.
// Does nothing if cssFile is empty.
func (s *Site) LoadCustomCSS(cssFile string) error {
	if len(cssFile) == 0 {
		return nil
	}
	customCSS, err := os.ReadFile(cssFile)
	if err != nil {
		return fmt.Errorf("failed to read CSS file %s: %w", cssFile, err)
	}
	_, _ = fmt.Printf("Loaded CSS file, %d bytes\n", len(customCSS))
	s.CSS = customCSS
	return nil
}

// NewSite creates and initializes a new Site instance.
// To configure custom CSS, call site.LoadCustomCSS after creation.
// To configure file uploads, set site.Fileuploads, site.MaxUploadSize, and site.MaxDocumentSize after creation.
func NewSite(
	filepathToData string,
	defaultPage string,
	debounce int,
	secret string,
	logger *lumber.ConsoleLogger,
) (*Site, error) {
	logger.Info("Initializing simple_wiki site...")

	site := &Site{
		PathToData:       filepathToData,
		DefaultPage:      defaultPage,
		Debounce:         debounce,
		SessionStore:     cookie.NewStore([]byte(secret)),
		Logger:           logger,
		MarkdownRenderer: &goldmarkrenderer.GoldmarkRenderer{},
	}
	// Real format canonicalizer wired into both read and write paths.
	// CanonicalReader returns canonical bytes from every read; Store's
	// writeRawTextLocked canonicalizes before every write. Replaces the
	// lazy save-on-read chain that previously lived in applyMigrationsForPage.
	canonicalizer := canonicalize.NewFormatCanonicalizer()
	site.store = pagestore.NewStore(filepathToData)
	site.store.SetCanonicalizer(canonicalizer)
	site.reader = pagestore.NewCanonicalReader(canonicalizer, site.store)

	logger.Info("Initializing site indexing...")
	err := site.InitializeIndexing()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize site: %w", err)
	}

	fileStorer, err := filestore.NewDiskFileStorer(filepathToData)
	if err != nil {
		return nil, fmt.Errorf("failed to create file storer: %w", err)
	}
	site.FileStorer = fileStorer

	logger.Info("Site initialization complete")
	return site, nil
}

const (
	tomlDelimiter          = "+++\n"
	mdExtension            = "md"
	newline                = "\n"
	failedToOpenPageErrFmt = "failed to open page %s: %w"
)

func (s *Site) sniffContentType(name string) (string, error) {
	file, err := os.Open(path.Join(s.PathToData, name))
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	// The mimetype library reads up to 3072 bytes by default.
	buffer := make([]byte, 3072)
	n, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	// Use mimetype library to detect content type. It is more accurate and
	// can detect charsets.
	mtype := mimetype.Detect(buffer[:n])
	return mtype.String(), nil
}

// startMigrationJobs starts background migration jobs for file shadowing and JSON archive migration.
func (s *Site) startMigrationJobs() {
	// Start file shadowing scan
	dataDirScanner := eager.NewFileSystemDataDirScanner(s.PathToData)
	scanJob := eager.NewFileShadowingMigrationScanJob(dataDirScanner, s.JobQueueCoordinator, s, s)
	if err := s.JobQueueCoordinator.EnqueueJob(scanJob); err != nil {
		s.Logger.Error("Failed to enqueue file shadowing scan job: %v", err)
	} else {
		s.Logger.Info("File shadowing scan started.")
	}

	// Start JSON archive migration to move .json files to __deleted__
	jsonArchiveJob := eager.NewJSONArchiveMigrationScanJob(s.PathToData, s.JobQueueCoordinator)
	if err := s.JobQueueCoordinator.EnqueueJob(jsonArchiveJob); err != nil {
		s.Logger.Error("Failed to enqueue JSON archive migration job: %v", err)
	} else {
		s.Logger.Info("JSON archive migration started.")
	}

	// One-shot migration: rewrite legacy `:tag` checklist items to `#tag` and
	// stamp `migrated_tags_syntax = true`. Once we're confident no pages have
	// the old syntax in the wild, the scan/job pair can be deleted entirely.
	checklistTagMigration := eager.NewChecklistTagSyntaxMigrationScanJob(dataDirScanner, s.JobQueueCoordinator, s)
	if err := s.JobQueueCoordinator.EnqueueJob(checklistTagMigration); err != nil {
		s.Logger.Error("Failed to enqueue checklist tag-syntax migration job: %v", err)
	} else {
		s.Logger.Info("Checklist tag-syntax migration started.")
	}

	// One-shot migration: promote legacy checklist items to the #984 shape
	// (ULIDs, per-item created_at/updated_at, per-list sync_token under
	// wiki.checklists.*). Once every page has been migrated and the
	// migrated_data_model flag is set everywhere, this scan becomes a
	// no-op and the migration code can be deleted.
	checklistDataModelMigration := eager.NewChecklistDataModelMigrationScanJob(
		dataDirScanner, s.JobQueueCoordinator, s, ulid.NewSystemGenerator(),
	)
	if err := s.JobQueueCoordinator.EnqueueJob(checklistDataModelMigration); err != nil {
		s.Logger.Error("Failed to enqueue checklist data-model migration job: %v", err)
	} else {
		s.Logger.Info("Checklist data-model migration started.")
	}

	// One-shot migration: move legacy top-level `system` and `template`
	// frontmatter keys under the reserved `wiki.*` namespace (per #997 and
	// ADR-0010). Once every page carries `wiki.migrated_namespaces = true`
	// and the dual-read fallback is removed (#1001), this scan can be
	// deleted entirely.
	namespaceMigration := eager.NewSystemTemplateNamespaceMigrationScanJob(
		dataDirScanner, s.JobQueueCoordinator, s,
	)
	if err := s.JobQueueCoordinator.EnqueueJob(namespaceMigration); err != nil {
		s.Logger.Error("Failed to enqueue system/template namespace migration job: %v", err)
	} else {
		s.Logger.Info("System/template namespace migration started.")
	}

	// One-shot migration (Phase 7 of the SyncEngine extraction): rewrite
	// pre-engine `wiki.connectors.<kind>.subscriptions[]` frontmatter into
	// the new `wiki.connectors.<kind>.bindings[]` shape, with adapter-
	// specific fields collapsed into a per-entry `adapter_state` subtree.
	// The engine's FrontmatterBindingStore reads ONLY the new shape after
	// this migration ships; once every wiki has been booted at least once
	// with the migration enabled, the scan/job pair becomes a no-op and
	// the code can be deleted.
	connectorsMigration := eager.NewConnectorsSubscriptionsToBindingsMigrationScanJob(
		dataDirScanner, s.JobQueueCoordinator, s,
	)
	if err := s.JobQueueCoordinator.EnqueueJob(connectorsMigration); err != nil {
		s.Logger.Error("Failed to enqueue connectors subscriptions->bindings migration job: %v", err)
	} else {
		s.Logger.Info("Connectors subscriptions->bindings migration started.")
	}
}

// Defaults for the agent-schedule machinery when CLI flags do not override
// them. The queue is intentionally generously-sized so that enqueue is
// effectively backpressure rather than a skip trigger.
const (
	defaultAgentScheduleConcurrencyValue = 2
	defaultAgentScheduleQueueCapValue    = 256
	// DefaultAgentTurnHardTimeout is the wall-clock timeout for one scheduled
	// agent turn when no CLI override is supplied.
	DefaultAgentTurnHardTimeout = 10 * time.Minute

	// agentScheduleRefreshConcurrency forces single-worker execution so
	// per-page refreshes run in submission order and never race the cron
	// registrar.
	agentScheduleRefreshConcurrency = 1
	// agentScheduleRefreshQueueCap is generously sized so enqueue acts as
	// backpressure rather than dropping refresh requests on a save burst.
	agentScheduleRefreshQueueCap = 256
)

// InitializeAgentScheduling pre-registers the AgentTurn queue and constructs
// the AgentScheduler. Must be called after CLI flags have populated
// AgentScheduleConcurrency and AgentScheduleQueueCap on the site (the values
// are baked into the queue at registration time).
//
// Safe to call multiple times — the second call returns immediately if the
// scheduler is already constructed.
func (s *Site) InitializeAgentScheduling() {
	if s.AgentScheduler != nil {
		return
	}
	if s.AgentScheduleConcurrency <= 0 {
		s.AgentScheduleConcurrency = defaultAgentScheduleConcurrencyValue
	}
	if s.AgentScheduleQueueCap <= 0 {
		s.AgentScheduleQueueCap = defaultAgentScheduleQueueCapValue
	}
	if s.AgentTurnHardTimeout <= 0 {
		s.AgentTurnHardTimeout = DefaultAgentTurnHardTimeout
	}
	if err := s.JobQueueCoordinator.RegisterQueue(AgentTurnJobName, s.AgentScheduleConcurrency, s.AgentScheduleQueueCap); err != nil {
		s.Logger.Warn("AgentTurn queue pre-registration failed: %v", err)
	}
	// AgentScheduleRefresh runs single-worker so per-page refreshes execute in
	// submission order, avoiding races against the cron registrar when the
	// same page is saved twice in quick succession.
	if err := s.JobQueueCoordinator.RegisterQueue(AgentScheduleRefreshJobName, agentScheduleRefreshConcurrency, agentScheduleRefreshQueueCap); err != nil {
		s.Logger.Warn("AgentScheduleRefresh queue pre-registration failed: %v", err)
	}
	s.AgentScheduler = NewAgentScheduler(
		s.AgentScheduleStore,
		s.ScheduledTurnDispatcher,
		s.FrontmatterIndexQueryer,
		s.CronScheduler,
		s.AgentTurnHardTimeout,
	)
}

// onInitialIndexingComplete is the callback fired by IndexCoordinator after
// the initial bulk-enqueue jobs all finish. It chains the inventory
// normalization job and loads agent schedules now that the frontmatter index
// is fully populated.
func (s *Site) onInitialIndexingComplete() {
	normJob, err := NewInventoryNormalizationJob(s, s.FrontmatterIndexQueryer, s.Logger)
	if err != nil {
		s.Logger.Error("Failed to create inventory normalization job: %v", err)
		return
	}
	if err := s.JobQueueCoordinator.EnqueueJob(normJob); err != nil {
		s.Logger.Error("Failed to enqueue inventory normalization job: %v", err)
	} else {
		s.Logger.Info("Inventory normalization job queued after indexing completed")
	}

	// One-time migration: move ai_agent_chat_context -> agent.chat_context
	// for any page that still has the legacy key. The job is idempotent so
	// running it on every startup is safe; it short-circuits when there is
	// nothing to do.
	migrationJob := NewChatContextMigrationJob(s, s.FrontmatterIndexQueryer)
	if err := s.JobQueueCoordinator.EnqueueJob(migrationJob); err != nil {
		s.Logger.Error("Failed to enqueue chat-context migration job: %v", err)
	}

	// Load and register every page's agent.schedules with the cron scheduler
	// once the frontmatter index is fully populated. Skips silently if
	// InitializeAgentScheduling has not been called yet (e.g. tests).
	if s.AgentScheduler == nil {
		return
	}
	if loadErr := s.AgentScheduler.LoadAll(); loadErr != nil {
		s.Logger.Error("Failed to load agent schedules: %v", loadErr)
	} else {
		s.Logger.Info("Agent schedules loaded into cron scheduler")
	}
}

// InitializeIndexing initializes the site's indexes.
func (s *Site) InitializeIndexing() error {
	frontmatterIndex := frontmatter.NewIndex(s)
	bleveIndex, err := bleve.NewIndex(s, frontmatterIndex)
	if err != nil {
		return fmt.Errorf("failed to create bleve index: %w", err)
	}

	s.FrontmatterIndexQueryer = frontmatterIndex
	s.BleveIndexQueryer = bleveIndex

	// Create new job queue coordinator and index coordinator
	s.JobQueueCoordinator = jobs.NewJobQueueCoordinator(s.Logger)
	s.IndexCoordinator = index.NewIndexCoordinator(s.JobQueueCoordinator, frontmatterIndex, bleveIndex)

	// Create and start cron scheduler for periodic jobs
	s.CronScheduler = jobs.NewCronScheduler(s.Logger)
	s.CronScheduler.Start()

	// Wire up the scheduled-agent infrastructure: dispatcher (server side
	// bridge to the pool), schedule store (typed agent.schedules access), and
	// chat-context store (typed agent.chat_context access). The schedule
	// store's terminal-status hook writes through the chat-context store so
	// every terminal transition records a background-activity entry visible to
	// interactive chat preambles.
	// Queue pre-registration and AgentScheduler construction are deferred to
	// InitializeAgentScheduling so they pick up CLI-supplied concurrency and
	// hard-timeout values.
	s.ScheduledTurnDispatcher = NewScheduledTurnDispatcher()
	s.AgentScheduleStore = NewAgentScheduleStore(s)
	s.AgentChatContextStore = NewAgentChatContextStore(s)
	s.AgentScheduleStore.SetBackgroundActivitySink(s.AgentChatContextStore)

	// Schedule inventory normalization job to run hourly at minute 0
	// This creates pages for items listed in inventory.items that don't have their own pages,
	// and generates an audit report of any inventory anomalies
	_, err = ScheduleInventoryNormalization(s.CronScheduler, s, "0 0 * * * *")
	if err != nil {
		s.Logger.Warn("Failed to schedule inventory normalization job: %v", err)
	} else {
		s.Logger.Info("Inventory normalization job scheduled to run hourly")
	}

	s.startMigrationJobs()

	// Get all files that need to be indexed
	listing, err := s.DirectoryList()
	if err != nil {
		return fmt.Errorf("failed to list pages for indexing: %w", err)
	}
	for _, re := range listing.ReadErrors {
		s.Logger.Error("Skipping page %q from indexing due to read error: %v", re.PageName, re.Err)
	}
	if len(listing.Entries) == 0 {
		s.Logger.Info("No pages found to index.")
		return nil
	}

	// Convert files to page identifiers
	pageIdentifiers := make([]wikipage.PageIdentifier, len(listing.Entries))
	for i, file := range listing.Entries {
		pageIdentifiers[i] = wikipage.PageIdentifier(file.Name())
	}

	// Start background indexing with completion callback to chain the normalization job.
	// Note: The callback executes asynchronously when all indexing jobs complete, not when this
	// function returns. Error handling inside the callback is separate from the outer error check.
	if err := s.IndexCoordinator.BulkEnqueuePagesWithCompletion(pageIdentifiers, index.Add, s.onInitialIndexingComplete); err != nil {
		// This error means the bulk enqueue failed immediately - the callback won't run
		s.Logger.Error("Failed to enqueue bulk indexing jobs: %v", err)
	}
	s.Logger.Info("Background indexing started for %d pages. Application is ready.", len(listing.Entries))

	return nil
}

// InitializeIndexingAndWait initializes indexing and waits for initial indexing to complete.
// This is primarily for testing to ensure all background jobs complete before tests proceed.
func (s *Site) InitializeIndexingAndWait(timeout time.Duration) error {
	if err := s.InitializeIndexing(); err != nil {
		return fmt.Errorf("failed to initialize indexing: %w", err)
	}

	// Wait for all initial indexing jobs to complete
	ctx := context.Background()
	completed, timedOut := s.IndexCoordinator.WaitForCompletionWithTimeout(ctx, timeout)
	if timedOut {
		return fmt.Errorf("timed out waiting for initial indexing to complete after %v", timeout)
	}
	if !completed {
		return errors.New("initial indexing was cancelled or failed")
	}

	return nil
}

// --- Site methods moved from page.go ---

// ensureStore lazy-initializes the pagestore.Store (and matching reader
// decorator) on first use so tests that construct Site directly (without
// going through NewSite) don't need to wire the store manually. Also
// re-creates the store if PathToData has been mutated (some tests reassign
// it to point at a read-only directory to exercise save-error paths) —
// without this check, the store would keep writing to the original path.
//
// Production code path is initialized in NewSite and PathToData is never
// mutated, so the re-creation branch is test-only.
func (s *Site) ensureStore() *pagestore.Store {
	s.storeInitMu.Lock()
	defer s.storeInitMu.Unlock()

	if s.store == nil || s.store.PathToData() != s.PathToData {
		s.store = pagestore.NewStore(s.PathToData)
		canon := canonicalize.NewFormatCanonicalizer()
		s.store.SetCanonicalizer(canon)
		s.reader = pagestore.NewCanonicalReader(canon, s.store)
	}
	if s.reader == nil {
		canon := canonicalize.NewFormatCanonicalizer()
		s.store.SetCanonicalizer(canon)
		s.reader = pagestore.NewCanonicalReader(canon, s.store)
	}
	return s.store
}

// ReadPage opens a page by its identifier. Delegates entirely to the
// CanonicalReader decorator (which wraps *pagestore.Store), so callers
// always receive canonical bytes regardless of on-disk state. No
// save-on-read side effect — the bug class this refactor eliminates is
// gone from the call graph.
func (s *Site) ReadPage(requestedIdentifier wikipage.PageIdentifier) (*wikipage.Page, error) {
	s.ensureStore()
	p, err := s.reader.ReadPage(requestedIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to read page %s: %w", requestedIdentifier, err)
	}
	return p, nil
}

// readOrInitPage opens a page or initializes a new one if it doesn't exist.
// Returns an error if page initialization fails to save.
func (s *Site) readOrInitPage(requestedIdentifier string, req *http.Request) (*wikipage.Page, error) {
	p, err := s.ReadPage(wikipage.PageIdentifier(requestedIdentifier))
	if err != nil {
		return nil, fmt.Errorf(failedToOpenPageErrFmt, requestedIdentifier, err)
	}

	if p.IsNew() {
		if err := s.initNewPage(p, req); err != nil {
			return nil, err
		}
	}

	if renderErr := p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer); renderErr != nil {
		s.Logger.Error("Error rendering page: %v", renderErr)
	}
	return p, nil
}

// initNewPage initializes a newly created page with frontmatter and template content.
func (s *Site) initNewPage(p *wikipage.Page, req *http.Request) error {
	prams := req.URL.Query()
	tmpl := prams.Get("tmpl")

	fm, err := BuildFrontmatterFromURLParams(p.Identifier, prams)
	if err != nil {
		return fmt.Errorf("failed to build frontmatter from URL params: %w", err)
	}

	if tmpl == "inv_item" {
		EnsureInventoryFrontmatterStructure(fm)
	}

	initialText, err := buildInitialPageText(fm, tmpl)
	if err != nil {
		return err
	}

	p.Text = initialText
	if renderErr := p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer); renderErr != nil {
		s.Logger.Error("Error rendering new page: %v", renderErr)
	}
	if err := s.savePageAndIndex(p); err != nil {
		s.Logger.Error("Failed to save new page '%s': %v", p.Identifier, err)
		return fmt.Errorf("failed to save new page '%s': %w", p.Identifier, err)
	}
	return nil
}

// buildInitialPageText constructs the initial markdown content for a new page.
func buildInitialPageText(fm map[string]any, tmpl string) (string, error) {
	fmBytes, err := toml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter to TOML: %w", err)
	}

	initialText := ""
	if len(fmBytes) > 0 {
		initialText = tomlDelimiter + string(fmBytes)
		if !bytes.HasSuffix(fmBytes, []byte(newline)) {
			initialText += newline
		}
		initialText += tomlDelimiter
	}

	initialText += `
# {{or .Title .Identifier}}
`
	if tmpl == "inv_item" {
		initialText += `
{{if IsContainer .Identifier }}
## Contents
{{ ShowInventoryContentsOf .Identifier }}
{{ end }}
`
	}
	return initialText, nil
}

// DirectoryEntry represents an entry in the wiki directory.
type DirectoryEntry struct {
	Path       string
	Length     int
	LastEdited time.Time
}

// Name returns the name of the directory entry.
func (d DirectoryEntry) Name() string {
	return d.Path
}

// Size returns the size of the directory entry.
func (d DirectoryEntry) Size() int64 {
	return int64(d.Length)
}

// Mode returns the file mode of the directory entry.
func (DirectoryEntry) Mode() os.FileMode {
	return os.ModePerm
}

// ModTime returns the modification time of the directory entry.
func (d DirectoryEntry) ModTime() time.Time {
	return d.LastEdited
}

// IsDir returns true if the directory entry is a directory.
func (DirectoryEntry) IsDir() bool {
	return false
}

// Sys returns the underlying data source of the directory entry.
func (DirectoryEntry) Sys() any {
	return nil
}

// PageReadError records a page identifier and the error encountered when reading that page during directory listing.
type PageReadError struct {
	PageName string
	Err      error
}

// DirectoryListing holds the result of a directory listing operation, including
// successfully read page entries and any per-page read errors collected during iteration.
type DirectoryListing struct {
	Entries    []os.FileInfo
	ReadErrors []PageReadError
}

// DirectoryList returns a listing of all wiki pages in the data directory.
// It returns a non-nil error only if the data directory itself cannot be read (e.g., directory is missing
// or unreadable). Individual page read failures are collected in the returned DirectoryListing.ReadErrors
// slice so that callers can inform the user which pages could not be loaded without aborting the entire listing.
func (s *Site) DirectoryList() (DirectoryListing, error) {
	files, err := os.ReadDir(s.PathToData)
	if err != nil {
		return DirectoryListing{}, fmt.Errorf("failed to read data directory: %w", err)
	}
	entries := make([]os.FileInfo, 0, len(files))
	var readErrors []PageReadError
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".md") {
			name := decodeFileName(f.Name())
			// Each ReadPage() call will acquire its own read lock
			p, err := s.ReadPage(wikipage.PageIdentifier(name))
			if err != nil {
				s.Logger.Error("Failed to read page %q for directory listing: %v", name, err)
				readErrors = append(readErrors, PageReadError{PageName: name, Err: err})
				continue
			}

			// Get file modification time from filesystem
			fileInfo, statErr := os.Stat(filepath.Join(s.PathToData, f.Name()))
			lastEdited := time.Now()
			if statErr == nil {
				lastEdited = fileInfo.ModTime()
			}

			entries = append(entries, DirectoryEntry{
				Path:       p.Identifier, // Use the actual Page.Identifier, not the decoded filename
				Length:     len(p.Text),
				LastEdited: lastEdited,
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ModTime().Before(entries[j].ModTime()) })
	return DirectoryListing{Entries: entries, ReadErrors: readErrors}, nil
}

// UploadEntry represents an uploaded file entry.
type UploadEntry struct {
	os.FileInfo
}

// UploadList returns a list of all uploaded files in the data directory.
func (s *Site) UploadList() ([]os.FileInfo, error) {
	paths, err := filepath.Glob(path.Join(s.PathToData, "sha256*"))
	if err != nil {
		return nil, err
	}
	result := make([]os.FileInfo, len(paths))
	for i := range paths {
		result[i], err = os.Stat(paths[i])
		if err != nil {
			return result, err
		}
	}
	return result, nil
}

// --- PageReaderMutator implementation ---

// modifyOrCreatePage atomically reads, modifies, and writes a page's full raw text
// through `*pagestore.Store.ModifyOrCreatePage`. The page-lock is held for the
// entire read-modify-write cycle; async indexing jobs are enqueued only after
// the lock has been released.
func (s *Site) modifyOrCreatePage(identifierStr string, modifier func(currentText string) (string, error)) error {
	if err := s.ensureStore().ModifyOrCreatePage(identifierStr, modifier); err != nil {
		return err
	}

	// Enqueue async indexing jobs after the lock has been released by the store.
	identifier := wikipage.PageIdentifier(identifierStr)
	if s.IndexCoordinator != nil {
		if err := s.IndexCoordinator.EnqueueIndexJob(identifier, index.Add); err != nil {
			s.Logger.Error("Failed to enqueue index job for %s: %v", identifierStr, err)
		}
	}
	if s.JobQueueCoordinator != nil {
		normJob := NewPageInventoryNormalizationJob(identifier, s, s.Logger)
		if err := s.JobQueueCoordinator.EnqueueJob(normJob); err != nil {
			s.Logger.Error("Failed to enqueue per-page inventory normalization job for %s: %v", identifierStr, err)
		}
	}

	// Reconcile any agent.schedules that the user may have edited so cron
	// pickups happen within seconds rather than at next restart. Enqueued as a
	// single-worker job so refreshes run in submission order through the same
	// machinery as every other background unit of work.
	if s.AgentScheduler != nil && s.JobQueueCoordinator != nil {
		refreshJob := NewAgentScheduleRefreshJob(s.AgentScheduler, identifierStr)
		if err := s.JobQueueCoordinator.EnqueueJob(refreshJob); err != nil {
			s.Logger.Error("Failed to enqueue agent schedule refresh for %s: %v", identifierStr, err)
		}
	}

	return nil
}

// ModifyMarkdown atomically reads the markdown section, calls modifier, and writes the result
// back while preserving the existing frontmatter. The entire cycle is held under the write lock.
func (s *Site) ModifyMarkdown(identifier wikipage.PageIdentifier, modifier func(wikipage.Markdown) (wikipage.Markdown, error)) error {
	return s.modifyOrCreatePage(string(identifier), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}

		currentMD, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for modification: %w", err)
		}

		newMD, err := modifier(currentMD)
		if err != nil {
			return "", err
		}

		currentFM, err := p.GetFrontMatter()
		if err != nil {
			return "", fmt.Errorf("failed to parse frontmatter during markdown modification: %w", err)
		}

		return wikipage.CombineFrontMatterAndMarkdown(currentFM, newMD)
	})
}

// ModifyFrontMatterAndMarkdown atomically reads both page sections, calls modifier,
// and writes both returned sections under a single page lock.
func (s *Site) ModifyFrontMatterAndMarkdown(identifier wikipage.PageIdentifier, modifier func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error)) error {
	return s.modifyOrCreatePage(string(identifier), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}

		currentFM, err := p.GetFrontMatter()
		if err != nil {
			return "", fmt.Errorf("failed to parse frontmatter for page modification: %w", err)
		}
		currentMD, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for page modification: %w", err)
		}

		newFM, newMD, err := modifier(currentFM, currentMD)
		if err != nil {
			return "", err
		}
		return wikipage.CombineFrontMatterAndMarkdown(newFM, newMD)
	})
}

// WriteFrontMatter atomically reads the current markdown, combines it with the new frontmatter,
// and writes the result — all under a single write lock to prevent concurrent write races.
func (s *Site) WriteFrontMatter(identifier wikipage.PageIdentifier, fm wikipage.FrontMatter) error {
	return s.modifyOrCreatePage(string(identifier), func(currentText string) (string, error) {
		p := &wikipage.Page{Text: currentText}

		md, err := p.GetMarkdown()
		if err != nil {
			return "", fmt.Errorf("failed to parse markdown for frontmatter write: %w", err)
		}

		return wikipage.CombineFrontMatterAndMarkdown(fm, md)
	})
}

// WriteMarkdown atomically reads the current frontmatter, combines it with the new markdown,
// and writes the result — all under a single write lock to prevent concurrent write races.
func (s *Site) WriteMarkdown(identifier wikipage.PageIdentifier, md wikipage.Markdown) error {
	return s.ModifyMarkdown(identifier, func(_ wikipage.Markdown) (wikipage.Markdown, error) {
		return md, nil
	})
}

// ReadFrontMatter reads the frontmatter for a page.
func (s *Site) ReadFrontMatter(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.FrontMatter, error) {
	page, err := s.ReadPage(identifier)
	if err != nil {
		return identifier, nil, fmt.Errorf(failedToOpenPageErrFmt, identifier, err)
	}
	if page.IsNew() {
		return identifier, nil, os.ErrNotExist
	}
	matter, err := page.GetFrontMatter()
	if err != nil {
		return identifier, nil, err
	}
	return identifier, matter, nil
}

// ReadMarkdown reads the markdown content for a page.
func (s *Site) ReadMarkdown(identifier wikipage.PageIdentifier) (wikipage.PageIdentifier, wikipage.Markdown, error) {
	page, err := s.ReadPage(identifier)
	if err != nil {
		return identifier, "", fmt.Errorf(failedToOpenPageErrFmt, identifier, err)
	}
	if page.IsNew() {
		return identifier, "", os.ErrNotExist
	}
	markdown, err := page.GetMarkdown()
	if err != nil {
		return identifier, "", err
	}
	return identifier, markdown, nil
}

// DeletePage deletes a page from disk via the pagestore's soft-delete (move
// to __deleted__/<timestamp>/). Cron registrations and index entries are
// cleaned up here on Site; the disk operation itself is the store's
// responsibility.
func (s *Site) DeletePage(identifier wikipage.PageIdentifier) error {
	s.Logger.Trace("Deleting page %s", identifier)

	// Drop any agent.schedules cron registrations for this page BEFORE the
	// file is moved out of place. Otherwise the cron keeps firing for a
	// non-existent page until restart. Done synchronously (not via the
	// AgentScheduleRefreshJob queue) because the file move below may be
	// observed by an in-flight cron fire microseconds later — we want the
	// registration gone before that window opens.
	if s.AgentScheduler != nil {
		s.AgentScheduler.UnregisterPage(string(identifier))
	}

	// Enqueue removal jobs for both frontmatter and bleve indexes
	if s.IndexCoordinator != nil {
		if err := s.IndexCoordinator.EnqueueIndexJob(identifier, index.Remove); err != nil {
			s.Logger.Error("Failed to enqueue index removal job for %s: %v", identifier, err)
		}
	}

	return s.ensureStore().SoftDeletePage(identifier)
}

// UpdatePageContent updates a page's full content, rendering and saving.
// Canonicalization of `newText` happens inside the pagestore.Store on the
// write path (see Store.writeRawTextLocked) — UpdatePageContent no longer
// runs the migration applicator itself.
func (s *Site) UpdatePageContent(identifier wikipage.PageIdentifier, newText string) error {
	p, err := s.ReadPage(identifier)
	if err != nil {
		return fmt.Errorf("failed to open page %s for update: %w", identifier, err)
	}

	// Update the text content
	p.Text = newText

	// Render the new page
	if renderErr := p.Render(s, s.MarkdownRenderer, TemplateExecutor{}, s.FrontmatterIndexQueryer); renderErr != nil {
		s.Logger.Error("Error rendering page: %v", renderErr)
	}

	// Save to disk; the store canonicalizes on the write path.
	return s.savePageAndIndex(p)
}

// savePageAndIndex handles the low-level persistence of a page to disk
func (s *Site) savePageAndIndex(p *wikipage.Page) error {
	err := s.savePage(p)
	if err != nil {
		return fmt.Errorf("failed to save page and index: %w", err)
	}

	// Enqueue indexing jobs for both frontmatter and bleve indexes
	if s.IndexCoordinator != nil {
		if err := s.IndexCoordinator.EnqueueIndexJob(wikipage.PageIdentifier(p.Identifier), index.Add); err != nil {
			s.Logger.Error("Failed to enqueue index job for %s: %v", p.Identifier, err)
		}
	}

	// Enqueue per-page inventory normalization job
	if s.JobQueueCoordinator != nil {
		normJob := NewPageInventoryNormalizationJob(wikipage.PageIdentifier(p.Identifier), s, s.Logger)
		if err := s.JobQueueCoordinator.EnqueueJob(normJob); err != nil {
			s.Logger.Error("Failed to enqueue per-page inventory normalization job for %s: %v", p.Identifier, err)
		}
	}

	return nil
}

// savePage saves a page to disk without triggering indexing.
// This is used by migrations to avoid circular references during read operations.
//
// Delegates the disk write to `*pagestore.Store.ModifyOrCreatePage` so the
// page-lock is shared with all other I/O. Bypassing the store here would
// re-introduce the dual-lock race the refactor exists to eliminate.
func (s *Site) savePage(p *wikipage.Page) error {
	// Use store's ModifyOrCreatePage with a constant-returning modifier so the
	// store's per-page lock protects the write; the read side of the
	// read-modify-write is wasted work but tiny compared to the write IO.
	if err := s.ensureStore().ModifyOrCreatePage(p.Identifier, func(_ string) (string, error) {
		return p.Text, nil
	}); err != nil {
		return fmt.Errorf("failed to save page %s: %w", p.Identifier, err)
	}
	return nil
}

// GetJobQueueCoordinator returns the job queue coordinator for progress monitoring.
func (s *Site) GetJobQueueCoordinator() *jobs.JobQueueCoordinator {
	return s.JobQueueCoordinator
}

// ReadOrInitPageForTesting exposes the readOrInitPage functionality for testing.
// This should only be used in tests.
func (s *Site) ReadOrInitPageForTesting(requestedIdentifier string, req *http.Request) (*wikipage.Page, error) {
	return s.readOrInitPage(requestedIdentifier, req)
}

// Utility functions for working with pages

// decodeFileName decodes a filename from base32.
func decodeFileName(s string) string {
	s2, _ := base32tools.DecodeFromBase32(strings.Split(s, ".")[0])
	return s2
}
