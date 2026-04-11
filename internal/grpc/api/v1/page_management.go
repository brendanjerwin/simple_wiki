package v1

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	apiv1 "github.com/brendanjerwin/simple_wiki/gen/go/api/v1"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/tailscale"
	"github.com/brendanjerwin/simple_wiki/templating"
	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/pelletier/go-toml/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// computeContentHash computes a SHA256 hash of the given markdown content,
// returned as a lowercase hex string. Used for optimistic concurrency control.
func computeContentHash(markdown wikipage.Markdown) string {
	h := sha256.Sum256([]byte(markdown))
	return hex.EncodeToString(h[:])
}

// buildPageText assembles the full wiki page text by prepending TOML frontmatter
// (enclosed in +++ delimiters) to the markdown body.
func buildPageText(frontmatter map[string]any, frontmatterToml []byte, markdown wikipage.Markdown) string {
	var b strings.Builder

	if len(frontmatter) > 0 {
		_, _ = b.WriteString("+++\n") // WriteString on strings.Builder never fails
		_, _ = b.Write(frontmatterToml) // Write on strings.Builder never fails
		if !bytes.HasSuffix(frontmatterToml, []byte("\n")) {
			_, _ = b.WriteString("\n") // WriteString on strings.Builder never fails
		}
		_, _ = b.WriteString("+++\n") // WriteString on strings.Builder never fails
	}

	_, _ = b.WriteString(string(markdown)) // WriteString on strings.Builder never fails

	return b.String()
}

// checkContentVersionHash verifies that the current page content matches the expected version hash.
// Returns an error if there is a version mismatch; returns nil if expectedHash is nil (no check requested).
func checkContentVersionHash(currentMarkdown wikipage.Markdown, expectedHash *string) error {
	if expectedHash == nil {
		return nil
	}

	currentHash := computeContentHash(currentMarkdown)
	if subtle.ConstantTimeCompare([]byte(currentHash), []byte(*expectedHash)) != 1 {
		return status.Error(codes.Aborted, "content version mismatch: page was modified since last read; re-read the page and retry")
	}

	return nil
}

// resolveMarkdownContent computes the markdown to write.
// When oldContentMarkdown is provided, performs a find-and-replace within originalMarkdown.
// Otherwise returns newContentMarkdown as the full replacement.
func resolveMarkdownContent(originalMarkdown wikipage.Markdown, oldContentMarkdown *string, newContentMarkdown string) (wikipage.Markdown, error) {
	if oldContentMarkdown == nil {
		return wikipage.Markdown(newContentMarkdown), nil
	}

	if !strings.Contains(string(originalMarkdown), *oldContentMarkdown) {
		return "", status.Error(codes.NotFound, "old_content_markdown not found in current page content")
	}

	return wikipage.Markdown(strings.Replace(string(originalMarkdown), *oldContentMarkdown, newContentMarkdown, 1)), nil
}

// verifyStoredContent reads back the page after a write to confirm the content is non-empty.
// If the stored content is empty or unreadable, it attempts to restore the original content.
func (s *Server) verifyStoredContent(pageID wikipage.PageIdentifier, originalMarkdown wikipage.Markdown) (wikipage.Markdown, error) {
	_, storedMarkdown, readBackErr := s.pageReaderMutator.ReadMarkdown(pageID)
	if readBackErr != nil || strings.TrimSpace(string(storedMarkdown)) == "" {
		_ = s.pageReaderMutator.WriteMarkdown(pageID, originalMarkdown)
		if readBackErr != nil {
			return "", status.Errorf(codes.Internal, "failed to verify stored content after write: %v", readBackErr)
		}
		return "", status.Error(codes.Internal, "invariant violation: write resulted in empty content; original content has been restored")
	}

	return storedMarkdown, nil
}

