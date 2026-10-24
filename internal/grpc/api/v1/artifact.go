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

// artifact source constants.
const (
	// ArtifactLangOpenuiLang is the only artifact language in this scope.
	ArtifactLangOpenuiLang = "openui-lang"
	// ArtifactLibrary is the wiki component library version the shipped
	// help page teaches.
	ArtifactLibrary = "wiki-core@1"
)

const (
	artifactErrPageRequired = "page is required"
	artifactHundredPercent  = 100
	artifactWikiRootKey     = "wiki"
	mdNewline               = "\n"
)

// artifact program limits. The Go validator's floor: reject gross
// violations, accept anything the renderer could conceivably render
// (authoritative parsing is browser-side lang-core — see ADR-0017).
const (
	artifactMaxSourceBytes = 1 << 16 // 64 KiB
)

// artifactLangRootRe matches the required root-assignment statement.
// In OpenUI Lang every statement is `identifier = Expression` and `root`
// names the entry point (the generated system prompt teaches "always
// start with root = <RootName>(...)"). A program without a root
// assignment renders nothing — that is the gross violation this
// conservative validator rejects.
var artifactLangRootRe = regexp.MustCompile(`(?m)^\s*root\s*=`)

// artifactLangAssignRe matches the general statement shape so partially
// renderable programs (statements beyond the root) are recognized.
var artifactLangAssignRe = regexp.MustCompile(`(?m)^\s*[A-Za-z][A-Za-z0-9_]*\s*=`)

// validateArtifactSource is the conservative Go Lang validator. The
// authoritative parse happens browser-side (lang-core streaming parser,
// which drops unknown components and invalid statements gracefully); this
// validator exists only to never land a program the renderer will choke
// on. It must not false-positive on partial-but-renderable programs:
// gross violations reject (InvalidArgument); everything else accepts.
func validateArtifactSource(source string) error {
	source = strings.TrimSpace(source)
	if source == "" {
		return status.Error(codes.InvalidArgument, "artifact source is empty")
	}
	if len(source) > artifactMaxSourceBytes {
		return status.Errorf(codes.InvalidArgument, "artifact source is %d bytes; the cap is %d bytes", len(source), artifactMaxSourceBytes)
	}

	if !artifactLangRootRe.MatchString(source) {
		return status.Errorf(codes.InvalidArgument,
			"artifact source has no `root =` entry-point statement; every OpenUI Lang program must define root (e.g. `root = Card(...)`) — see the help/artifacts-lang page")
	}
	if !artifactLangAssignRe.MatchString(source) {
		return status.Errorf(codes.InvalidArgument,
			"artifact source has no `identifier =` statements; OpenUI Lang programs are lists of assignments — see the help/artifacts-lang page")
	}

	// A program containing its own ``` terminator would corrupt the
	// written fence; SetArtifact callers send raw source (the wiki wraps
	// it). Reject that.
	if strings.Contains(source, "```") {
		return status.Error(codes.InvalidArgument, "artifact source must not contain a ``` fence terminator; the wiki wraps the source itself")
	}

	return nil
}

