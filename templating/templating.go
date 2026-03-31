package templating

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/brendanjerwin/simple_wiki/wikiidentifiers"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/stoewer/go-strcase"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

const singleSpace = " "

type InventoryFrontmatter struct {
	Container string   `json:"container"`
	Items     []string `json:"items"`
}


type TemplateContext struct {
	// CAUTION: avoid changing the structure of TemplateContext without considering backward compatibility.
	// If you change the structure, consider adding a migration to handle existing pages that may rely on the old structure.
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Map         map[string]any
	Inventory   InventoryFrontmatter `json:"inventory"`
}

func ConstructTemplateContextFromFrontmatter(fm wikipage.FrontMatter, query wikipage.IQueryFrontmatterIndex) (TemplateContext, error) {
	return ConstructTemplateContextFromFrontmatterWithVisited(fm, query, make(map[string]bool))
}

func ConstructTemplateContextFromFrontmatterWithVisited(fm wikipage.FrontMatter, query wikipage.IQueryFrontmatterIndex, visited map[string]bool) (TemplateContext, error) {
	fmBytes, err := json.Marshal(fm)
	if err != nil {
		return TemplateContext{}, err
	}

	templateContext := TemplateContext{}
	err = json.Unmarshal(fmBytes, &templateContext)
	if err != nil {
		return TemplateContext{}, err
	}

	templateContext.Map = fm

	// Check for circular reference in inventory processing
	if visited[templateContext.Identifier] {
		// Return context without processing inventory items to prevent infinite recursion
		if templateContext.Inventory.Items == nil {
			templateContext.Inventory.Items = []string{}
		}
		return templateContext, nil
	}

	// Mark this identifier as visited
	visited[templateContext.Identifier] = true
	defer func() {
		delete(visited, templateContext.Identifier)
	}()

	if templateContext.Inventory.Items == nil {
		templateContext.Inventory.Items = []string{}
	}

	// Create a map to store unique items
	uniqueItems := make(map[string]bool)

	// Add existing items to the map
	for _, item := range templateContext.Inventory.Items {
		mungedItem, err := wikiidentifiers.MungeIdentifier(item)
		if err != nil {
			return TemplateContext{}, fmt.Errorf("munging inventory item %q: %w", item, err)
		}
		uniqueItems[mungedItem] = true
	}

	// Add new items to the map (protected from circular references)
	itemsFromIndex := query.QueryExactMatch("inventory.container", templateContext.Identifier)
	
	
	for _, item := range itemsFromIndex {
		// If there was an item that existed as a title in the list of items, remove it.
		// This is to support the workflow of items first being listed directly on the inventory container,
		// but later getting their own page and being linked to the inventory container through the inventory.container frontmatter key.
		itemTitle := query.GetValue(item, "title")
		delete(uniqueItems, itemTitle)

		uniqueItems[string(item)] = true
	}

	// Convert the map back to a slice
	templateContext.Inventory.Items = make([]string, 0, len(uniqueItems))
	for item := range uniqueItems {
		templateContext.Inventory.Items = append(templateContext.Inventory.Items, item)
	}

	sort.Strings(templateContext.Inventory.Items)

	return templateContext, nil
}

const (
	maxInventoryDepth        = 10               // Maximum depth for recursive inventory traversal
	templateExecutionTimeout = 30 * time.Second // Timeout for template execution
	timeoutMessage           = "  [Template execution timeout]"

	// Template function name constants (used in FuncMaps and validation stubs)
	funcNameShowInventory    = "ShowInventoryContentsOf"
	funcNameLinkTo           = "LinkTo"
	funcNameIsContainer      = "IsContainer"
	funcNameFindBy           = "FindBy"
	funcNameFindByPrefix     = "FindByPrefix"
	funcNameFindByKeyExists  = "FindByKeyExistence"
	funcNameChecklist        = "Checklist"
	funcNameBlog             = "Blog"

	templateTimeoutErrFmt = "template execution timed out after %v"
)


func BuildShowInventoryContentsOf(site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, indent int) func(string) string {
	// Create a background context for backward compatibility
	ctx := context.Background()
	return BuildShowInventoryContentsOfWithContext(ctx, site, query, indent)
}

