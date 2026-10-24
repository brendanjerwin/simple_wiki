package v1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// page app source constants.
const (
	// PageAppLangOpenuiLang is the only page app language in this scope.
	PageAppLangOpenuiLang = "openui-lang"
	// PageAppLibrary is the wiki component library version the shipped
	// help page teaches.
	PageAppLibrary = "wiki-core@1"
)

const (
	pageAppErrPageRequired = "page is required"
	pageAppErrReadFailed   = "failed to read page: %v"
	pageAppHundredPercent  = 100
	pageAppWikiRootKey     = "wiki"
	mdNewline               = "\n"
)

// page app program limits. The Go validator's floor: reject gross
// violations, accept anything the renderer could conceivably render
// (authoritative parsing is browser-side lang-core — see ADR-0017).
const (
	pageAppMaxSourceBytes = 1 << 16 // 64 KiB
)

// pageAppLangRootRe matches the required root-assignment statement.
// In OpenUI Lang every statement is `identifier = Expression` and `root`
// names the entry point (the generated system prompt teaches "always
// start with root = <RootName>(...)"). A program without a root
// assignment renders nothing — that is the gross violation this
// conservative validator rejects.
var pageAppLangRootRe = regexp.MustCompile(`(?m)^\s*root\s*=`)

// pageAppLangAssignRe matches the general statement shape so partially
// renderable programs (statements beyond the root) are recognized.
var pageAppLangAssignRe = regexp.MustCompile(`(?m)^\s*[A-Za-z][A-Za-z0-9_]*\s*=`)

// validatePageAppSource is the conservative Go Lang validator. The
// authoritative parse happens browser-side (lang-core streaming parser,
// which drops unknown components and invalid statements gracefully); this
// validator exists only to never land a program the renderer will choke
// on. It must not false-positive on partial-but-renderable programs:
// gross violations reject (InvalidArgument); everything else accepts.
func validatePageAppSource(source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return status.Error(codes.InvalidArgument, "page app source is empty")
	}
	if len(source) > pageAppMaxSourceBytes {
		return status.Errorf(codes.InvalidArgument, "page app source is %d bytes; the cap is %d bytes", len(source), pageAppMaxSourceBytes)
	}

	if !pageAppLangRootRe.MatchString(source) {
		return status.Errorf(codes.InvalidArgument,
			"page app source has no `root =` entry-point statement; every OpenUI Lang program must define root (e.g. `root = Card(...)`) — see the page help-page apps-lang (api_v1_PageManagementService_ReadPage)")
	}
	if !pageAppLangAssignRe.MatchString(source) {
		return status.Errorf(codes.InvalidArgument,
			"page app source has no `identifier =` statements; OpenUI Lang programs are lists of assignments — see the page help-page apps-lang (api_v1_PageManagementService_ReadPage)")
	}

	// A program containing its own ``` terminator would corrupt the
	// written fence; SetPageApp callers send raw source (the wiki wraps
	// it). Reject that.
	if strings.Contains(source, "```") {
		return status.Error(codes.InvalidArgument, "page app source must not contain a ``` fence terminator; the wiki wraps the source itself")
	}

	return nil
}

// pageAppVersionHash computes the sha256 OCC handle for a page app
// program, prefixed like the page-version hashes for a consistent
// mental model.
func pageAppVersionHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// watcherRegistry is a tiny page-keyed pub-sub shared by WatchPageApp and
// WatchState. Each subscriber gets events for its page until its stream
// errors; the publisher never blocks (events enqueue up to the
// per-subscriber buffer, then the drop is covered by the next full replay
// since every event carries the current source/state).
type watcherRegistry[T any] struct {
	mu   sync.Mutex
	subs map[wikipage.PageIdentifier]map[chan T]struct{}
}

func newWatcherRegistry[T any]() *watcherRegistry[T] {
	return &watcherRegistry[T]{subs: make(map[wikipage.PageIdentifier]map[chan T]struct{})}
}

// subscribe returns a buffered channel that will receive future events
// for the given page only.
func (r *watcherRegistry[T]) subscribe(page wikipage.PageIdentifier) chan T {
	ch := make(chan T, 32)
	r.mu.Lock()
	pageSubs, ok := r.subs[page]
	if !ok {
		pageSubs = make(map[chan T]struct{})
		r.subs[page] = pageSubs
	}
	pageSubs[ch] = struct{}{}
	r.mu.Unlock()
	return ch
}

// unsubscribe removes a subscriber (called when its stream ends).
func (r *watcherRegistry[T]) unsubscribe(page wikipage.PageIdentifier, ch chan T) {
	r.mu.Lock()
	if pageSubs, ok := r.subs[page]; ok {
		delete(pageSubs, ch)
		if len(pageSubs) == 0 {
			delete(r.subs, page)
		}
	}
	r.mu.Unlock()
}