// artifactVersionHash computes the sha256 OCC handle for an artifact
// program, prefixed like the page-version hashes for a consistent
// mental model.
func artifactVersionHash(source string) string {
	sum := sha256.Sum256([]byte(source))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// watcherRegistry is a tiny page-keyed pub-sub shared by WatchArtifact and
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

// artifactMutator owns every mutation of a page's artifact program. It
// serializes concurrent Set/Clear on the same page (the page-lock the
// page store holds inside ModifyMarkdown covers the frontmatter read-
// modify-write; this mutex prevents two RPCs from interleaving their
// read-validate-stamp phases).
type artifactMutator struct {
	pages      wikipage.PageReaderMutator
	stateWatch *watcherRegistry[*apiv1.WatchArtifactStateResponse]
	progWatch  *watcherRegistry[*apiv1.WatchArtifactResponse]
	mu         sync.Mutex
}

// splitArtifactBody returns (artifactSource, proseRemainder, found) for
// a page's markdown.
func splitArtifactBody(markdown string) (source string, prose string, found bool) {
	return wikipage.SplitArtifactFences(markdown)
}

// getArtifactStamps reads the wiki.artifact frontmatter subtree.
func getArtifactStamps(fm wikipage.FrontMatter) (map[string]any, bool) {
	wiki, ok := fm[artifactWikiRootKey].(map[string]any)
	if !ok {
		return nil, false
	}
	artifact, ok := wiki["artifact"].(map[string]any)
	if !ok {
		return nil, false
	}
	return artifact, true
}

// getCurrentArtifact reads the current artifact (source + stamps) from a
// page. found=false when the page exists but has no artifact.
func (a *artifactMutator) getCurrentArtifact(page wikipage.PageIdentifier) (source string, stamps map[string]any, found bool, err error) {
	_, markdown, err := a.pages.ReadMarkdown(page)
	if err != nil {
		return "", nil, false, err
	}
	source, _, found = splitArtifactBody(string(markdown))
	if !found {
		return "", nil, false, nil
	}
	_, fm, err := a.pages.ReadFrontMatter(page)
	if err != nil {
		return "", nil, false, err
	}
	stamps, _ = getArtifactStamps(fm)
	return source, stamps, found, nil
}

// requireArtifactAccess applies the page-level guards every artifact RPC
// shares: user-mutable (not a system page) and wiki.authorization ACL.
func (a *artifactMutator) requireArtifactAccess(ctx context.Context, page wikipage.PageIdentifier) error {
	if err := requireUserMutable(a.pages, page); err != nil {
		return err
	}
	return requireAuthorized(ctx, a.pages, page)
}

// newArtifactMutator constructs the artifact funnel over a page store.
func newArtifactMutator(pages wikipage.PageReaderMutator) *artifactMutator {
	return &artifactMutator{
		pages:      pages,
		progWatch:  newWatcherRegistry[*apiv1.WatchArtifactResponse](),
		stateWatch: newWatcherRegistry[*apiv1.WatchArtifactStateResponse](),
	}
}

// getArtifactMutator returns the server's artifact funnel, constructing a
// default over the page store when none was wired. Construction is
// guarded by a Once so concurrent first calls (an agent's SetArtifact and
// a browser's WatchArtifact landing together) never race the field write.
func (s *Server) getArtifactMutator() (*artifactMutator, error) {
	s.artifactMutatorOnce.Do(func() {
		if s.pageReaderMutator != nil {
			s.artifactMutator = newArtifactMutator(s.pageReaderMutator)
		}
	})
	if s.artifactMutator == nil {
		return nil, status.Error(codes.Internal, "artifact services not configured: no page store")
	}
	return s.artifactMutator, nil
}

// validateChangedArtifactFence is the edit-path artifact funnel
// (criterion 6): a write (UpdatePageContent / UpdateWholePage / CreatePage)
// whose new body carries a wiki-artifact fence that is *new or changed*
// passes through the same validation + placement-guard gate as
// SetArtifact, so no route can land an unvalidated fence. An unchanged
// fence (ordinary prose edit on an artifact page) skips re-validation —
// the program was validated when written.
func validateChangedArtifactFence(_ wikipage.FrontMatter, currentMarkdown wikipage.Markdown, newWholeMarkdown string) error {
	oldSource, _, oldFound := splitArtifactBody(string(currentMarkdown))
	newSource, _, newFound := splitArtifactBody(newWholeMarkdown)

	if !newFound {
		return nil // no fence in the new body: nothing to validate
	}

	if oldFound && oldSource == newSource {
		// Unchanged program: ordinary prose edit; the placement guard
		// still applies to the prose the write would remove.
		return nil
	}

	if err := validateArtifactSource(newSource); err != nil {
		return err
	}

	// The candidate body IS the write's resulting body (the caller supplies
	// the whole markdown): the guard compares current prose vs resulting
	// prose after fence removal. Edit-path writes have no confirm field;
	// the error names SetArtifact (which has one).
	return checkArtifactProseGuard(string(currentMarkdown), newWholeMarkdown)
}

// GetArtifact implements GetArtifact.
func (s *Server) GetArtifact(ctx context.Context, req *apiv1.GetArtifactRequest) (*apiv1.GetArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requireArtifactAccess(ctx, page); err != nil {
		return nil, err
	}

	source, stamps, found, err := mut.getCurrentArtifact(page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page %q not found", req.GetPage())
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}
	if !found {
		return nil, status.Errorf(codes.NotFound, "page %q has no artifact", req.GetPage())
	}

	resp := &apiv1.GetArtifactResponse{
		Source:  source,
		Lang:    stampString(stamps, "lang", ArtifactLangOpenuiLang),
		Version: stampInt(stamps, "version", 0),
		Library: stampString(stamps, "library", ArtifactLibrary),
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

// SetArtifact implements the single program write door: validate, guard,
// stamp, persist, publish.
func (s *Server) SetArtifact(ctx context.Context, req *apiv1.SetArtifactRequest) (*apiv1.SetArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())

	if err := mut.requireArtifactAccess(ctx, page); err != nil {
		return nil, err
	}
	if err := validateArtifactSource(req.GetSource()); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	identity := tailscale.IdentityFromContext(ctx)

	var result *apiv1.SetArtifactResponse
	modifyErr := mutatePageMarkdown(mut.pages, page, identity, func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		newFM, newMD, rpcResponse, err := applySetArtifact(currentFM, currentMarkdown, req, identity)
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
		s.artifactMutator.progWatch.publish(page, &apiv1.WatchArtifactResponse{
			Kind:        apiv1.WatchArtifactResponse_UPDATED,
			Source:      req.GetSource(),
			Version:     result.GetVersion(),
			VersionHash: result.GetVersionHash(),
		})
	}
	return result, nil
}

// applySetArtifact is SetArtifact's mutation body: OCC check, placement
// guard, stamping, fence rewrite. Reads only its arguments so the atomic
// modifier drives it. (Plain function — no receiver state involved.)
func applySetArtifact(
	currentFM wikipage.FrontMatter,
	currentMarkdown wikipage.Markdown,
	req *apiv1.SetArtifactRequest,
	identity tailscale.IdentityValue,
) (wikipage.FrontMatter, wikipage.Markdown, *apiv1.SetArtifactResponse, error) {
	_, _, existingFound := splitArtifactBody(string(currentMarkdown))
	stamps, stampsFound := getArtifactStamps(currentFM)
	if !stampsFound {
		stamps = map[string]any{}
	}

	// OCC: enforced when expected_version_hash is non-empty. On first
	// write an empty hash is accepted; on replacement an omitted hash is
	// last-writer-wins (the help page + proto doc teach supplying the
	// hash from GetArtifact so conflicts surface instead of overwriting).
	if existingFound {
		fenceContent, _, _ := splitArtifactBody(string(currentMarkdown))
		currentHash := artifactVersionHash(fenceContent)
		if req.GetExpectedVersionHash() != "" && req.GetExpectedVersionHash() != currentHash {
			return nil, "", nil, status.Errorf(codes.Aborted,
				"artifact version mismatch: page %q's artifact was modified since last read; re-run GetArtifact and retry", req.GetPage())
		}
	} else if req.GetExpectedVersionHash() != "" {
		return nil, "", nil, status.Errorf(codes.Aborted,
			"artifact version mismatch: page %q has no artifact but expected_version_hash was set; retry without it", req.GetPage())
	}

	storedSource := strings.TrimSuffix(req.GetSource(), mdNewline)

	// The exact body this write will produce. Default: composition
	// (fence installed above the preserved prose). With
	// confirm_replace_content=true: the app-standalone conversion.
	var candidateBody string
	if req.GetConfirmReplaceContent() {
		candidateBody = replaceArtifactFence("", storedSource)
	} else {
		candidateBody = replaceArtifactFence(string(currentMarkdown), storedSource)
	}
	if guardErr := artifactProseGuard(req.GetConfirmReplaceContent())(string(currentMarkdown), candidateBody); guardErr != nil {
		return nil, "", nil, guardErr
	}

	nextVersion := stampInt(stamps, "version", 0) + 1
	newHash := artifactVersionHash(storedSource)

	stamps["lang"] = ArtifactLangOpenuiLang
	stamps["version"] = nextVersion
	stamps["version_hash"] = newHash
	stamps["library"] = ArtifactLibrary
	// RFC3339 string, not time.Time: the frontmatter indexer rejects
	// non-scalar values (a time.Time left whole pages unindexed — see e2e).
	stamps["updated_at"] = time.Now().UTC().Format(time.RFC3339)
	if identity != nil && !identity.IsAnonymous() {
		stamps["updated_by"] = identity.LoginName()
		stamps["automated"] = identity.IsAgent()
	}

	return setWikiSubtree(currentFM, "artifact", stamps),
		wikipage.Markdown(candidateBody),
		&apiv1.SetArtifactResponse{
			Version:     nextVersion,
			VersionHash: newHash,
			Library:     ArtifactLibrary,
		},
		nil
}

// ClearArtifact removes the fence and wiki.artifact subtree; prose is
// untouched.
func (s *Server) ClearArtifact(ctx context.Context, req *apiv1.ClearArtifactRequest) (*apiv1.ClearArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}

	page := wikipage.PageIdentifier(req.GetPage())
	mut, mutErr := s.getArtifactMutator()
	if mutErr != nil {
		return nil, mutErr
	}

	if err := mut.requireArtifactAccess(ctx, page); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	var publishEvent *apiv1.WatchArtifactResponse
	modifyErr := mutatePageMarkdown(mut.pages, page, tailscale.IdentityFromContext(ctx), func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		_, _, existingFound := splitArtifactBody(string(currentMarkdown))
		if !existingFound {
			return nil, "", status.Errorf(codes.NotFound, "page %q has no artifact", req.GetPage())
		}

		fenceContent, _, _ := splitArtifactBody(string(currentMarkdown))
		currentHash := artifactVersionHash(fenceContent)
		if req.GetExpectedVersionHash() != "" && req.GetExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"artifact version mismatch: page %q's artifact was modified since last read; re-run GetArtifact and retry", req.GetPage())
		}

		newFM := removeWikiSubtree(currentFM, "artifact")
		newMD := stripArtifactFences(string(currentMarkdown))

		publishEvent = &apiv1.WatchArtifactResponse{
			Kind:        apiv1.WatchArtifactResponse_CLEARED,
			VersionHash: currentHash,
		}
		return newFM, wikipage.Markdown(newMD), nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}

	s.artifactMutator.progWatch.publish(page, publishEvent)
	return &apiv1.ClearArtifactResponse{}, nil
}

