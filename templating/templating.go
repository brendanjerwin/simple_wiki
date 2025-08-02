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

	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
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
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	FrontmatterMap        map[string]any
	Inventory  InventoryFrontmatter `json:"inventory"`
}

func ConstructTemplateContextFromFrontmatter(fm wikipage.FrontMatter, query frontmatter.IQueryFrontmatterIndex) (TemplateContext, error) {
	return ConstructTemplateContextFromFrontmatterWithVisited(fm, query, make(map[string]bool))
}

func ConstructTemplateContextFromFrontmatterWithVisited(fm wikipage.FrontMatter, query frontmatter.IQueryFrontmatterIndex, visited map[string]bool) (TemplateContext, error) {
	fmBytes, err := json.Marshal(fm)
	if err != nil {
		return TemplateContext{}, err
	}

	templateContext := TemplateContext{}
	err = json.Unmarshal(fmBytes, &templateContext)
	if err != nil {
		return TemplateContext{}, err
	}

	templateContext.FrontmatterMap = fm

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
	maxRecursionDepth = 10
	templateExecutionTimeout = 30 * time.Second // Timeout for template execution
)

func BuildShowInventoryContentsOf(site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex, indent int) func(string) string {
	return BuildShowInventoryContentsOfWithLimit(site, query, indent, maxRecursionDepth, make(map[string]bool))
}

func BuildShowInventoryContentsOfWithVisited(site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex, indent int, visited map[string]bool) func(string) string {
	return BuildShowInventoryContentsOfWithLimit(site, query, indent, maxRecursionDepth, visited)
}

func BuildShowInventoryContentsOfWithLimit(site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex, indent int, maxDepth int, visited map[string]bool) func(string) string {
	isContainer := BuildIsContainer(query)

	return func(containerIdentifier string) string {
		// Check for circular reference
		if visited[containerIdentifier] {
			return "  [Circular reference detected]"
		}

		// Check for maximum recursion depth
		if indent > maxDepth {
			return "  [Maximum depth reached]"
		}

		// Mark this container as visited
		visited[containerIdentifier] = true
		defer func() {
			delete(visited, containerIdentifier)
		}()

		_, containerFrontmatter, err := site.ReadFrontMatter(containerIdentifier)
		if err != nil {
			return err.Error()
		}
		containerTemplateContext, err := ConstructTemplateContextFromFrontmatterWithVisited(containerFrontmatter, query, visited)
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
		// Pass the shared visited map to template functions to prevent circular references
		funcs := template.FuncMap{
			"LinkTo":                  BuildLinkToWithVisited(site, containerTemplateContext, query, visited),
			"ShowInventoryContentsOf": BuildShowInventoryContentsOfWithLimit(site, query, indent+1, maxDepth, visited),
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

		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, containerTemplateContext)
		if err != nil {
			return err.Error()
		}

		return buf.String()
	}
}

func BuildLinkTo(site wikipage.PageReader, currentPageTemplateContext TemplateContext, query frontmatter.IQueryFrontmatterIndex) func(string) string {
	// Legacy function without visited map for backward compatibility
	return BuildLinkToWithVisited(site, currentPageTemplateContext, query, make(map[string]bool))
}

func BuildLinkToWithVisited(site wikipage.PageReader, currentPageTemplateContext TemplateContext, query frontmatter.IQueryFrontmatterIndex, visited map[string]bool) func(string) string {
	isContainer := BuildIsContainer(query)
	return func(identifierToLink string) string {
		if identifierToLink == "" {
			return "N/A"
		}

		// Check for circular reference to prevent infinite recursion
		if visited[identifierToLink] {
			// Return a safe fallback link without triggering template execution
			titleCaser := cases.Title(language.AmericanEnglish)
			titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", " "))
			return "[" + titleCasedTitle + " (circular reference)](/" + identifierToLink + ")"
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
				// special inventory item link with attributes
				return "[" + titleCasedTitle + "](/" + identifierToLink + "?tmpl=inv_item&inventory.container=" + currentPageTemplateContext.Identifier + "&title=" + urlEncodedTitle + ")"
			}

			return "[" + titleCasedTitle + "](/" + identifierToLink + "?title=" + urlEncodedTitle + ")"
		}

		// Linked Page Exists
		tmplString := "{{if index . \"title\"}}[{{ index . \"title\" }}](/{{ index . \"identifier\" }}){{else}}[{{ index . \"identifier\" }}](/{{ index . \"identifier\" }}){{end}}"
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

func BuildIsContainer(query frontmatter.IQueryFrontmatterIndex) func(string) bool {
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
func ExecuteTemplate(templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex) ([]byte, error) {
	// Create a new visited map for this template execution context to prevent circular references
	return ExecuteTemplateWithVisited(templateString, fm, site, query, make(map[string]bool))
}

// ExecuteTemplateWithVisited executes a template string with the given frontmatter and site context,
// using a shared visited map to prevent circular references across all template functions.
func ExecuteTemplateWithVisited(templateString string, fm wikipage.FrontMatter, site wikipage.PageReader, query frontmatter.IQueryFrontmatterIndex, visited map[string]bool) ([]byte, error) {
	// Set a reasonable timeout for template execution to prevent hangs
	timeout := templateExecutionTimeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	resultChan := make(chan []byte, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("template execution panic: %v", r)
			}
		}()

		templateContext, err := ConstructTemplateContextFromFrontmatterWithVisited(fm, query, visited)
		if err != nil {
			errorChan <- err
			return
		}

		// Pass the shared visited map to all template functions to prevent circular references
		funcs := template.FuncMap{
			"ShowInventoryContentsOf": BuildShowInventoryContentsOfWithVisited(site, query, 0, visited),
			"LinkTo":                  BuildLinkToWithVisited(site, templateContext, query, visited),
			"IsContainer":             BuildIsContainer(query),
			"FindBy":                  query.QueryExactMatch,
			"FindByPrefix":            query.QueryPrefixMatch,
			"FindByKeyExistence":      query.QueryKeyExistence,
		}

		tmpl, err := template.New("page").Funcs(funcs).Parse(templateString)
		if err != nil {
			errorChan <- err
			return
		}

		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, templateContext)
		if err != nil {
			errorChan <- err
			return
		}

		resultChan <- buf.Bytes()
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("template execution timed out after %v - possible circular reference or infinite loop", timeout)
	}
}