func BuildShowInventoryContentsOfWithContext(ctx context.Context, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, indent int) func(string) string {
	isContainer := BuildIsContainer(query)

	return func(containerIdentifier string) string {
		// Simple depth protection - prevent infinite recursion in circular references
		if indent > maxInventoryDepth {
			return "  [Maximum depth reached]"
		}

		// Check context cancellation before processing
		select {
		case <-ctx.Done():
			return timeoutMessage
		default:
		}

		return buildShowInventoryContentsOfSync(ctx, site, query, containerIdentifier, indent, isContainer)
	}
}

func buildShowInventoryContentsOfSync(ctx context.Context, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, containerIdentifier string, indent int, isContainer func(string) bool) string {
	// Check context cancellation at the start
	select {
	case <-ctx.Done():
		return timeoutMessage
	default:
	}

	_, containerFrontmatter, err := site.ReadFrontMatter(wikipage.PageIdentifier(containerIdentifier))
	if err != nil {
		return err.Error()
	}
	
	// Check context cancellation after reading frontmatter
	select {
	case <-ctx.Done():
		return timeoutMessage
	default:
	}
	
	// Use the simple version without visited map complexity
	containerTemplateContext, err := ConstructTemplateContextFromFrontmatter(containerFrontmatter, query)
	if err != nil {
		return err.Error()
	}

	tmplString := `
{{ range .Inventory.Items }}
{{ if IsContainer . }}
{{ __Indent }} - **{{ LinkTo . }}**
{{ ShowInventoryContentsOf . }}
{{ else }}
{{ __Indent }} - {{ LinkTo . }}
{{ end }}
{{ end }}
`
	// Functions with recursive ShowInventoryContentsOf for nested containers
	funcs := template.FuncMap{
		"LinkTo":                  BuildLinkTo(site, containerTemplateContext, query),
		"ShowInventoryContentsOf": BuildShowInventoryContentsOfWithContext(ctx, site, query, indent+1),
		"IsContainer":             isContainer,
		"FindBy":                  query.QueryExactMatch,
		"FindByPrefix":            query.QueryPrefixMatch,
		"FindByKeyExistence":      query.QueryKeyExistence,
		"__Indent":                func() string { return strings.Repeat(singleSpace, indent*2) },
	}

	tmpl, err := template.New("content").Funcs(funcs).Parse(tmplString)
	if err != nil {
		return err.Error()
	}

	// Check context cancellation before template execution
	select {
	case <-ctx.Done():
		return timeoutMessage
	default:
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, containerTemplateContext)
	if err != nil {
		return err.Error()
	}

	return buf.String()
}

func BuildLinkTo(site wikipage.PageReader, currentPageTemplateContext TemplateContext, query wikipage.IQueryFrontmatterIndex) func(string) string {
	// Legacy function without visited map for backward compatibility
	return BuildLinkToWithVisited(site, currentPageTemplateContext, query, make(map[string]bool))
}

func BuildLinkToWithVisited(site wikipage.PageReader, currentPageTemplateContext TemplateContext, query wikipage.IQueryFrontmatterIndex, visited map[string]bool) func(string) string {
	isContainer := BuildIsContainer(query)
	return func(identifierToLink string) string {
		if identifierToLink == "" {
			return "N/A"
		}

		if visited[identifierToLink] {
			return buildCircularReferenceLink(identifierToLink)
		}

		visited[identifierToLink] = true
		defer func() {
			delete(visited, identifierToLink)
		}()

		resolvedIdentifier, frontmatterForLinkedPage, err := site.ReadFrontMatter(wikipage.PageIdentifier(identifierToLink))
		if err != nil {
			if isContainer(currentPageTemplateContext.Identifier) {
				return buildNewContainerPageLink(identifierToLink, currentPageTemplateContext.Identifier)
			}
			return buildNewPageLink(identifierToLink)
		}

		return buildExistingPageLink(string(resolvedIdentifier), frontmatterForLinkedPage)
	}
}

func buildCircularReferenceLink(identifierToLink string) string {
	titleCaser := cases.Title(language.AmericanEnglish)
	titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", singleSpace))
	mungedIdentifier, err := wikiidentifiers.MungeIdentifier(identifierToLink)
	if err != nil {
		return fmt.Sprintf("[ERROR: LinkTo circular reference, munging %q: %v]", identifierToLink, err)
	}
	return "[" + titleCasedTitle + " (circular reference)](/" + mungedIdentifier + ")"
}