// loadTemplateFrontmatter loads and validates a template page's frontmatter.
func (s *Server) loadTemplateFrontmatter(templateIdentifier string) (map[string]any, error) {
	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(templateIdentifier))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("template '%s' does not exist", templateIdentifier)
		}
		return nil, fmt.Errorf("failed to read template '%s': %w", templateIdentifier, err)
	}

	// Validate that the page is marked as a template
	if !isTemplatePage(fm) {
		return nil, fmt.Errorf("page '%s' is not a template (missing template: true)", templateIdentifier)
	}

	return fm, nil
}

// isTemplatePage checks if a frontmatter map indicates a template page.
func isTemplatePage(fm map[string]any) bool {
	templateVal, ok := fm["template"]
	if !ok {
		return false
	}

	switch v := templateVal.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(v, "true")
	case int64:
		// TOML parses integers as int64
		return v != 0
	case float64:
		// Handle floats for robustness
		return v != 0
	default:
		return false
	}
}

// applyTemplateFrontmatter copies template frontmatter keys to dest,
// excluding the identifier and template keys.
func applyTemplateFrontmatter(dest, src map[string]any) {
	for k, v := range src {
		if k != identifierKey && k != "template" {
			dest[k] = v
		}
	}
}

// applyProvidedFrontmatter copies user-provided frontmatter keys to dest,
// excluding the identifier key.
func applyProvidedFrontmatter(dest map[string]any, frontmatter *structpb.Struct) {
	if frontmatter == nil {
		return
	}
	for k, v := range frontmatter.AsMap() {
		if k != identifierKey {
			dest[k] = v
		}
	}
}

// buildNewPageFrontmatter assembles frontmatter for a new page by merging
// template defaults with any explicitly provided frontmatter values.
func (s *Server) buildNewPageFrontmatter(identifier string, template *string, frontmatter *structpb.Struct) (map[string]any, error) {
	fm := map[string]any{identifierKey: identifier}

	if template != nil && *template != "" {
		templateFm, err := s.loadTemplateFrontmatter(*template)
		if err != nil {
			return nil, err
		}
		applyTemplateFrontmatter(fm, templateFm)
	}

	applyProvidedFrontmatter(fm, frontmatter)
	return fm, nil
}

// checkIdentifierAvailability checks if an identifier is available and returns info about existing page if not.
func (s *Server) checkIdentifierAvailability(identifier string) (bool, *apiv1.ExistingPageInfo) {
	_, fm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	if err != nil {
		// Page doesn't exist
		return true, nil
	}

	// Page exists, build info
	existingPage := &apiv1.ExistingPageInfo{
		Identifier: identifier,
	}

	// Get title from frontmatter
	if title, ok := fm["title"].(string); ok {
		existingPage.Title = title
	}

	// Get container from inventory.container
	if inv, ok := fm["inventory"].(map[string]any); ok {
		if container, ok := inv["container"].(string); ok {
			existingPage.Container = container
		}
	}

	return false, existingPage
}

// findUniqueIdentifier finds a unique identifier by adding numeric suffixes.
func (s *Server) findUniqueIdentifier(baseIdentifier string) string {
	// Try suffixes _1, _2, _3, etc.
	for i := 1; i < maxUniqueIdentifierAttempts; i++ {
		candidate := fmt.Sprintf("%s_%d", baseIdentifier, i)

		_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(candidate))
		if err != nil {
			// Page doesn't exist, we found a unique identifier
			return candidate
		}
	}

	// Fallback: return with a high number
	return baseIdentifier + "_999"
}

// resolveCheckInterval returns the polling interval for WatchPage.
// Defaults to 1 second when not specified; enforces a 100ms minimum to protect the server.
func resolveCheckInterval(requestedIntervalMs int32) time.Duration {
	const defaultInterval = 1 * time.Second
	const minIntervalMs = 100

	interval := time.Duration(requestedIntervalMs) * time.Millisecond
	if interval == 0 {
		interval = defaultInterval
	}

	if interval < minIntervalMs*time.Millisecond {
		interval = minIntervalMs * time.Millisecond
	}

	return interval
}

