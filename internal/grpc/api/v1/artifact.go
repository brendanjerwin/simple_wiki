package v1

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// artifactFenceInfoString is the info string on the fenced code block that
	// holds the artifact source (mirrors wikipage.ArtifactFenceInfoString).
	artifactFenceInfoString = "wiki-artifact"

	// v1LibraryVersion is the only supported library in this release.
	v1LibraryVersion = "wiki-core@1"

	// artifactStateKeyPrefix is the frontmatter key under which artifact state
	// is stored. Full key: wiki.artifact_state.
	artifactStateKey = "artifact_state"

	// Placement guard thresholds (mirror checkWholePageWipeGuard constants).
	artifactPlacementMinProseBytes = 1024
	artifactPlacementWipeThreshold = 10

	// artifactDefaultPollMs is the default WatchArtifact polling cadence.
	artifactDefaultPollMs = 1000
	// artifactMinPollMs / artifactMaxPollMs clamp client-supplied intervals.
	artifactMinPollMs = 100
	artifactMaxPollMs = 60000

	// wikiKey is the top-level frontmatter key used for all wiki-managed subtrees.
	wikiKey = "wiki"
	// sha256Prefix is prepended to hex-encoded SHA-256 digests in OCC handles.
	sha256Prefix = "sha256:"
)

// --- ArtifactService ---

// GetArtifact implements ArtifactService.GetArtifact.
func (s *Server) GetArtifact(_ context.Context, req *apiv1.GetArtifactRequest) (*apiv1.GetArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	_, markdown, err := s.pageReaderMutator.ReadMarkdown(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.GetPage())
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	fence := wikipage.SplitArtifactFences(string(markdown))
	if !fence.HasFence {
		return nil, status.Errorf(codes.NotFound, "page %q has no artifact", req.GetPage())
	}

	_, fm, fmErr := s.pageReaderMutator.ReadFrontMatter(pageID)
	if fmErr != nil {
		return nil, status.Errorf(codes.Internal, "failed to read frontmatter: %v", fmErr)
	}

	meta := readArtifactMetadata(fm)
	return &apiv1.GetArtifactResponse{
		Source:   fence.Source,
		Metadata: meta,
	}, nil
}

// SetArtifact implements ArtifactService.SetArtifact.
func (s *Server) SetArtifact(ctx context.Context, req *apiv1.SetArtifactRequest) (*apiv1.SetArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}
	if req.GetSource() == "" {
		return nil, status.Error(codes.InvalidArgument, "source is required")
	}
	if valErr := validateArtifactSource(req.GetSource()); valErr != nil {
		return nil, valErr
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	if guardErr := requireUserMutable(s.pageReaderMutator, pageID); guardErr != nil {
		return nil, guardErr
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, pageID); authErr != nil {
		return nil, authErr
	}
	atomicMod, amErr := requirePageAtomicModifier(s.pageReaderMutator)
	if amErr != nil {
		return nil, amErr
	}

	identity := tailscale.IdentityFromContext(ctx)
	var newMeta *apiv1.ArtifactMetadata

	modErr := atomicMod.ModifyFrontMatterAndMarkdown(pageID,
		func(fm wikipage.FrontMatter, md wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
			return applySetArtifact(req, identity, fm, md, &newMeta)
		},
		identity,
	)
	if modErr != nil {
		if _, ok := status.FromError(modErr); ok {
			return nil, modErr
		}
		return nil, status.Errorf(codes.Internal, "failed to write artifact: %v", modErr)
	}
	return &apiv1.SetArtifactResponse{Metadata: newMeta}, nil
}