func buildNewPageLink(identifierToLink string) string {
	titleCaser := cases.Title(language.AmericanEnglish)
	titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", singleSpace))
	urlEncodedTitle := url.QueryEscape(titleCasedTitle)
	mungedIdentifier, err := wikiidentifiers.MungeIdentifier(identifierToLink)
	if err != nil {
		return fmt.Sprintf("[ERROR: LinkTo new page, munging %q: %v]", identifierToLink, err)
	}
	return "[" + titleCasedTitle + "](/" + mungedIdentifier + "?title=" + urlEncodedTitle + ")"
}

func buildNewContainerPageLink(identifierToLink, containerIdentifier string) string {
	titleCaser := cases.Title(language.AmericanEnglish)
	titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", singleSpace))
	urlEncodedTitle := url.QueryEscape(titleCasedTitle)
	mungedIdentifier, err := wikiidentifiers.MungeIdentifier(identifierToLink)
	if err != nil {
		return fmt.Sprintf("[ERROR: LinkTo new inventory item, munging %q: %v]", identifierToLink, err)
	}
	return "[" + titleCasedTitle + "](/" + mungedIdentifier + "?tmpl=inv_item&inventory.container=" + containerIdentifier + "&title=" + urlEncodedTitle + ")"
}

func buildExistingPageLink(resolvedIdentifier string, frontmatter wikipage.FrontMatter) string {
	mungedIdentifier, err := wikiidentifiers.MungeIdentifier(resolvedIdentifier)
	if err != nil {
		return fmt.Sprintf("[ERROR: LinkTo existing page, munging %q: %v]", resolvedIdentifier, err)
	}
	tmplString := "{{if index . \"title\"}}[{{ index . \"title\" }}](/" + mungedIdentifier + "){{else}}[{{ index . \"identifier\" }}](/" + mungedIdentifier + "){{end}}"
	tmpl, parseErr := template.New("content").Parse(tmplString)
	if parseErr != nil {
		return parseErr.Error()
	}
	buf := &bytes.Buffer{}
	if execErr := tmpl.Execute(buf, frontmatter); execErr != nil {
		return execErr.Error()
	}
	return buf.String()
}

// BuildChecklist returns a template function that renders a wiki-checklist custom element
// with a server-rendered fallback list of checklist items inside it.
func BuildChecklist(templateContext TemplateContext) func(string) string {
	return func(listName string) string {
		fallback := renderChecklistFallback(templateContext.Map, listName)
		return fmt.Sprintf(`<wiki-checklist list-name="%s" page="%s">%s</wiki-checklist>`,
			html.EscapeString(listName),
			html.EscapeString(templateContext.Identifier),
			fallback,
		)
	}
}

func renderChecklistFallback(frontmatter map[string]any, listName string) string {
	checklists, ok := frontmatter["checklists"].(map[string]any)
	if !ok {
		return ""
	}
	list, ok := checklists[listName].(map[string]any)
	if !ok {
		return ""
	}
	items, ok := list["items"].([]any)
	if !ok || len(items) == 0 {
		return ""
	}

	var buf strings.Builder
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		text, textOk := item["text"].(string)
		if !textOk || text == "" {
			continue
		}
		checked, checkedOk := item["checked"].(bool)
		if !checkedOk {
			checked = false
		}
		marker := "[ ]"
		if checked {
			marker = "[x]"
		}
		// Single-line plain text to avoid goldmark paragraph breaks.
		_, _ = fmt.Fprintf(&buf, `<span class="checklist-item">%s %s</span>`, marker, html.EscapeString(text))
	}
	return buf.String()
}

const blogSnippetMaxChars = 200

// BuildBlog returns a template function that renders a wiki-blog custom element
// with a server-rendered fallback list of blog posts inside it.
func BuildBlog(templateContext TemplateContext, query wikipage.IQueryFrontmatterIndex, site wikipage.PageReader) func(string, int) string {
	return func(blogIdentifier string, maxArticles int) string {
		posts := query.QueryExactMatchSortedBy("blog.identifier", blogIdentifier, "blog.published-date", false, maxArticles)

		var articlesBuf strings.Builder
		for _, postID := range posts {
			_, _ = articlesBuf.WriteString(renderBlogArticle(postID, query, site))
		}
		articles := articlesBuf.String()

		hideNewPost := ""
		if blogMap, ok := templateContext.Map["blog"].(map[string]any); ok {
			switch v := blogMap["hide-new-post"].(type) {
			case bool:
				if v {
					hideNewPost = ` hide-new-post`
				}
			case string:
				if v == "true" {
					hideNewPost = ` hide-new-post`
				}
			}
		}

		return fmt.Sprintf(`<wiki-blog blog-id="%s" max-articles="%d" page="%s"%s>%s</wiki-blog>`,
			html.EscapeString(blogIdentifier),
			maxArticles,
			html.EscapeString(templateContext.Identifier),
			hideNewPost,
			articles,
		)
	}
}