// sendPageUpdateIfChanged checks whether the page content has changed since lastHash and,
// if so, sends a WatchPageResponse on the stream. Returns the current hash (updated or unchanged).
// On a transient read error it logs and returns the previous hash so the caller can retry.
func (s *Server) sendPageUpdateIfChanged(stream apiv1.PageManagementService_WatchPageServer, pageID wikipage.PageIdentifier, lastHash string) (string, error) {
	currentHash, currentModTime, err := s.readPageHashAndModTime(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return "", status.Errorf(codes.NotFound, "page deleted: %s", string(pageID))
		}
		s.logger.Warn("WatchPage: failed to read page content, continuing: %v", err)
		return lastHash, nil
	}

	if currentHash == lastHash {
		return lastHash, nil
	}

	if err := stream.Send(&apiv1.WatchPageResponse{
		VersionHash:  currentHash,
		LastModified: timestamppb.New(currentModTime),
	}); err != nil {
		return "", err
	}

	return currentHash, nil
}

// readPageHashAndModTime reads a page's content hash and file modification time.
func (s *Server) readPageHashAndModTime(pageID wikipage.PageIdentifier) (string, time.Time, error) {
	page, err := s.pageOpener.ReadPage(pageID)
	if err != nil {
		return "", time.Time{}, err
	}
	if page.IsNew() {
		return "", time.Time{}, os.ErrNotExist
	}
	markdown, err := page.GetMarkdown()
	if err != nil {
		return "", time.Time{}, err
	}
	return computeContentHash(wikipage.Markdown(markdown)), page.ModTime, nil
}

// DeletePage implements the DeletePage RPC.
func (s *Server) DeletePage(ctx context.Context, req *apiv1.DeletePageRequest) (*apiv1.DeletePageResponse, error) {
	err := s.pageReaderMutator.DeletePage(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "page not found: %s", req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to delete page: %v", err)
	}

	identity := tailscale.IdentityFromContext(ctx)
	s.logger.Info("[AUDIT] delete | page: %q | user: %q", req.PageName, identity.ForLog())

	return &apiv1.DeletePageResponse{
		Success: true,
		Error:   "",
	}, nil
}

// ReadPage implements the ReadPage RPC.
func (s *Server) ReadPage(_ context.Context, req *apiv1.ReadPageRequest) (*apiv1.ReadPageResponse, error) {
	// Read the page markdown and frontmatter
	_, markdown, err := s.pageReaderMutator.ReadMarkdown(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	_, frontmatter, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Convert frontmatter to TOML
	frontmatterToml, err := toml.Marshal(frontmatter)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to marshal frontmatter: %v", err)
	}

	pageText := buildPageText(frontmatter, frontmatterToml, markdown)

	// Create a Page object and render it
	page := &wikipage.Page{
		Identifier: req.PageName,
		Text:       pageText,
	}

	// Render the page if rendering dependencies are available
	var renderedHTML string
	var renderedMarkdown string

	if s.markdownRenderer != nil && s.templateExecutor != nil {
		renderErr := page.Render(s.pageReaderMutator, s.markdownRenderer, s.templateExecutor, s.frontmatterIndexQueryer)
		if renderErr != nil {
			return nil, status.Errorf(codes.Internal, "failed to render page: %v", renderErr)
		}
		renderedHTML = string(page.RenderedPage)
		renderedMarkdown = string(page.RenderedMarkdown)
	}

	return &apiv1.ReadPageResponse{
		ContentMarkdown:         string(markdown),
		FrontMatterToml:         string(frontmatterToml),
		RenderedContentHtml:     renderedHTML,
		RenderedContentMarkdown: renderedMarkdown,
		VersionHash:             computeContentHash(markdown),
	}, nil
}