// publish delivers an event to every current subscriber of the page
// without blocking.
func (r *watcherRegistry[T]) publish(page wikipage.PageIdentifier, event T) {
	r.mu.Lock()
	var subs []chan T
	if pageSubs, ok := r.subs[page]; ok {
		subs = make([]chan T, 0, len(pageSubs))
		for ch := range pageSubs {
			subs = append(subs, ch)
		}
	}
	r.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- event:
		default:
			// Slow subscriber: drop the event; the next event replays the
			// full current state anyway (every event carries it).
		}
	}
}

// pageAppMutator owns every mutation of a page's page app program. It
// serializes concurrent Set/Clear on the same page (the page-lock the
// page store holds inside ModifyMarkdown covers the frontmatter read-
// modify-write; this mutex prevents two RPCs from interleaving their
// read-validate-stamp phases).
type pageAppMutator struct {
	pages      wikipage.PageReaderMutator
	stateWatch *watcherRegistry[*apiv1.WatchPageAppStateResponse]
	progWatch  *watcherRegistry[*apiv1.WatchPageAppResponse]
	mu         sync.Mutex
}

// splitPageAppBody returns (pageAppSource, proseRemainder, found) for
// a page's markdown.
func splitPageAppBody(markdown string) (source string, prose string, found bool) {
	return wikipage.SplitPageAppFences(markdown)
}

// getPageAppStamps reads the wiki.page_app frontmatter subtree.
func getPageAppStamps(fm wikipage.FrontMatter) (map[string]any, bool) {
	wiki, ok := fm[pageAppWikiRootKey].(map[string]any)
	if !ok {
		return nil, false
	}
	pageApp, ok := wiki["page_app"].(map[string]any)
	if !ok {
		return nil, false
	}
	return pageApp, true
}

// getCurrentPageApp reads the current page app (source + stamps) from a
// page. found=false when the page exists but has no page app.
func (a *pageAppMutator) getCurrentPageApp(page wikipage.PageIdentifier) (source string, stamps map[string]any, found bool, err error) {
	_, markdown, err := a.pages.ReadMarkdown(page)
	if err != nil {
		return "", nil, false, err
	}
	source, _, found = splitPageAppBody(string(markdown))
	if !found {
		return "", nil, false, nil
	}
	_, fm, err := a.pages.ReadFrontMatter(page)
	if err != nil {
		return "", nil, false, err
	}
	stamps, _ = getPageAppStamps(fm)
	return source, stamps, found, nil
}

// requirePageAppAccess applies the page-level guards every page app RPC
// shares: user-mutable (not a system page) and wiki.authorization ACL.
func (a *pageAppMutator) requirePageAppAccess(ctx context.Context, page wikipage.PageIdentifier) error {
	if err := requireUserMutable(a.pages, page); err != nil {
		return err
	}
	return requireAuthorized(ctx, a.pages, page)
}

// newPageAppMutator constructs the page app funnel over a page store.
func newPageAppMutator(pages wikipage.PageReaderMutator) *pageAppMutator {
	return &pageAppMutator{
		pages:      pages,
		progWatch:  newWatcherRegistry[*apiv1.WatchPageAppResponse](),
		stateWatch: newWatcherRegistry[*apiv1.WatchPageAppStateResponse](),
	}
}

// getPageAppMutator returns the server's page app funnel, constructing a
// default over the page store when none was wired. Construction is
// guarded by a Once so concurrent first calls (an agent's SetPageApp and
// a browser's WatchPageApp landing together) never race the field write.
func (s *Server) getPageAppMutator() (*pageAppMutator, error) {
	s.pageAppMutatorOnce.Do(func() {
		if s.pageReaderMutator != nil {
			s.pageAppMutator = newPageAppMutator(s.pageReaderMutator)
		}
	})
	if s.pageAppMutator == nil {
		return nil, status.Error(codes.Internal, "page app services not configured: no page store")
	}
	return s.pageAppMutator, nil
}