func renderBlogArticle(postID wikipage.PageIdentifier, query wikipage.IQueryFrontmatterIndex, site wikipage.PageReader) string {
	title := query.GetValue(postID, "title")
	if title == "" {
		title = string(postID)
	}
	publishedDate := query.GetValue(postID, "blog.published-date")
	subtitle := query.GetValue(postID, "blog.subtitle")
	externalURL := query.GetValue(postID, "blog.external_url")
	snippet := blogSnippet(postID, query, site)

	linkHref := "/" + html.EscapeString(string(postID))
	if externalURL != "" && isSafeURL(externalURL) {
		linkHref = html.EscapeString(externalURL)
	}

	article := fmt.Sprintf(`<span class="blog-article"><a href="%s">%s</a>`, linkHref, html.EscapeString(title))
	if externalURL != "" {
		article += fmt.Sprintf(` <a href="/%s" class="wiki-link">[wiki]</a>`, html.EscapeString(string(postID)))
	}
	if subtitle != "" {
		article += fmt.Sprintf(` <span class="subtitle">%s</span>`, html.EscapeString(subtitle))
	}
	if publishedDate != "" {
		article += fmt.Sprintf(` <span class="date">%s</span>`, html.EscapeString(publishedDate))
	}
	if snippet != "" {
		article += fmt.Sprintf(` <span class="snippet">%s</span>`, html.EscapeString(snippet))
	}
	article += "</span>"
	return article
}

// isSafeURL checks that a URL uses a safe scheme (http or https).
// Rejects javascript:, data:, and other schemes that could be XSS vectors.
func isSafeURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	scheme := strings.ToLower(u.Scheme)
	return scheme == "http" || scheme == "https"
}

// markdownSyntax matches common markdown syntax characters that would be
// interpreted by goldmark if left in the server-rendered fallback HTML.
var markdownSyntax = regexp.MustCompile(`(?m)^#{1,6}\s+|[*_~` + "`" + `\[\]]`)

func blogSnippet(postID wikipage.PageIdentifier, query wikipage.IQueryFrontmatterIndex, site wikipage.PageReader) string {
	raw := query.GetValue(postID, "blog.summary_markdown")
	if raw == "" {
		_, markdown, err := site.ReadMarkdown(postID)
		if err != nil || len(markdown) == 0 {
			return ""
		}
		raw = string(markdown)
	}
	runes := []rune(raw)
	if len(runes) > blogSnippetMaxChars {
		raw = string(runes[:blogSnippetMaxChars])
	}
	// Collapse to a single line of plain text so goldmark cannot interpret
	// markdown syntax or create paragraph breaks inside the fallback HTML.
	// strings.Fields splits on all whitespace (including \r\n, \n) and
	// strings.Join reassembles with single spaces — no explicit newline replacement needed.
	raw = markdownSyntax.ReplaceAllString(raw, "")
	raw = strings.Join(strings.Fields(raw), singleSpace)
	return raw
}

func BuildIsContainer(query wikipage.IQueryFrontmatterIndex) func(string) bool {
	return func(identifier string) bool {
		if identifier == "" {
			return false
		}

		// Primary: Check explicit is_container field
		if query.GetValue(wikipage.PageIdentifier(identifier), "inventory.is_container") == "true" {
			return true
		}

		// Fallback for legacy: items reference this as their container
		if len(query.QueryExactMatch("inventory.container", identifier)) > 0 {
			return true
		}

		return false
	}
}

// ExecuteChatTemplate executes a template string with a restricted set of macros
// suitable for chat messages. Excludes interactive widget macros (Checklist, Blog)
// that render web components inappropriate for chat bubbles. Includes timeout protection.
func ExecuteChatTemplate(templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), templateExecutionTimeout)
	defer cancel()

	return executeChatTemplateWorker(ctx, templateString, fm, site, query, make(map[string]bool))
}

