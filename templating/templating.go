package templating

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
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

type InventoryFrontmatter struct {
	Container string   `json:"container"`
	Items     []string `json:"items"`
}


type TemplateContext struct {
	// CAUTION: avoid changing the structure of TemplateContext without considering backward compatibility.
	// If you change the structure, consider adding a migration to handle existing pages that may rely on the old structure.
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Map        map[string]any
	Inventory  InventoryFrontmatter `json:"inventory"`
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
		uniqueItems[wikiidentifiers.MungeIdentifier(item)] = true
	}

	// Add new items to the map (protected from circular references)
	itemsFromIndex := query.QueryExactMatch("inventory.container", templateContext.Identifier)
	
	
	for _, item := range itemsFromIndex {
		// If there was an item that existed as a title in the list of items, remove it.
		// This is to support the workflow of items first being listed directly on the inventory container,
		// but later getting their own page and being linked to the inventory container through the inventory.container frontmatter key.
		itemTitle := query.GetValue(item, "title")
		delete(uniqueItems, itemTitle)

		uniqueItems[item] = true
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
	maxRecursionDepth        = 10
	maxInventoryDepth        = 10               // Maximum depth for recursive inventory traversal  
	templateExecutionTimeout = 30 * time.Second // Timeout for template execution
	timeoutMessage           = "  [Template execution timeout]"
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

	_, containerFrontmatter, err := site.ReadFrontMatter(containerIdentifier)
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
		"__Indent":                func() string { return strings.Repeat(" ", indent*2) },
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

		// Check for circular reference to prevent infinite recursion
		if visited[identifierToLink] {
			// Return a safe fallback link without triggering template execution - use munged identifier for URL
			titleCaser := cases.Title(language.AmericanEnglish)
			titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", " "))
			mungedIdentifier := wikiidentifiers.MungeIdentifier(identifierToLink)
			return "[" + titleCasedTitle + " (circular reference)](/" + mungedIdentifier + ")"
		}

		// Mark this page as visited to prevent recursion
		visited[identifierToLink] = true
		defer func() {
			delete(visited, identifierToLink)
		}()

		identifierToLink, frontmatterForLinkedPage, err := site.ReadFrontMatter(identifierToLink)
		if err != nil {
			titleCaser := cases.Title(language.AmericanEnglish)
			titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", " "))
			urlEncodedTitle := url.QueryEscape(titleCasedTitle)
			// Doesnt look like it exists yet, return a link.
			// It'll render and let the page get created.
			if isContainer(currentPageTemplateContext.Identifier) {
				// special inventory item link with attributes - use munged identifier for URL
				mungedIdentifier := wikiidentifiers.MungeIdentifier(identifierToLink)
				return "[" + titleCasedTitle + "](/" + mungedIdentifier + "?tmpl=inv_item&inventory.container=" + currentPageTemplateContext.Identifier + "&title=" + urlEncodedTitle + ")"
			}

			// Use munged identifier for URL
			mungedIdentifier := wikiidentifiers.MungeIdentifier(identifierToLink)
			return "[" + titleCasedTitle + "](/" + mungedIdentifier + "?title=" + urlEncodedTitle + ")"
		}

		// Linked Page Exists - use munged identifier for URL
		mungedIdentifier := wikiidentifiers.MungeIdentifier(identifierToLink)
		tmplString := "{{if index . \"title\"}}[{{ index . \"title\" }}](/" + mungedIdentifier + "){{else}}[{{ index . \"identifier\" }}](/" + mungedIdentifier + "){{end}}"
		tmpl, err := template.New("content").Parse(tmplString)
		if err != nil {
			return err.Error()
		}

		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, frontmatterForLinkedPage)
		if err != nil {
			return err.Error()
		}

		return buf.String()
	}
}

func BuildIsContainer(query wikipage.IQueryFrontmatterIndex) func(string) bool {
	return func(identifier string) bool {
		if identifier == "" {
			return false
		}

		if len(query.QueryExactMatch("inventory.container", identifier)) > 0 {
			return true
		}

		if query.GetValue(identifier, "inventory.items") != "" {
			return true
		}

		return false
	}
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
		return nil, fmt.Errorf("template execution timed out after %v", templateExecutionTimeout)
	default:
	}

	templateContext, err := ConstructTemplateContextFromFrontmatterWithVisited(fm, query, visited)
	if err != nil {
		return nil, fmt.Errorf("failed to construct template context: %w", err)
	}

	// Check context cancellation after frontmatter construction
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("template execution timed out after %v", templateExecutionTimeout)
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
		"ShowInventoryContentsOf": BuildShowInventoryContentsOfWithContext(ctx, site, query, 0),
		"LinkTo":                  BuildLinkTo(site, templateContext, query),
		"IsContainer":             BuildIsContainer(query),
		"FindBy":                  query.QueryExactMatch,
		"FindByPrefix":            query.QueryPrefixMatch,
		"FindByKeyExistence":      query.QueryKeyExistence,
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