// validateChangedPageAppFence is the edit-path page app funnel
// (criterion 6): a write (UpdatePageContent / UpdateWholePage / CreatePage)
// whose new body carries a wiki-page-app fence that is *new or changed*
// passes through the same validation + placement-guard gate as
// SetPageApp, so no route can land an unvalidated fence. An unchanged
// fence (ordinary prose edit on a page app page) skips re-validation —
// the program was validated when written.
func validateChangedPageAppFence(_ wikipage.FrontMatter, currentMarkdown wikipage.Markdown, newWholeMarkdown string) error {
	oldSource, _, oldFound := splitPageAppBody(string(currentMarkdown))
	newSource, _, newFound := splitPageAppBody(newWholeMarkdown)

	if !newFound {
		return nil // no fence in the new body: nothing to validate
	}

	if oldFound && oldSource == newSource {
		// Unchanged program: ordinary prose edit; the placement guard
		// still applies to the prose the write would remove.
		return nil
	}

	if err := validatePageAppSource(newSource); err != nil {
		return err
	}

	// The candidate body IS the write's resulting body (the caller supplies
	// the whole markdown): the guard compares current prose vs resulting
	// prose after fence removal. Edit-path writes have no confirm field;
	// the error names SetPageApp (which has one).
	return checkPageAppProseGuard(string(currentMarkdown), newWholeMarkdown)
}

// GetPageApp implements GetPageApp.
func (s *Server) GetPageApp(ctx context.Context, req *apiv1.GetPageAppRequest) (*apiv1.GetPageAppResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}

	source, stamps, found, err := mut.getCurrentPageApp(page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page %q not found", req.GetPage())
		}
		return nil, status.Errorf(codes.Internal, pageAppErrReadFailed, err)
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "page %q has no page app", req.GetPage())
	}

	resp := &apiv1.GetPageAppResponse{
		Source:  source,
		Lang:    stampString(stamps, "lang", PageAppLangOpenuiLang),
		Version: stampInt(stamps, "version", 0),
		Library: stampString(stamps, "library", PageAppLibrary),
	}
	if hash, ok := stamps["version_hash"].(string); ok {
		resp.VersionHash = hash
	}
	if t, ok := readStampTime(stamps, "updated_at"); ok {
		resp.UpdatedAt = timestamppb.New(t)
	}
	if by, ok := stamps["updated_by"].(string); ok {
		resp.UpdatedBy = by
	}
	if b, ok := stamps["automated"].(bool); ok {
		resp.Automated = b
	}
	return resp, nil
}

// readStampTime reads an updated_at stamp: RFC3339 string (current
// encoding) or a legacy time.Time (pages written before the string
// encoding).
func readStampTime(stamps map[string]any, key string) (time.Time, bool) {
	switch v := stamps[key].(type) {
	case string:
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return time.Time{}, false
		}
		return t, true
	case time.Time:
		return v, true
	default:
		return time.Time{}, false
	}
}

// stampString reads a string stamp with a default.
func stampString(stamps map[string]any, key, fallback string) string {
	if v, ok := stamps[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

// stampInt reads an int64 stamp from TOML-decoded frontmatter (which
// decodes integers as int64, but older writers may have produced int).
func stampInt(stamps map[string]any, key string, fallback int64) int64 {
	switch v := stamps[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return fallback
	}
}

// SetPageApp implements the single program write door: validate, guard,
// stamp, persist, publish.
func (s *Server) SetPageApp(ctx context.Context, req *apiv1.SetPageAppRequest) (*apiv1.SetPageAppResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())

	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}
	if err := validatePageAppSource(req.GetSource()); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	identity := tailscale.IdentityFromContext(ctx)

	var result *apiv1.SetPageAppResponse
	modifyErr := mutatePageMarkdown(mut.pages, page, identity, func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		newFM, newMD, rpcResponse, err := applySetPageApp(currentFM, currentMarkdown, req, identity)
		if err != nil {
			return nil, "", err
		}
		result = rpcResponse
		return newFM, newMD, nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}
	// The UPDATED watch event is fully derivable from the RPC response.
	if result != nil {
		s.pageAppMutator.progWatch.publish(page, &apiv1.WatchPageAppResponse{
			Kind:        apiv1.WatchPageAppResponse_UPDATED,
			Source:      req.GetSource(),
			Version:     result.GetVersion(),
			VersionHash: result.GetVersionHash(),
		})
	}
	return result, nil
}