func executeChatTemplateWorker(ctx context.Context, templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, visited map[string]bool) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf(templateTimeoutErrFmt, templateExecutionTimeout)
	default:
	}

	templateContext, err := ConstructTemplateContextFromFrontmatterWithVisited(fm, query, visited)
	if err != nil {
		return nil, fmt.Errorf("failed to construct template context: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf(templateTimeoutErrFmt, templateExecutionTimeout)
	default:
	}

	tmpl, err := buildChatTemplateWithFunctions(ctx, templateString, site, query, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to build template: %w", err)
	}

	return executeTemplateInternal(ctx, tmpl, templateContext)
}

// buildChatTemplateWithFunctions creates a template with chat-safe functions only.
// Excludes Checklist and Blog which render interactive web components.
func buildChatTemplateWithFunctions(ctx context.Context, templateString string, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, templateContext TemplateContext) (*template.Template, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	funcs := template.FuncMap{
		funcNameShowInventory:   BuildShowInventoryContentsOfWithContext(ctx, site, query, 0),
		funcNameLinkTo:          BuildLinkTo(site, templateContext, query),
		funcNameIsContainer:     BuildIsContainer(query),
		funcNameFindBy:          query.QueryExactMatch,
		funcNameFindByPrefix:    query.QueryPrefixMatch,
		funcNameFindByKeyExists: query.QueryKeyExistence,
		// Stubs for interactive widget macros that are not supported in chat context.
		// These prevent parse errors when messages contain Checklist or Blog macros.
		funcNameChecklist: func(string) string { return "" },
		funcNameBlog:      func(string, int) string { return "" },
	}

	return template.New("page").Funcs(funcs).Parse(templateString)
}

// ExecuteTemplate executes a template string with the given frontmatter and site context.
// Includes timeout protection to prevent infinite hangs.
func ExecuteTemplate(templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex) ([]byte, error) {
	// Set a reasonable timeout for template execution to prevent hangs
	ctx, cancel := context.WithTimeout(context.Background(), templateExecutionTimeout)
	defer cancel()

	// Create a new visited map for this template execution to prevent circular references
	return executeTemplateWorker(ctx, templateString, fm, site, query, make(map[string]bool))
}






// executeTemplateWorker performs the actual template execution with context cancellation support.
func executeTemplateWorker(ctx context.Context, templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, visited map[string]bool) ([]byte, error) {
	// Check context cancellation before starting
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf(templateTimeoutErrFmt, templateExecutionTimeout)
	default:
	}

	templateContext, err := ConstructTemplateContextFromFrontmatterWithVisited(fm, query, visited)
	if err != nil {
		return nil, fmt.Errorf("failed to construct template context: %w", err)
	}

	// Check context cancellation after frontmatter construction
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf(templateTimeoutErrFmt, templateExecutionTimeout)
	default:
	}

	tmpl, err := buildTemplateWithFunctions(ctx, templateString, site, query, templateContext)
	if err != nil {
		return nil, fmt.Errorf("failed to build template: %w", err)
	}

	return executeTemplateInternal(ctx, tmpl, templateContext)
}


// buildTemplateWithFunctions creates a template with all necessary functions.
func buildTemplateWithFunctions(ctx context.Context, templateString string, site wikipage.PageReader, query wikipage.IQueryFrontmatterIndex, templateContext TemplateContext) (*template.Template, error) {
	// Check context cancellation before building functions
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	funcs := template.FuncMap{
		funcNameShowInventory:   BuildShowInventoryContentsOfWithContext(ctx, site, query, 0),
		funcNameLinkTo:          BuildLinkTo(site, templateContext, query),
		funcNameIsContainer:     BuildIsContainer(query),
		funcNameFindBy:          query.QueryExactMatch,
		funcNameFindByPrefix:    query.QueryPrefixMatch,
		funcNameFindByKeyExists: query.QueryKeyExistence,
		funcNameChecklist:       BuildChecklist(templateContext),
		funcNameBlog:            BuildBlog(templateContext, query, site),
	}

	return template.New("page").Funcs(funcs).Parse(templateString)
}

// executeTemplateInternal executes the template and returns the result.
func executeTemplateInternal(ctx context.Context, tmpl *template.Template, templateContext TemplateContext) ([]byte, error) {
	// Check context cancellation before template execution
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	buf := &bytes.Buffer{}
	err := tmpl.Execute(buf, templateContext)
	if err != nil {
		return nil, fmt.Errorf("template execution failed: %w", err)
	}

	return buf.Bytes(), nil
}