// RenderMarkdown implements the RenderMarkdown RPC.
// Renders arbitrary markdown content to HTML with chat-safe template macros.
func (s *Server) RenderMarkdown(ctx context.Context, req *apiv1.RenderMarkdownRequest) (*apiv1.RenderMarkdownResponse, error) {
	select {
	case <-ctx.Done():
		return nil, status.FromContextError(ctx.Err()).Err()
	default:
	}

	if req.Content == "" {
		return &apiv1.RenderMarkdownResponse{RenderedHtml: ""}, nil
	}

	if s.markdownRenderer == nil {
		return nil, status.Error(codes.FailedPrecondition, "markdown renderer is not available")
	}

	markdownBytes := []byte(req.Content)

	// Always execute chat-safe template macros, enriching with page frontmatter when available
	frontmatter := make(wikipage.FrontMatter)
	if req.Page != "" {
		_, pageFM, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.Page))
		if err != nil && !os.IsNotExist(err) {
			return nil, status.Errorf(codes.Internal, "failed to read page frontmatter: %v", err)
		}
		if err == nil {
			frontmatter = pageFM
		}
	}

	chatExecutor := server.ChatTemplateExecutor{}
	expanded, err := chatExecutor.ExecuteTemplate(string(markdownBytes), frontmatter, s.pageReaderMutator, s.frontmatterIndexQueryer)
	if err != nil {
		// Template execution failed — render the raw markdown without macros
		s.logger.Warn("chat template execution failed, rendering raw markdown: %v", err)
	} else {
		markdownBytes = expanded
	}

	renderedHTML, err := wikipage.RenderMarkdownToHTML(markdownBytes, s.markdownRenderer)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to render markdown: %v", err)
	}

	return &apiv1.RenderMarkdownResponse{RenderedHtml: string(renderedHTML)}, nil
}

// GenerateIdentifier implements the GenerateIdentifier RPC.
// Converts text to wiki identifier format and checks if it's available.
func (s *Server) GenerateIdentifier(_ context.Context, req *apiv1.GenerateIdentifierRequest) (*apiv1.GenerateIdentifierResponse, error) {
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "text is required")
	}

	// Generate the base identifier
	identifier, err := wikiidentifiers.MungeIdentifier(req.Text)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "cannot generate identifier from text: %v", err)
	}

	// Check if page exists
	isUnique, existingPage := s.checkIdentifierAvailability(identifier)

	// If ensure_unique is requested and page exists, find a unique suffix
	if req.EnsureUnique && !isUnique {
		identifier = s.findUniqueIdentifier(identifier)
		isUnique = true
		existingPage = nil
	}

	return &apiv1.GenerateIdentifierResponse{
		Identifier:   identifier,
		IsUnique:     isUnique,
		ExistingPage: existingPage,
	}, nil
}

// CreatePage implements the CreatePage RPC.
// Creates a new wiki page with optional template support.
func (s *Server) CreatePage(_ context.Context, req *apiv1.CreatePageRequest) (*apiv1.CreatePageResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	identifier, err := wikiidentifiers.MungeIdentifier(req.PageName)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid page name: %v", err)
	}

	_, existingFm, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(identifier))
	if err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "failed to check page existence: %v", err)
	}
	if existingFm != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   fmt.Sprintf("page already exists: %s", identifier),
		}, nil
	}

	fm, err := s.buildNewPageFrontmatter(identifier, req.Template, req.Frontmatter)
	if err != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	markdown := req.ContentMarkdown
	if markdown == "" {
		markdown = wikipage.DefaultPageTemplate
	}

	if err := templating.ValidateTemplate(string(markdown)); err != nil {
		return &apiv1.CreatePageResponse{
			Success: false,
			Error:   fmt.Sprintf(invalidTemplateErrFmt, err),
		}, nil
	}

	if err := s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(identifier), wikipage.FrontMatter(fm)); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(identifier), wikipage.Markdown(markdown)); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, err)
	}

	return &apiv1.CreatePageResponse{
		Success: true,
	}, nil
}