// applySetArtifact is the pure mutation logic for SetArtifact. It performs the
// OCC check, placement guard, markdown rebuild, frontmatter stamping, and
// populates out with the new ArtifactMetadata on success.
func applySetArtifact(
	req *apiv1.SetArtifactRequest,
	identity tailscale.IdentityValue,
	currentFM wikipage.FrontMatter,
	currentMarkdown wikipage.Markdown,
	out **apiv1.ArtifactMetadata,
) (wikipage.FrontMatter, wikipage.Markdown, error) {
	currentFence := wikipage.SplitArtifactFences(string(currentMarkdown))

	// OCC check on the artifact version_hash.
	if req.ExpectedVersionHash != nil && currentFence.HasFence {
		currentHash := computeArtifactHash(currentFence.Source)
		if subtle.ConstantTimeCompare([]byte(currentHash), []byte(*req.ExpectedVersionHash)) != 1 {
			return nil, "", status.Error(codes.Aborted, "artifact version mismatch: artifact was modified since last read; re-read and retry")
		}
	}

	// Placement guard: reject if substantial prose would be cleared.
	if !req.GetConfirmReplaceContent() {
		if proseSize := len(strings.TrimSpace(currentFence.ProseRemainder)); proseSize >= artifactPlacementMinProseBytes {
			return nil, "", status.Errorf(codes.FailedPrecondition,
				"this page has substantial prose (%d bytes); setting an artifact that clears it requires confirm_replace_content=true. "+
					"Safer alternatives: (1) keep the prose — the artifact renders above it (app-above-prose composition); "+
					"(2) create a new artifact page and link from here. "+
					"Re-send with confirm_replace_content=true only if the user explicitly wants this page to become the app.",
				proseSize)
		}
	}

	newSource := req.GetSource()
	newMarkdown := buildArtifactMarkdown(currentFence.ProseRemainder, newSource)

	existingMeta := readArtifactMetadata(currentFM)
	newVersion := existingMeta.GetVersion() + 1
	newHash := computeArtifactHash(newSource)
	now := time.Now()

	fm := copyFrontmatter(currentFM)
	wikiSubtree := getOrCreateMap(fm, wikiKey)
	wikiSubtree["artifact"] = map[string]any{
		"lang": "openui-lang", "version": newVersion,
		"version_hash": sha256Prefix + newHash, "library": v1LibraryVersion,
		"updated_at": now.UTC().Format(time.RFC3339),
		"updated_by": identity.Name(), "automated": identity.IsAgent(),
	}
	fm[wikiKey] = wikiSubtree

	*out = &apiv1.ArtifactMetadata{
		Version: newVersion, VersionHash: sha256Prefix + newHash,
		Library: v1LibraryVersion, UpdatedAt: timestamppb.New(now),
		UpdatedBy: identity.Name(), Automated: identity.IsAgent(),
	}
	return fm, wikipage.Markdown(newMarkdown), nil
}

// buildArtifactMarkdown assembles the page body from the prose remainder and
// the new artifact source: fence block precedes the prose (app-above-prose).
func buildArtifactMarkdown(proseRemainder, source string) string {
	fenceBlock := "```" + artifactFenceInfoString + "\n" + source + "\n```"
	if strings.TrimSpace(proseRemainder) != "" {
		return fenceBlock + "\n\n" + proseRemainder
	}
	return fenceBlock
}

// ClearArtifact implements ArtifactService.ClearArtifact.
func (s *Server) ClearArtifact(ctx context.Context, req *apiv1.ClearArtifactRequest) (*apiv1.ClearArtifactResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	if guardErr := requireUserMutable(s.pageReaderMutator, pageID); guardErr != nil {
		return nil, guardErr
	}
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, pageID); authErr != nil {
		return nil, authErr
	}

	atomicMod, amErr := requirePageAtomicModifier(s.pageReaderMutator)
	if amErr != nil {
		return nil, amErr
	}

	identity := tailscale.IdentityFromContext(ctx)

	modErr := atomicMod.ModifyFrontMatterAndMarkdown(
		pageID,
		func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
			// OCC check on the version hash.
			if req.ExpectedVersionHash != nil {
				currentFence := wikipage.SplitArtifactFences(string(currentMarkdown))
				if currentFence.HasFence {
					currentHash := computeArtifactHash(currentFence.Source)
					if subtle.ConstantTimeCompare([]byte(currentHash), []byte(*req.ExpectedVersionHash)) != 1 {
						return nil, "", status.Error(codes.Aborted, "artifact version mismatch: artifact was modified since last read; re-read and retry")
					}
				}
			}

			// Strip the fence; keep the prose.
			fence := wikipage.SplitArtifactFences(string(currentMarkdown))
			newMarkdown := fence.ProseRemainder

			// Remove wiki.artifact subtree from frontmatter.
			fm := copyFrontmatter(currentFM)
			if wikiSubtree, ok := fm[wikiKey].(map[string]any); ok {
				delete(wikiSubtree, "artifact")
				if len(wikiSubtree) == 0 {
					delete(fm, wikiKey)
				} else {
					fm[wikiKey] = wikiSubtree
				}
			}

			return fm, wikipage.Markdown(newMarkdown), nil
		},
		identity,
	)
	if modErr != nil {
		if _, ok := status.FromError(modErr); ok {
			return nil, modErr
		}
		return nil, status.Errorf(codes.Internal, "failed to clear artifact: %v", modErr)
	}

	return &apiv1.ClearArtifactResponse{Success: true}, nil
}