// validationFuncMap returns a FuncMap with stub implementations for all
// registered template functions. Used for parse-only validation so that we
// can detect unknown function names without executing any real logic.
// This map must be kept in sync with the runtime FuncMap in buildTemplateWithFunctions.
func validationFuncMap() template.FuncMap {
	return template.FuncMap{
		funcNameShowInventory:   func(string) string { return "" },
		funcNameLinkTo:          func(string) string { return "" },
		funcNameIsContainer:     func(string) bool { return false },
		funcNameFindBy:          func(string, string) []string { return nil },
		funcNameFindByPrefix:    func(string, string) []string { return nil },
		funcNameFindByKeyExists: func(string) []string { return nil },
		funcNameChecklist:       func(string) string { return "" },
		funcNameBlog:            func(string, int) string { return "" },
	}
}

// knownTemplateFuncNames returns the sorted list of all registered template
// function names. Derived from validationFuncMap to avoid drift.
func knownTemplateFuncNames() []string {
	fm := validationFuncMap()
	names := make([]string, 0, len(fm))
	for name := range fm {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// templatePositionRe matches the position prefix produced by text/template parse errors:
// "template: NAME:LINE:COL: …" or "template: NAME:LINE: …"
var templatePositionRe = regexp.MustCompile(`^template:\s+[^:]+:(\d+)(?::(\d+))?:`)

// ValidateTemplate parses the markdown as a Go text/template and verifies that
// every function call refers to a registered macro. It returns nil when the
// template is valid, or an error with a human-readable message that includes:
//   - the name of the unknown macro
//   - its line (and column) position in the source
//   - a "Did you mean …?" suggestion for common misspellings (e.g. wrong case)
//   - the full list of available macros
func ValidateTemplate(markdown string) error {
	_, err := template.New("page").Funcs(validationFuncMap()).Parse(markdown)
	if err == nil {
		return nil
	}
	return formatTemplateValidationError(err)
}

// formatTemplateValidationError turns a raw text/template parse error into a
// user-friendly message. It specifically handles the "function not defined"
// case with position info and suggestions; all other parse errors are wrapped
// with a brief prefix.
func formatTemplateValidationError(parseErr error) error {
	errStr := parseErr.Error()

	const funcNotDefinedMarker = `function "`
	if idx := strings.Index(errStr, funcNotDefinedMarker); idx >= 0 {
		rest := errStr[idx+len(funcNotDefinedMarker):]
		if endIdx := strings.Index(rest, `"`); endIdx >= 0 {
			unknownFunc := rest[:endIdx]

			posInfo := extractTemplatePosition(errStr)
			known := knownTemplateFuncNames()
			suggestion := findMacroSuggestion(unknownFunc, known)

			var msg strings.Builder
			_, _ = msg.WriteString(fmt.Sprintf("invalid macro %q", unknownFunc))
			if posInfo != "" {
				_, _ = msg.WriteString(" at " + posInfo)
			}
			if suggestion != "" {
				_, _ = msg.WriteString(fmt.Sprintf("; did you mean %q?", suggestion))
			}
			_, _ = msg.WriteString("; available macros: " + strings.Join(known, ", "))

			return errors.New(msg.String())
		}
	}

	return fmt.Errorf("invalid template syntax: %w", parseErr)
}

// extractTemplatePosition parses the position prefix from a text/template error
// string and returns a human-readable "line N, column M" string, or "" if the
// position cannot be determined.
func extractTemplatePosition(errStr string) string {
	m := templatePositionRe.FindStringSubmatch(errStr)
	if m == nil {
		return ""
	}
	if m[2] != "" {
		return fmt.Sprintf("line %s, column %s", m[1], m[2])
	}
	return fmt.Sprintf("line %s", m[1])
}

// findMacroSuggestion returns the known macro name that most closely matches
// the unknown name, or "" when no close match exists.
// Currently checks for an exact case-insensitive match (the most common mistake).
func findMacroSuggestion(unknown string, known []string) string {
	lowerUnknown := strings.ToLower(unknown)
	for _, k := range known {
		if strings.ToLower(k) == lowerUnknown {
			return k
		}
	}
	return ""
}