// applySetPageApp is SetPageApp's mutation body: OCC check, placement
// guard, stamping, fence rewrite. Reads only its arguments so the atomic
// modifier drives it. (Plain function — no receiver state involved.)
func applySetPageApp(
	currentFM wikipage.FrontMatter,
	currentMarkdown wikipage.Markdown,
	req *apiv1.SetPageAppRequest,
	identity tailscale.IdentityValue,
) (wikipage.FrontMatter, wikipage.Markdown, *apiv1.SetPageAppResponse, error) {
	_, _, existingFound := splitPageAppBody(string(currentMarkdown))
	stamps, stampsFound := getPageAppStamps(currentFM)
	if !stampsFound {
		stamps = map[string]any{}
	}

	// OCC: enforced when expected_version_hash is non-empty. On first
	// write an empty hash is accepted; on replacement an omitted hash is
	// last-writer-wins (the help page + proto doc teach supplying the
	// hash from GetPageApp so conflicts surface instead of overwriting).
	if existingFound {
		fenceContent, _, _ := splitPageAppBody(string(currentMarkdown))
		currentHash := pageAppVersionHash(fenceContent)
		if req.GetExpectedVersionHash() != "" && req.GetExpectedVersionHash() != currentHash {
			return nil, "", nil, status.Errorf(codes.Aborted,
				"page app version mismatch: page %q's page app was modified since last read; re-run GetPageApp and retry", req.GetPage())
		}
	} else if req.GetExpectedVersionHash() != "" {
		return nil, "", nil, status.Errorf(codes.Aborted,
			"page app version mismatch: page %q has no page app but expected_version_hash was set; retry without it", req.GetPage())
	}

	storedSource := strings.TrimSuffix(req.GetSource(), mdNewline)

	// The exact body this write will produce. Default: composition
	// (fence installed above the preserved prose). With
	// confirm_replace_content=true: the app-standalone conversion.
	var candidateBody string
	if req.GetConfirmReplaceContent() {
		candidateBody = replacePageAppFence("", storedSource)
	} else {
		candidateBody = replacePageAppFence(string(currentMarkdown), storedSource)
	}
	if guardErr := pageAppProseGuard(req.GetConfirmReplaceContent())(string(currentMarkdown), candidateBody); guardErr != nil {
		return nil, "", nil, guardErr
	}

	nextVersion := stampInt(stamps, "version", 0) + 1
	newHash := pageAppVersionHash(storedSource)

	stamps["lang"] = PageAppLangOpenuiLang
	stamps["version"] = nextVersion
	stamps["version_hash"] = newHash
	stamps["library"] = PageAppLibrary
	// RFC3339 string, not time.Time: the frontmatter indexer rejects
	// non-scalar values (a time.Time left whole pages unindexed — see e2e).
	stamps["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	if identity != nil && !identity.IsAnonymous() {
		stamps["updated_by"] = identity.LoginName()
		stamps["automated"] = identity.IsAgent()
	}

	return setWikiSubtree(currentFM, "page_app", stamps),
		wikipage.Markdown(candidateBody),
		&apiv1.SetPageAppResponse{
			Version:     nextVersion,
			VersionHash: newHash,
			Library:     PageAppLibrary,
		},
		nil
}

// ClearPageApp removes the fence and wiki.page_app subtree; prose is
// untouched.
func (s *Server) ClearPageApp(ctx context.Context, req *apiv1.ClearPageAppRequest) (*apiv1.ClearPageAppResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}

	page := wikipage.PageIdentifier(req.GetPage())
	mut, mutErr := s.getPageAppMutator()
	if mutErr != nil {
		return nil, mutErr
	}

	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	var publishEvent *apiv1.WatchPageAppResponse
	modifyErr := mutatePageMarkdown(mut.pages, page, tailscale.IdentityFromContext(ctx), func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		_, _, existingFound := splitPageAppBody(string(currentMarkdown))
		if !existingFound {
			return nil, "", status.Errorf(codes.NotFound, "page %q has no page app", req.GetPage())
		}

		fenceContent, _, _ := splitPageAppBody(string(currentMarkdown))
		currentHash := pageAppVersionHash(fenceContent)
		if req.GetExpectedVersionHash() != "" && req.GetExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"page app version mismatch: page %q's page app was modified since last read; re-run GetPageApp and retry", req.GetPage())
		}

		newFM := removeWikiSubtree(currentFM, "page_app")
		newMD := stripPageAppFences(string(currentMarkdown))

		publishEvent = &apiv1.WatchPageAppResponse{
			Kind:        apiv1.WatchPageAppResponse_CLEARED,
			VersionHash: currentHash,
		}
		return newFM, wikipage.Markdown(newMD), nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}

	s.pageAppMutator.progWatch.publish(page, publishEvent)
	return &apiv1.ClearPageAppResponse{}, nil
}