// WatchArtifact implements ArtifactService.WatchArtifact (server-streaming).
func (s *Server) WatchArtifact(req *apiv1.WatchArtifactRequest, stream apiv1.ArtifactService_WatchArtifactServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}
	pageID := wikipage.PageIdentifier(req.GetPage())
	pollMs := clampPollInterval(req.CheckIntervalMs)

	sender := &artifactSender{server: s, pageID: pageID, stream: stream}
	lastHash, err := sender.send()
	if err != nil {
		return err
	}
	return pollForArtifactChanges(stream.Context(), pollMs, lastHash, sender)
}

// artifactSender encapsulates reading and sending the current artifact state.
type artifactSender struct {
	server *Server
	pageID wikipage.PageIdentifier
	stream apiv1.ArtifactService_WatchArtifactServer
}

func (a *artifactSender) send() (string, error) {
	_, markdown, err := a.server.pageReaderMutator.ReadMarkdown(a.pageID)
	if err != nil {
		return "", err
	}
	fence := wikipage.SplitArtifactFences(string(markdown))
	resp := &apiv1.WatchArtifactResponse{}
	if !fence.HasFence {
		resp.Cleared = true
	} else {
		resp.Source = fence.Source
		if _, fm, fmErr := a.server.pageReaderMutator.ReadFrontMatter(a.pageID); fmErr == nil {
			meta := readArtifactMetadata(fm)
			resp.VersionHash = meta.GetVersionHash()
			resp.Version = meta.GetVersion()
		}
	}
	return resp.VersionHash, a.stream.Send(resp)
}

func (a *artifactSender) currentHash() string {
	_, markdown, err := a.server.pageReaderMutator.ReadMarkdown(a.pageID)
	if err != nil {
		return ""
	}
	fence := wikipage.SplitArtifactFences(string(markdown))
	if !fence.HasFence {
		return ""
	}
	_, fm, err := a.server.pageReaderMutator.ReadFrontMatter(a.pageID)
	if err != nil {
		return ""
	}
	return readArtifactMetadata(fm).GetVersionHash()
}

func pollForArtifactChanges(ctx context.Context, pollMs int, lastHash string, sender *artifactSender) error {
	ticker := time.NewTicker(time.Duration(pollMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if current := sender.currentHash(); current != lastHash {
				newHash, sendErr := sender.send()
				if sendErr != nil {
					return sendErr
				}
				lastHash = newHash
			}
		}
	}
}

// --- ArtifactStateService ---

// GetArtifactState implements ArtifactStateService.GetArtifactState.
func (s *Server) GetArtifactState(_ context.Context, req *apiv1.GetArtifactStateRequest) (*apiv1.GetArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	_, fm, err := s.pageReaderMutator.ReadFrontMatter(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			// No page means no state; return empty.
			return &apiv1.GetArtifactStateResponse{State: map[string]string{}}, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	state, versionHash := readArtifactState(fm)
	return &apiv1.GetArtifactStateResponse{
		State:            state,
		StateVersionHash: versionHash,
	}, nil
}

// MergeArtifactState implements ArtifactStateService.MergeArtifactState.
func (s *Server) MergeArtifactState(ctx context.Context, req *apiv1.MergeArtifactStateRequest) (*apiv1.MergeArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}
	if len(req.GetPatch()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "patch is required and must not be empty")
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, pageID); authErr != nil {
		return nil, authErr
	}

	atomicMod, amErr := requirePageAtomicModifier(s.pageReaderMutator)
	if amErr != nil {
		return nil, amErr
	}

	identity := tailscale.IdentityFromContext(ctx)
	var newState map[string]string
	var newHash string

	modErr := atomicMod.ModifyFrontMatterAndMarkdown(
		pageID,
		func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
			currentState, currentHash := readArtifactState(currentFM)

			// OCC check.
			if req.ExpectedStateVersionHash != nil {
				if subtle.ConstantTimeCompare([]byte(currentHash), []byte(*req.ExpectedStateVersionHash)) != 1 {
					return nil, "", status.Error(codes.Aborted, "artifact state version mismatch: state was modified since last read; re-read and retry")
				}
			}

			// Shallow merge: patch keys overwrite existing keys.
			merged := make(map[string]string, len(currentState)+len(req.GetPatch()))
			for k, v := range currentState {
				merged[k] = v
			}
			for k, v := range req.GetPatch() {
				merged[k] = v
			}

			// Compute new version hash.
			newHash = computeStateHash(merged)

			// Write state into wiki.artifact_state subtree.
			fm := copyFrontmatter(currentFM)
			wikiSubtree := getOrCreateMap(fm, wikiKey)
			stateSubtree := make(map[string]any, len(merged))
			for k, v := range merged {
				stateSubtree[k] = v
			}
			wikiSubtree[artifactStateKey] = stateSubtree
			fm[wikiKey] = wikiSubtree

			newState = merged
			return fm, currentMarkdown, nil
		},
		identity,
	)
	if modErr != nil {
		if _, ok := status.FromError(modErr); ok {
			return nil, modErr
		}
		return nil, status.Errorf(codes.Internal, "failed to merge artifact state: %v", modErr)
	}

	return &apiv1.MergeArtifactStateResponse{
		State:            newState,
		StateVersionHash: sha256Prefix + newHash,
	}, nil
}