// UpdatePageContent implements the UpdatePageContent RPC.
// Updates only the markdown content of an existing page, preserving its frontmatter.
// If old_content_markdown is provided, performs a find-and-replace: locates the old content
// within the page and substitutes it with new_content_markdown, leaving the rest intact.
// If expected_version_hash is provided, the write will be rejected if the current content
// has changed since the hash was computed (optimistic concurrency control).
// Empty content is rejected; use ClearPageContent to explicitly clear a page's content.
func (s *Server) UpdatePageContent(_ context.Context, req *apiv1.UpdatePageContentRequest) (*apiv1.UpdatePageContentResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	if strings.TrimSpace(req.NewContentMarkdown) == "" {
		return nil, status.Error(codes.InvalidArgument, "new_content_markdown cannot be empty; use ClearPageContent to explicitly clear page content")
	}

	if req.OldContentMarkdown != nil && strings.TrimSpace(*req.OldContentMarkdown) == "" {
		return nil, status.Error(codes.InvalidArgument, "old_content_markdown cannot be empty when provided")
	}

	// Read current content for: (1) page existence check, and (2) rollback data if the
	// post-write invariant check fails.
	_, originalMarkdown, err := s.pageReaderMutator.ReadMarkdown(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, "failed to read current content: %v", err)
	}

	// ModifyMarkdown holds the write lock for the entire hash-check + write cycle,
	// eliminating the TOCTOU race that existed when these were separate operations.
	modifyErr := s.pageReaderMutator.ModifyMarkdown(
		wikipage.PageIdentifier(req.PageName),
		func(currentMarkdown wikipage.Markdown) (wikipage.Markdown, error) {
			// Version hash check is now atomic with the write — no TOCTOU window.
			if err := checkContentVersionHash(currentMarkdown, req.ExpectedVersionHash); err != nil {
				return "", err
			}

			if err := templating.ValidateTemplate(req.NewContentMarkdown); err != nil {
				return "", status.Errorf(codes.InvalidArgument, invalidTemplateErrFmt, err)
			}

			// Compute the markdown to write: find-and-replace when old_content_markdown is set,
			// otherwise replace the entire page content.
			return resolveMarkdownContent(currentMarkdown, req.OldContentMarkdown, req.NewContentMarkdown)
		},
	)
	if modifyErr != nil {
		if _, ok := status.FromError(modifyErr); ok {
			return nil, modifyErr
		}
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, modifyErr)
	}

	// Invariant check: read back the stored content to ensure the write did not blank the page.
	// If the content is missing or empty after a successful write, attempt to restore the
	// original content to prevent data loss.
	storedMarkdown, err := s.verifyStoredContent(wikipage.PageIdentifier(req.PageName), originalMarkdown)
	if err != nil {
		return nil, err
	}

	return &apiv1.UpdatePageContentResponse{
		Success:     true,
		VersionHash: computeContentHash(storedMarkdown),
	}, nil
}

// ClearPageContent implements the ClearPageContent RPC.
// Explicitly clears the markdown content of a page, preserving its frontmatter.
// confirm_clear must be true to prevent accidental data loss.
func (s *Server) ClearPageContent(_ context.Context, req *apiv1.ClearPageContentRequest) (*apiv1.ClearPageContentResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	if !req.ConfirmClear {
		return nil, status.Error(codes.InvalidArgument, "confirm_clear must be true to clear page content")
	}

	// Verify the page exists
	_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(req.PageName), ""); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to clear markdown: %v", err)
	}

	return &apiv1.ClearPageContentResponse{Success: true}, nil
}