// WatchPageApp implements the server-streaming hot-swap driver.
func (s *Server) WatchPageApp(req *apiv1.WatchPageAppRequest, stream apiv1.PageAppService_WatchPageAppServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}

	page := wikipage.PageIdentifier(req.GetPage())
	mut, mutErr := s.getPageAppMutator()
	if mutErr != nil {
		return mutErr
	}

	if err := mut.requirePageAppAccess(stream.Context(), page); err != nil {
		return err
	}

	ch := mut.progWatch.subscribe(page)
	defer mut.progWatch.unsubscribe(page, ch)

	// Subscribe BEFORE the snapshot: an accepted SetPageApp landing
	// between the snapshot read and subscription would otherwise be
	// silently lost. The version-hash guard drops the redundant replay.
	source, stamps, found, err := mut.getCurrentPageApp(page)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "failed to read current page app: %v", err)
	}
	var lastHash string
	if found {
		lastHash = stampString(stamps, "version_hash", "")
		if err := stream.Send(&apiv1.WatchPageAppResponse{
			Kind:        apiv1.WatchPageAppResponse_INITIAL,
			Source:      source,
			Version:     stampInt(stamps, "version", 0),
			VersionHash: lastHash,
		}); err != nil {
			return err
		}
	}

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event := <-ch:
			// Dedupe only replays of the same program state; CLEARED always
			// passes (its empty source is a real state change, not a replay
			// of the last program hash).
			if event.GetKind() != apiv1.WatchPageAppResponse_CLEARED &&
				event.GetVersionHash() != "" && event.GetVersionHash() == lastHash {
				continue
			}
			lastHash = event.GetVersionHash()
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// ---------------------------------------------------------------------------
// state service

// ---------------------------------------------------------------------------
// write-path helpers

// mutatePageMarkdown runs fn atomically against a page's frontmatter +
// markdown via the same pageAtomicModifier the page-management service
// uses; the page-store write lock covers the read-modify-write. The
// identity parameter drives ADR-0016 history attribution (author,
// is_agent) — page app writes must record who wrote them.
func mutatePageMarkdown(pages wikipage.PageReaderMutator, page wikipage.PageIdentifier, identity wikipage.Identity, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error)) error {
	atomicMod, amErr := requirePageAtomicModifier(pages)
	if amErr != nil {
		return amErr
	}
	return atomicMod.ModifyFrontMatterAndMarkdown(page, fn, identity)
}

// checkPageAppProseGuard is the placement guard (mechanical backstop for
// the composition model): a write that drops more than 10% of the page's
// substantial prose (existing non-fence body ≥1KB) requires explicit
// confirmation. SetPageApp's default path composes (keeps prose) so it
// passes vacuously; the app-standalone conversion sets
// confirm_replace_content=true; edit-path writes (UpdatePageContent /
// UpdateWholePage / CreatePage) have no confirm field, so a
// changed-fence write that clears substantial prose fails with
// FailedPrecondition naming the alternatives.
// pageAppAllowsClear is the confirmed-write guard: the caller has already
// checked confirm_replace_content, so this yields unconditionally (the
// deliberate app-standalone conversion).
func pageAppAllowsClear(_, _ string) error {
	return nil
}

// pageAppProseGuard returns the guard variant for a SetPageApp write.
// Confirmed writes may clear prose; unconfirmed ones cannot. (The two
// variants exist so no function carries a control-flag parameter.)
// revive:disable-next-line:flag-parameter -- the bool IS the confirm_replace_content
// request field; selection is the whole point of this function.
func pageAppProseGuard(confirmed bool) func(string, string) error {
	if confirmed {
		return pageAppAllowsClear
	}
	return checkPageAppProseGuard
}

// checkPageAppProseGuard blocks prose-clearing writes outright
// (edit path — no confirm field exists there).
func checkPageAppProseGuard(existingMarkdown, newMarkdown string) error {
	const (
		minExistingSizeBytes = 1024
		wipeThresholdPercent = 10
	)

	_, proseBefore, _ := splitPageAppBody(existingMarkdown)
	_, proseAfter, _ := splitPageAppBody(newMarkdown)
	before := len(strings.TrimSpace(proseBefore))
	if before < minExistingSizeBytes {
		return nil
	}
	after := len(strings.TrimSpace(proseAfter))

	// An ordinary edit keeps ≥90% of the substantial prose.
	if after*pageAppHundredPercent >= before*(pageAppHundredPercent-wipeThresholdPercent) {
		return nil
	}

	droppedPercent := float64(pageAppHundredPercent) - float64(after)/float64(before)*float64(pageAppHundredPercent)
	return status.Errorf(codes.FailedPrecondition,
		"this write would drop %.0f%% of the page's prose (%d bytes kept of %d); the app composes ABOVE existing prose by default, so prose rarely needs removing. "+
			"Alternatives: (1) keep the prose — resent via SetPageApp, the app renders above it; (2) create a standalone page app page and link to it from here; "+
			"(3) if the page should become the app alone, resend SetPageApp with confirm_replace_content=true to deliberately replace the content",
		droppedPercent, after, before)
}

// ---------------------------------------------------------------------------
// PageAppStateService — CAS'd state under the reserved wiki.page_app_state
// subtree. The state OCC hash is namespace-scoped (computed over the
// subtree only), NOT the page's whole-content hash: two tabs writing one
// page app's state conflict with each other, and unrelated page-markdown
// edits never invalidate a state handle.