// DeleteArtifactState implements ArtifactStateService.DeleteArtifactState.
func (s *Server) DeleteArtifactState(ctx context.Context, req *apiv1.DeleteArtifactStateRequest) (*apiv1.DeleteArtifactStateResponse, error) {
	if req.GetPage() == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.GetPage())
	if authErr := requireAuthorized(ctx, s.pageReaderMutator, pageID); authErr != nil {
		return nil, authErr
	}

	atomicMod, amErr := requirePageAtomicModifier(s.pageReaderMutator)
	if amErr != nil {
		return nil, amErr
	}

	identity := tailscale.IdentityFromContext(ctx)

	modErr := atomicMod.ModifyFrontMatterAndMarkdown(
		pageID,
		func(currentFM wikipage.FrontMatter, currentMarkdown wikipage.Markdown) (wikipage.FrontMatter, wikipage.Markdown, error) {
			fm := copyFrontmatter(currentFM)
			if wikiSubtree, ok := fm[wikiKey].(map[string]any); ok {
				delete(wikiSubtree, artifactStateKey)
				if len(wikiSubtree) == 0 {
					delete(fm, wikiKey)
				} else {
					fm[wikiKey] = wikiSubtree
				}
			}
			return fm, currentMarkdown, nil
		},
		identity,
	)
	if modErr != nil {
		if _, ok := status.FromError(modErr); ok {
			return nil, modErr
		}
		return nil, status.Errorf(codes.Internal, "failed to delete artifact state: %v", modErr)
	}

	return &apiv1.DeleteArtifactStateResponse{Success: true}, nil
}

// WatchArtifactState implements ArtifactStateService.WatchArtifactState (server-streaming).
func (s *Server) WatchArtifactState(req *apiv1.WatchArtifactStateRequest, stream apiv1.ArtifactStateService_WatchArtifactStateServer) error {
	if req.GetPage() == "" {
		return status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}
	pageID := wikipage.PageIdentifier(req.GetPage())
	pollMs := clampPollInterval(req.CheckIntervalMs)

	stateSender := &artifactStateSender{server: s, pageID: pageID, stream: stream}
	lastHash, err := stateSender.send()
	if err != nil {
		return err
	}
	return pollForStateChanges(stream.Context(), pollMs, lastHash, stateSender)
}

// artifactStateSender encapsulates reading and sending the current artifact state.
type artifactStateSender struct {
	server *Server
	pageID wikipage.PageIdentifier
	stream apiv1.ArtifactStateService_WatchArtifactStateServer
}

func (a *artifactStateSender) send() (string, error) {
	_, fm, err := a.server.pageReaderMutator.ReadFrontMatter(a.pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", a.stream.Send(&apiv1.WatchArtifactStateResponse{State: map[string]string{}})
		}
		return "", err
	}
	state, versionHash := readArtifactState(fm)
	return versionHash, a.stream.Send(&apiv1.WatchArtifactStateResponse{
		StateVersionHash: versionHash,
		State:            state,
	})
}

func (a *artifactStateSender) currentHash() string {
	_, fm, err := a.server.pageReaderMutator.ReadFrontMatter(a.pageID)
	if err != nil {
		return ""
	}
	_, hash := readArtifactState(fm)
	return hash
}

func pollForStateChanges(ctx context.Context, pollMs int, lastHash string, sender *artifactStateSender) error {
	ticker := time.NewTicker(time.Duration(pollMs) * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if current := sender.currentHash(); current != lastHash {
				newHash, sendErr := sender.send()
				if sendErr != nil {
					return sendErr
				}
				lastHash = newHash
			}
		}
	}
}

// --- Helpers ---