// UpdateWholePage implements the UpdateWholePage RPC.
// Replaces the full content of an existing page, including its frontmatter.
// The new_whole_markdown field must contain the complete page text (frontmatter + markdown).
func (s *Server) UpdateWholePage(_ context.Context, req *apiv1.UpdateWholePageRequest) (*apiv1.UpdateWholePageResponse, error) {
	if req.PageName == "" {
		return nil, status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	// Verify the page exists
	_, _, err := s.pageReaderMutator.ReadFrontMatter(wikipage.PageIdentifier(req.PageName))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return nil, status.Errorf(codes.Internal, failedToReadFrontmatterErrFmt, err)
	}

	// Parse frontmatter and markdown from the combined content
	page := &wikipage.Page{
		Identifier: req.PageName,
		Text:       req.NewWholeMarkdown,
	}

	fm, err := page.GetFrontMatter()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse frontmatter: %v", err)
	}

	md, err := page.GetMarkdown()
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "failed to parse markdown: %v", err)
	}

	if err := templating.ValidateTemplate(string(md)); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, invalidTemplateErrFmt, err)
	}

	// Preserve the page identifier in frontmatter
	if fm == nil {
		fm = make(map[string]any)
	}
	fm[identifierKey] = req.PageName

	if err := s.pageReaderMutator.WriteMarkdown(wikipage.PageIdentifier(req.PageName), md); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteMarkdownErrFmt, err)
	}

	if err := s.pageReaderMutator.WriteFrontMatter(wikipage.PageIdentifier(req.PageName), fm); err != nil {
		return nil, status.Errorf(codes.Internal, failedToWriteFrontmatterErrFmt, err)
	}

	return &apiv1.UpdateWholePageResponse{Success: true}, nil
}

// ListTemplates implements the ListTemplates RPC.
// Returns all pages marked as templates (with template: true frontmatter).
func (s *Server) ListTemplates(_ context.Context, req *apiv1.ListTemplatesRequest) (*apiv1.ListTemplatesResponse, error) {
	// Build exclusion set
	excludeSet := make(map[string]bool)
	for _, id := range req.ExcludeIdentifiers {
		excludeSet[id] = true
	}

	// Query pages with template: true
	templatePages := s.frontmatterIndexQueryer.QueryExactMatch("template", "true")

	templates := make([]*apiv1.TemplateInfo, 0, len(templatePages))
	for _, pageID := range templatePages {
		// Skip excluded identifiers
		if excludeSet[string(pageID)] {
			continue
		}

		// Read frontmatter to get title and description
		_, fm, err := s.pageReaderMutator.ReadFrontMatter(pageID)
		if err != nil {
			// Skip pages that can't be read
			continue
		}

		template := &apiv1.TemplateInfo{
			Identifier: string(pageID),
		}

		// Get title
		if title, ok := fm["title"].(string); ok {
			template.Title = title
		}

		// Get description
		if desc, ok := fm["description"].(string); ok {
			template.Description = desc
		}

		templates = append(templates, template)
	}

	return &apiv1.ListTemplatesResponse{
		Templates: templates,
	}, nil
}

// WatchPage implements the WatchPage RPC for real-time page content change notifications.
// It streams the current version_hash of the page content when it changes.
func (s *Server) WatchPage(req *apiv1.WatchPageRequest, stream apiv1.PageManagementService_WatchPageServer) error {
	if req.PageName == "" {
		return status.Error(codes.InvalidArgument, pageNameRequiredErr)
	}

	pageID := wikipage.PageIdentifier(req.PageName)
	interval := resolveCheckInterval(req.GetCheckIntervalMs())

	// Read initial content hash and mod time
	hash, modTime, err := s.readPageHashAndModTime(pageID)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Errorf(codes.NotFound, pageNotFoundErrFmt, req.PageName)
		}
		return status.Errorf(codes.Internal, "failed to read page: %v", err)
	}

	lastHash := hash

	// Send initial hash and mod time immediately
	if err := stream.Send(&apiv1.WatchPageResponse{
		VersionHash:  lastHash,
		LastModified: timestamppb.New(modTime),
	}); err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Stream updates when content changes
	for {
		select {
		case <-ticker.C:
			newHash, err := s.sendPageUpdateIfChanged(stream, pageID, lastHash)
			if err != nil {
				return err
			}
			lastHash = newHash

		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}