// stateSubtreeKey is the wiki.* subtree key holding page app state.
const stateSubtreeKey = "page_app_state"

// getStateSubtree reads the raw page_app_state subtree from frontmatter.
func getStateSubtree(fm wikipage.FrontMatter) map[string]any {
	raw, _ := getWikiSubtree(fm, stateSubtreeKey)
	return raw
}

// pageAppStateHash computes the state OCC handle: sha256 over the
// subtree's canonical TOML encoding (nil subtree → empty-state hash).
func pageAppStateHash(raw map[string]any) string {
	if raw == nil {
		raw = map[string]any{}
	}
	b, err := toml.Marshal(raw)
	if err != nil {
		b = []byte(fmt.Sprintf("%v", raw))
	}
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// structFromMap converts a raw frontmatter map into a Struct via JSON
// round-tripping (frontmatter values are JSON-compatible primitives,
// lists, and maps).
func structFromMap(raw map[string]any) (*structpb.Struct, error) {
	if len(raw) == 0 {
		return &structpb.Struct{}, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	st := &structpb.Struct{}
	if err := st.UnmarshalJSON(b); err != nil {
		return nil, err
	}
	return st, nil
}

// structToMap converts a Struct into a plain map for frontmatter storage.
func structToMap(st *structpb.Struct) map[string]any {
	if st == nil {
		return map[string]any{}
	}
	b, err := st.MarshalJSON()
	if err != nil {
		return map[string]any{}
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return map[string]any{}
	}
	return out
}

// structFromStateValue flattens a state Struct into values to merge.
func structFromStateValue(src, patch map[string]any) map[string]any {
	out := make(map[string]any, len(src)+len(patch))
	for k, v := range src {
		out[k] = v
	}
	for k, v := range patch {
		switch pv := v.(type) {
		case map[string]any:
			if base, ok := out[k].(map[string]any); ok {
				out[k] = structFromStateValue(base, pv)
			} else {
				out[k] = pv
			}
		case []any:
			// Arrays union (per the state-service contract: scalars replace,
			// arrays union) with string-valued elements deduplicated.
			base, _ := out[k].([]any)
			out[k] = unionStringishArrays(base, pv)
		default:
			out[k] = pv
		}
	}
	return out
}

// unionStringishArrays unions two arrays, deduplicating by string form.
func unionStringishArrays(base, patch []any) []any {
	seen := make(map[string]struct{}, len(base)+len(patch))
	out := make([]any, 0, len(base)+len(patch))
	for _, v := range append(append([]any{}, base...), patch...) {
		key := fmt.Sprintf("%v", v)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	return out
}

// GetPageAppState implements GetPageAppState.
func (s *Server) GetPageAppState(ctx context.Context, req *apiv1.GetPageAppStateRequest) (*apiv1.GetPageAppStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}

	_, fm, err := mut.pages.ReadFrontMatter(page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page %q not found", req.GetPage())
		}
		return nil, status.Errorf(codes.Internal, pageAppErrReadFailed, err)
	}

	raw := getStateSubtree(fm)
	st, err := structFromMap(raw)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert page app state: %v", err)
	}
	return &apiv1.GetPageAppStateResponse{
		State:            st,
		StateVersionHash: pageAppStateHash(raw),
	}, nil
}

// MergePageAppState implements MergePageAppState (deep-merge + CAS).
func (s *Server) MergePageAppState(ctx context.Context, req *apiv1.MergePageAppStateRequest) (*apiv1.MergePageAppStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	if req.GetState() == nil {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}

	patch := structToMap(req.GetState())

	mut.mu.Lock()
	defer mut.mu.Unlock()

	var (
		outState *structpb.Struct
		outHash  string
	)
	modifyErr := mutatePageMarkdown(mut.pages, page, tailscale.IdentityFromContext(ctx), func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		raw := getStateSubtree(currentFM)
		currentHash := pageAppStateHash(raw)
		if req.GetStateExpectedVersionHash() != "" && req.GetStateExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"page app state version mismatch on %q: state was written since last read; re-run GetState and retry", req.GetPage())
		}

		merged := structFromStateValue(raw, patch)
		newHash := pageAppStateHash(merged)
		st, err := structFromMap(merged)
		if err != nil {
			return nil, "", status.Errorf(codes.Internal, "failed to convert merged state: %v", err)
		}
		outState = st
		outHash = newHash
		return setWikiSubtree(currentFM, stateSubtreeKey, merged), currentMarkdown, nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}

	s.pageAppMutator.stateWatch.publish(page, &apiv1.WatchPageAppStateResponse{
		Kind:             apiv1.WatchPageAppStateResponse_MERGED,
		State:            outState,
		StateVersionHash: outHash,
	})
	return &apiv1.MergePageAppStateResponse{State: outState, StateVersionHash: outHash}, nil
}