// validateArtifactSource runs a conservative Go-side check on the OpenUI Lang
// source. The authoritative parse lives in the browser (lang-core); this
// check exists to reject gross violations (empty, oversized) while allowing
// partial programs that the renderer can handle gracefully.
func validateArtifactSource(source string) error {
	const maxSourceBytes = 512 * 1024 // 512 KB hard ceiling
	if len(source) > maxSourceBytes {
		return status.Errorf(codes.InvalidArgument,
			"artifact source is too large (%d bytes); maximum is %d bytes", len(source), maxSourceBytes)
	}
	// Additional structural checks can be added here as the OpenUI Lang
	// grammar stabilizes. For now we rely on the renderer's own robustness
	// model (unknown components drop with unknown-component, partial programs
	// render what they can).
	return nil
}

// computeArtifactHash returns the hex-encoded SHA-256 of the source string.
// The "sha256:" prefix is added by callers.
func computeArtifactHash(source string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(source))
	return hex.EncodeToString(h.Sum(nil))
}

// computeStateHash returns the hex-encoded SHA-256 of a deterministic
// serialization of the state map.
func computeStateHash(state map[string]string) string {
	// Use TOML marshal for a deterministic (sorted-key) encoding.
	h := sha256.New()
	if len(state) > 0 {
		b, err := toml.Marshal(state)
		if err == nil {
			_, _ = h.Write(b)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

// readArtifactMetadata extracts the wiki.artifact subtree from frontmatter
// into an ArtifactMetadata proto. Missing fields yield zero values.
func readArtifactMetadata(fm wikipage.FrontMatter) *apiv1.ArtifactMetadata {
	meta := &apiv1.ArtifactMetadata{}
	if fm == nil {
		return meta
	}
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		return meta
	}
	artifact, ok := wiki["artifact"].(map[string]any)
	if !ok {
		return meta
	}
	if v, ok := artifact["version"]; ok {
		switch n := v.(type) {
		case int64:
			meta.Version = n
		case int:
			meta.Version = int64(n)
		case float64:
			meta.Version = int64(n)
		}
	}
	if v, ok := artifact["version_hash"].(string); ok {
		meta.VersionHash = v
	}
	if v, ok := artifact["library"].(string); ok {
		meta.Library = v
	}
	if v, ok := artifact["updated_by"].(string); ok {
		meta.UpdatedBy = v
	}
	if v, ok := artifact["automated"].(bool); ok {
		meta.Automated = v
	}
	if v, ok := artifact["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			meta.UpdatedAt = timestamppb.New(t)
		}
	}
	return meta
}

// readArtifactState extracts the wiki.artifact_state subtree and returns the
// state as a string map plus the current state_version_hash.
func readArtifactState(fm wikipage.FrontMatter) (map[string]string, string) {
	empty := map[string]string{}
	if fm == nil {
		return empty, ""
	}
	wiki, ok := fm[wikiKey].(map[string]any)
	if !ok {
		return empty, ""
	}
	stateRaw, ok := wiki[artifactStateKey].(map[string]any)
	if !ok {
		return empty, ""
	}
	state := make(map[string]string, len(stateRaw))
	for k, v := range stateRaw {
		state[k] = fmt.Sprintf("%v", v)
	}
	hash := computeStateHash(state)
	return state, sha256Prefix + hash
}

// copyFrontmatter makes a shallow copy of the top-level frontmatter map so
// writes into the copy don't alias the original.
func copyFrontmatter(fm wikipage.FrontMatter) wikipage.FrontMatter {
	if fm == nil {
		return make(wikipage.FrontMatter)
	}
	out := make(wikipage.FrontMatter, len(fm))
	for k, v := range fm {
		out[k] = v
	}
	return out
}

// getOrCreateMap returns the map at key k in m, creating it if absent.
func getOrCreateMap(m map[string]any, k string) map[string]any {
	if existing, ok := m[k].(map[string]any); ok {
		out := make(map[string]any, len(existing))
		for ek, ev := range existing {
			out[ek] = ev
		}
		return out
	}
	return make(map[string]any)
}

// HasArtifactFence returns true when the given page's markdown contains a
// ```wiki-artifact fence block. Used by the web handler to set HasArtifact
// in the template data for view composition (app-above-prose rendering).
func HasArtifactFence(markdown string) bool {
	return wikipage.SplitArtifactFences(markdown).HasFence
}

// clampPollInterval returns the polling cadence in milliseconds, clamped to
// [artifactMinPollMs, artifactMaxPollMs]. A nil or zero value yields the default.
func clampPollInterval(requestedMs *int64) int {
	if requestedMs == nil {
		return artifactDefaultPollMs
	}
	requested := int(*requestedMs)
	if requested < artifactMinPollMs {
		return artifactMinPollMs
	}
	if requested > artifactMaxPollMs {
		return artifactMaxPollMs
	}
	return requested
}