// WatchArtifact implements the server-streaming hot-swap driver.
func (s *Server) WatchArtifact(req *apiv1.WatchArtifactRequest, stream apiv1.ArtifactService_WatchArtifactServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}

	page := wikipage.PageIdentifier(req.GetPage())
	mut, mutErr := s.getArtifactMutator()
	if mutErr != nil {
		return mutErr
	}

	if err := mut.requireArtifactAccess(stream.Context(), page); err != nil {
		return err
	}

	ch := mut.progWatch.subscribe(page)
	defer mut.progWatch.unsubscribe(page, ch)

	// Subscribe BEFORE the snapshot: an accepted SetArtifact landing
	// between the snapshot read and subscription would otherwise be
	// silently lost. The version-hash guard drops the redundant replay.
	source, stamps, found, err := mut.getCurrentArtifact(page)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "failed to read current artifact: %v", err)
	}
	var lastHash string
	if found {
		lastHash = stampString(stamps, "version_hash", "")
		if err := stream.Send(&apiv1.WatchArtifactResponse{
			Kind:        apiv1.WatchArtifactResponse_INITIAL,
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
			if event.GetKind() != apiv1.WatchArtifactResponse_CLEARED &&
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
// is_agent) — artifact writes must record who wrote them.
func mutatePageMarkdown(pages wikipage.PageReaderMutator, page wikipage.PageIdentifier, identity wikipage.Identity, fn func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error)) error {
	atomicMod, amErr := requirePageAtomicModifier(pages)
	if amErr != nil {
		return amErr
	}
	return atomicMod.ModifyFrontMatterAndMarkdown(page, fn, identity)
}

// checkArtifactProseGuard is the placement guard (mechanical backstop for
// the composition model): a write that drops more than 10% of the page's
// substantial prose (existing non-fence body ≥1KB) requires explicit
// confirmation. SetArtifact's default path composes (keeps prose) so it
// passes vacuously; the app-standalone conversion sets
// confirm_replace_content=true; edit-path writes (UpdatePageContent /
// UpdateWholePage / CreatePage) have no confirm field, so a
// changed-fence write that clears substantial prose fails with
// FailedPrecondition naming the alternatives.
// guardAllowsClear is the confirmed-write guard: the caller has already
// checked confirm_replace_content, so this yields unconditionally (the
// deliberate app-standalone conversion).
func guardAllowsClear(_, _ string) error {
	return nil
}

// artifactProseGuard returns the guard variant for a SetArtifact write.
// Confirmed writes may clear prose; unconfirmed ones cannot. (The two
// variants exist so no function carries a control-flag parameter.)
// revive:disable-next-line:flag-parameter -- the bool IS the confirm_replace_content
// request field; selection is the whole point of this function.
func artifactProseGuard(confirmed bool) func(string, string) error {
	if confirmed {
		return guardAllowsClear
	}
	return checkArtifactProseGuard
}

// checkArtifactProseGuard blocks prose-clearing writes outright
// (edit path — no confirm field exists there).
func checkArtifactProseGuard(existingMarkdown, newMarkdown string) error {
	const (
		minExistingSizeBytes = 1024
		wipeThresholdPercent = 10
	)

	_, proseBefore, _ := splitArtifactBody(existingMarkdown)
	_, proseAfter, _ := splitArtifactBody(newMarkdown)
	before := len(strings.TrimSpace(proseBefore))
	if before < minExistingSizeBytes {
		return nil
	}
	after := len(strings.TrimSpace(proseAfter))

	// An ordinary edit keeps ≥90% of the substantial prose.
	if after*artifactHundredPercent >= before*(artifactHundredPercent-wipeThresholdPercent) {
		return nil
	}

	droppedPercent := float64(artifactHundredPercent) - float64(after)/float64(before)*float64(artifactHundredPercent)
	return status.Errorf(codes.FailedPrecondition,
		"this write would drop %.0f%% of the page's prose (%d bytes kept of %d); the app composes ABOVE existing prose by default, so prose rarely needs removing. "+
			"Alternatives: (1) keep the prose — resent via SetArtifact, the app renders above it; (2) create a standalone artifact page and link to it from here; "+
			"(3) if the page should become the app alone, resend SetArtifact with confirm_replace_content=true to deliberately replace the content",
		droppedPercent, after, before)
}

// ---------------------------------------------------------------------------
// ArtifactStateService — CAS'd state under the reserved wiki.artifact_state
// subtree. The state OCC hash is namespace-scoped (computed over the
// subtree only), NOT the page's whole-content hash: two tabs writing one
// artifact's state conflict with each other, and unrelated page-markdown
// edits never invalidate a state handle.

// stateSubtreeKey is the wiki.* subtree key holding artifact state.
const stateSubtreeKey = "artifact_state"

// getStateSubtree reads the raw artifact_state subtree from frontmatter.
func getStateSubtree(fm wikipage.FrontMatter) map[string]any {
	raw, _ := getWikiSubtree(fm, stateSubtreeKey)
	return raw
}

// artifactStateHash computes the state OCC handle: sha256 over the
// subtree's canonical TOML encoding (nil subtree → empty-state hash).
func artifactStateHash(raw map[string]any) string {
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

// GetArtifactState implements GetArtifactState.
func (s *Server) GetArtifactState(ctx context.Context, req *apiv1.GetArtifactStateRequest) (*apiv1.GetArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requireArtifactAccess(ctx, page); err != nil {
		return nil, err
	}

	_, fm, err := mut.pages.ReadFrontMatter(page)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page %q not found", req.GetPage())
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	raw := getStateSubtree(fm)
	st, err := structFromMap(raw)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to convert artifact state: %v", err)
	}
	return &apiv1.GetArtifactStateResponse{
		State:            st,
		StateVersionHash: artifactStateHash(raw),
	}, nil
}

// MergeArtifactState implements MergeArtifactState (deep-merge + CAS).
func (s *Server) MergeArtifactState(ctx context.Context, req *apiv1.MergeArtifactStateRequest) (*apiv1.MergeArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	if req.GetState() == nil {
		return nil, status.Error(codes.InvalidArgument, "state is required")
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requireArtifactAccess(ctx, page); err != nil {
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
		currentHash := artifactStateHash(raw)
		if req.GetStateExpectedVersionHash() != "" && req.GetStateExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"artifact state version mismatch on %q: state was written since last read; re-run GetState and retry", req.GetPage())
		}

		merged := structFromStateValue(raw, patch)
		newHash := artifactStateHash(merged)
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

	s.artifactMutator.stateWatch.publish(page, &apiv1.WatchArtifactStateResponse{
		Kind:             apiv1.WatchArtifactStateResponse_MERGED,
		State:            outState,
		StateVersionHash: outHash,
	})
	return &apiv1.MergeArtifactStateResponse{State: outState, StateVersionHash: outHash}, nil
}

// DeleteArtifactState implements DeleteArtifactState.
func (s *Server) DeleteArtifactState(ctx context.Context, req *apiv1.DeleteArtifactStateRequest) (*apiv1.DeleteArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return nil, err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requireArtifactAccess(ctx, page); err != nil {
		return nil, err
	}

	mut.mu.Lock()
	defer mut.mu.Unlock()

	modifyErr := mutatePageMarkdown(mut.pages, page, tailscale.IdentityFromContext(ctx), func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
		raw := getStateSubtree(currentFM)
		currentHash := artifactStateHash(raw)
		if req.GetStateExpectedVersionHash() != "" && req.GetStateExpectedVersionHash() != currentHash {
			return nil, "", status.Errorf(codes.Aborted,
				"artifact state version mismatch on %q: state was written since last read; re-run GetState and retry", req.GetPage())
		}
		return removeWikiSubtree(currentFM, stateSubtreeKey), currentMarkdown, nil
	})
	if modifyErr != nil {
		return nil, modifyErr
	}

	s.artifactMutator.stateWatch.publish(page, &apiv1.WatchArtifactStateResponse{
		Kind:             apiv1.WatchArtifactStateResponse_DELETED,
		State:            &structpb.Struct{},
		StateVersionHash: artifactStateHash(nil),
	})
	return &apiv1.DeleteArtifactStateResponse{}, nil
}

// WatchArtifactState implements the server-streaming state watcher.
func (s *Server) WatchArtifactState(req *apiv1.WatchArtifactStateRequest, stream apiv1.ArtifactStateService_WatchArtifactStateServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, artifactErrPageRequired)
	}
	mut, err := s.getArtifactMutator()
	if err != nil {
		return err
	}

	page := wikipage.PageIdentifier(req.GetPage())
	if err := mut.requireArtifactAccess(stream.Context(), page); err != nil {
		return err
	}

	ch := mut.stateWatch.subscribe(page)
	defer mut.stateWatch.unsubscribe(page, ch)

	// Subscribe BEFORE the snapshot (same race rule as WatchArtifact: a
	// merge landing between snapshot read and subscription must not be
	// lost; the hash guard dedupes the replay).
	_, fm, err := mut.pages.ReadFrontMatter(page)
	if err != nil && !os.IsNotExist(err) {
		return status.Errorf(codes.Internal, "failed to read page: %v", err)
	}
	raw := getStateSubtree(fm)
	st, cerr := structFromMap(raw)
	if cerr != nil {
		return status.Errorf(codes.Internal, "failed to convert artifact state: %v", cerr)
	}
	lastHash := artifactStateHash(raw)
	if err := stream.Send(&apiv1.WatchArtifactStateResponse{
		Kind:             apiv1.WatchArtifactStateResponse_INITIAL,
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

// getWikiSubtree reads fm[artifactWikiRootKey][subtreeKey] as a map.
func getWikiSubtree(fm wikipage.FrontMatter, subtreeKey string) (map[string]any, bool) {
	wiki, ok := fm[artifactWikiRootKey].(map[string]any)
	if !ok {
		return nil, false
	}
	sub, ok := wiki[subtreeKey].(map[string]any)
	if !ok {
		return nil, false
	}
	return sub, true
}

// setWikiSubtree returns a copy of fm with fm[artifactWikiRootKey][subtreeKey] set,
// preserving all other keys (including other wiki.* subtrees).
func setWikiSubtree(fm wikipage.FrontMatter, subtreeKey string, value map[string]any) wikipage.FrontMatter {
	out := make(wikipage.FrontMatter, len(fm)+1)
	for k, v := range fm {
		out[k] = v
	}
	wikiRaw, ok := fm[artifactWikiRootKey].(map[string]any)
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

// removeWikiSubtree returns a copy of fm without fm[artifactWikiRootKey][subtreeKey].
// The wiki key itself is removed when no other subtrees remain.
func removeWikiSubtree(fm wikipage.FrontMatter, subtreeKey string) wikipage.FrontMatter {
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = v
	}
	wikiRaw, ok := fm[artifactWikiRootKey].(map[string]any)
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

// replaceArtifactFence returns the body with its artifact fence (if any)
// replaced by a new one holding the given source; a body without a fence
// gets the new fence appended at the end, preserving prose above
// (composition: app renders above prose; prose is never dropped).
func replaceArtifactFence(body string, source string) string {
	_, prose, found := splitArtifactBody(body)
	fence := wikipage.ArtifactFenceOpen + mdNewline + strings.TrimSuffix(source, mdNewline) + mdNewline + "```"
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

// stripArtifactFences returns the body with all artifact fences removed
// (the prose remainder, identically to what rendering and indexing see).
func stripArtifactFences(body string) string {
	_, prose, _ := splitArtifactBody(body)
	return prose
}

// ---------------------------------------------------------------------------
// edit-path funnel

// modifyMarkdownWithArtifactFunnelCtx is the ctx-threaded funnel: the
// caller's identity (from ctx) is recorded on any restamp this funnel
// performs, so /edit saves attribute correctly. It returns the final
// frontmatter the funnel wrote (post-restamp), so callers can derive
// response version hashes from what is actually on disk.
func (s *Server) modifyMarkdownWithArtifactFunnelCtx(
	ctx context.Context,
	page wikipage.PageIdentifier,
	baseModifier func(wikipage.FrontMatter, wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error),
) (wikipage.FrontMatter, error) {
	mut, err := s.getArtifactMutator()
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

		oldSource, _, oldFound := splitArtifactBody(string(currentMarkdown))
		newSource, _, newFound := splitArtifactBody(string(newMD))

		// Placement guard FIRST, on the real resulting body, whenever the
		// write touches the artifact shape at all — a candidate that clears
		// substantial prose (even with an unchanged program) must fail here.
		// Vacuous when prose is preserved, so ordinary prose edits pass.
		if newFound || oldFound {
			if guardErr := checkArtifactProseGuard(string(currentMarkdown), string(newMD)); guardErr != nil {
				return nil, "", guardErr
			}
		}

		// No fence in the candidate: nothing artifact-shaped left to
		// funnel (the guard above already vetted the prose removal). An
		// existing artifact whose fence the write removed keeps its stamps
		// (they describe the last written program; ClearArtifact is the
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
		if err := validateArtifactSource(newSource); err != nil {
			return nil, "", err
		}

		stamps, _ := getArtifactStamps(currentFM)
		if stamps == nil {
			stamps = map[string]any{}
		}
		if identity != nil && !identity.IsAnonymous() {
			stamps["updated_by"] = identity.LoginName()
			stamps["automated"] = identity.IsAgent()
		}
		stamps["lang"] = ArtifactLangOpenuiLang
		stamps["version"] = stampInt(stamps, "version", 0) + 1
		stamps["version_hash"] = artifactVersionHash(newSource)
		stamps["library"] = ArtifactLibrary
		// RFC3339 string (see SetArtifact's stamp write for the indexer rationale).
		stamps["updated_at"] = time.Now().UTC().Format(time.RFC3339)

		finalFM = setWikiSubtree(newFM, "artifact", stamps)
		return finalFM, newMD, nil
	})
	if modErr != nil {
		return nil, modErr
	}
	return finalFM, nil
}