// DeletePageAppState implements DeletePageAppState.
func (s *Server) DeletePageAppState(ctx context.Context, req *apiv1.DeletePageAppStateRequest) (*apiv1.DeletePageAppStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requirePageAppAccess(ctx, page); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	modifyErr := mutatePageMarkdown(mut.pages, page, tailscale.IdentityFromContext(ctx), func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		raw := getStateSubtree(currentFM)
		currentHash := pageAppStateHash(raw)
		if req.GetStateExpectedVersionHash() != "" && req.GetStateExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"page app state version mismatch on %q: state was written since last read; re-run GetState and retry", req.GetPage())
		}
		return removeWikiSubtree(currentFM, stateSubtreeKey), currentMarkdown, nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}

	s.pageAppMutator.stateWatch.publish(page, &apiv1.WatchPageAppStateResponse{
		Kind:             apiv1.WatchPageAppStateResponse_DELETED,
		State:            &structpb.Struct{},
		StateVersionHash: pageAppStateHash(nil),
	})
	return &apiv1.DeletePageAppStateResponse{}, nil
}

// WatchPageAppState implements the server-streaming state watcher.
func (s *Server) WatchPageAppState(req *apiv1.WatchPageAppStateRequest, stream apiv1.PageAppStateService_WatchPageAppStateServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, pageAppErrPageRequired)
	}
	mut, err := s.getPageAppMutator()
	if err != nil {
		return err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requirePageAppAccess(stream.Context(), page); err != nil {
		return err
	}

	ch := mut.stateWatch.subscribe(page)
	defer mut.stateWatch.unsubscribe(page, ch)

	// Subscribe BEFORE the snapshot (same race rule as WatchPageApp: a
	// merge landing between snapshot read and subscription must not be
	// lost; the hash guard dedupes the replay).
	_, fm, err := mut.pages.ReadFrontMatter(page)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, pageAppErrReadFailed, err)
	}
	raw := getStateSubtree(fm)
	st, cerr := structFromMap(raw)
	if cerr != nil {
		return status.Errorf(codes.Internal, "failed to convert page app state: %v", cerr)
	}
	lastHash := pageAppStateHash(raw)
	if err := stream.Send(&apiv1.WatchPageAppStateResponse{
		Kind:             apiv1.WatchPageAppStateResponse_INITIAL,
		State:            st,
		StateVersionHash: lastHash,
	}); err != nil {
		return err
	}

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case event := <-ch:
			if event.GetStateVersionHash() != "" && event.GetStateVersionHash() == lastHash {
				continue
			}
			lastHash = event.GetStateVersionHash()
			if err := stream.Send(event); err != nil {
				return err
			}
		}
	}
}

// ---------------------------------------------------------------------------
// frontmatter wiki.* subtree helpers

// getWikiSubtree reads fm[pageAppWikiRootKey][subtreeKey] as a map.
func getWikiSubtree(fm wikipage.FrontMatter, subtreeKey string) (map[string]any, bool) {
	wiki, ok := fm[pageAppWikiRootKey].(map[string]any)
	if !ok {
		return nil, false
	}
	sub, ok := wiki[subtreeKey].(map[string]any)
	if !ok {
		return nil, false
	}
	return sub, true
}

// setWikiSubtree returns a copy of fm with fm[pageAppWikiRootKey][subtreeKey] set,
// preserving all other keys (including other wiki.* subtrees).
func setWikiSubtree(fm wikipage.FrontMatter, subtreeKey string, value map[string]any) wikipage.FrontMatter {
	out := make(wikipage.FrontMatter, len(fm)+1)
	for k, v := range fm {
		out[k] = v
	}
	wikiRaw, ok := fm[pageAppWikiRootKey].(map[string]any)
	wiki := make(map[string]any, len(wikiRaw)+1)
	if ok {
		for k, v := range wikiRaw {
			wiki[k] = v
		}
	}
	wiki[subtreeKey] = value
	out["wiki"] = wiki
	return out
}

// removeWikiSubtree returns a copy of fm without fm[pageAppWikiRootKey][subtreeKey].
// The wiki key itself is removed when no other subtrees remain.
func removeWikiSubtree(fm wikipage.FrontMatter, subtreeKey string) wikipage.FrontMatter {
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = v
	}
	wikiRaw, ok := fm[pageAppWikiRootKey].(map[string]any)
	if !ok {
		return out
	}
	wiki := make(map[string]any, len(wikiRaw))
	for k, v := range wikiRaw {
		if k == subtreeKey {
			continue
		}
		wiki[k] = v
	}
	if len(wiki) == 0 {
		delete(out, "wiki")
	} else {
		out["wiki"] = wiki
	}
	return out
}

// ---------------------------------------------------------------------------
// fence rewrite helpers

// replacePageAppFence returns the body with its page app fence (if any)
// replaced by a new one holding the given source; a body without a fence
// gets the new fence appended at the end, preserving prose above
// (composition: app renders above prose; prose is never dropped).
func replacePageAppFence(body string, source string) string {
	_, prose, found := splitPageAppBody(body)
	fence := wikipage.PageAppFenceOpen + mdNewline + strings.TrimSuffix(source, mdNewline) + mdNewline + "```"
	if !found {
		if strings.TrimSpace(prose) == "" {
			return fence + mdNewline
		}
		return strings.TrimRight(prose, mdNewline) + "\n\n" + fence + mdNewline
	}
	trimmedProse := strings.Trim(prose, mdNewline)
	if trimmedProse == "" {
		return fence + mdNewline
	}
	return trimmedProse + "\n\n" + fence + mdNewline
}

// stripPageAppFences returns the body with all page app fences removed
// (the prose remainder, identically to what rendering and indexing see).
func stripPageAppFences(body string) string {
	_, prose, _ := splitPageAppBody(body)
	return prose
}

// ---------------------------------------------------------------------------
// edit-path funnel

// modifyMarkdownWithPageAppFunnelCtx is the ctx-threaded funnel: the
// caller's identity (from ctx) is recorded on any restamp this funnel
// performs, so /edit saves attribute correctly. It returns the final
// frontmatter the funnel wrote (post-restamp), so callers can derive
// response version hashes from what is actually on disk.
func (s *Server) modifyMarkdownWithPageAppFunnelCtx(
	ctx context.Context,
	page wikipage.PageIdentifier,
	baseModifier func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error),
) (wikipage.FrontMatter, error) {
	mut, err := s.getPageAppMutator()
	if err != nil {
		return nil, err
	}
	identity := tailscale.IdentityFromContext(ctx)

	mut.mu.Lock()
	defer mut.mu.Unlock()

	var finalFM wikipage.FrontMatter
	modErr := mutatePageMarkdown(mut.pages, page, identity, func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		newFM, newMD, err := baseModifier(currentFM, currentMarkdown)
		if err != nil {
			return nil, "", err
		}

		oldSource, _, oldFound := splitPageAppBody(string(currentMarkdown))
		newSource, _, newFound := splitPageAppBody(string(newMD))

		// Placement guard FIRST, on the real resulting body, whenever the
		// write touches the page app shape at all — a candidate that clears
		// substantial prose (even with an unchanged program) must fail here.
		// Vacuous when prose is preserved, so ordinary prose edits pass.
		if newFound || oldFound {
			if guardErr := checkPageAppProseGuard(string(currentMarkdown), string(newMD)); guardErr != nil {
				return nil, "", guardErr
			}
		}

		// No fence in the candidate: nothing page app-shaped left to
		// funnel (the guard above already vetted the prose removal). An
		// existing page app whose fence the write removed keeps its stamps
		// (they describe the last written program; ClearPageApp is the
		// explicit removal path that also drops them).
		if !newFound {
			finalFM = newFM
			return newFM, newMD, nil
		}

		// Unchanged program: ordinary prose edit; stamps stay valid.
		if oldFound && oldSource == newSource {
			finalFM = newFM
			return newFM, newMD, nil
		}

		// Changed or new fence: run the funnel — validation + restamp,
		// atomically with the body write (the placement guard already ran
		// above, on the real body).
		if err := validatePageAppSource(newSource); err != nil {
			return nil, "", err
		}

		stamps, _ := getPageAppStamps(currentFM)
		if stamps == nil {
			stamps = map[string]any{}
		}
		if identity != nil && !identity.IsAnonymous() {
			stamps["updated_by"] = identity.LoginName()
			stamps["automated"] = identity.IsAgent()
		}
		stamps["lang"] = PageAppLangOpenuiLang
		stamps["version"] = stampInt(stamps, "version", 0) + 1
		stamps["version_hash"] = pageAppVersionHash(newSource)
		stamps["library"] = PageAppLibrary
		// RFC3339 string (see SetPageApp's stamp write for the indexer rationale).
		stamps["updated_at"] = time.Now().UTC().Format(time.RFC3339)

		finalFM = setWikiSubtree(newFM, "page_app", stamps)
		return finalFM, newMD, nil
	})
	if modErr != nil {
		return nil, modErr
	}
	return finalFM, nil
}
