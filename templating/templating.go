package templating

import (
	"bytes"
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"text/template"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index"
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
	Map        map[string]interface{}
	Inventory  InventoryFrontmatter `json:"inventory"`
}

func ConstructTemplateContextFromFrontmatter(frontmatter common.FrontMatter, query index.IQueryFrontmatterIndex) (TemplateContext, error) {
	bytes, err := json.Marshal(frontmatter)
	if err != nil {
		return TemplateContext{}, err
	}

	context := TemplateContext{}
	err = json.Unmarshal(bytes, &context)
	if err != nil {
		return TemplateContext{}, err
	}

	context.Map = frontmatter

	if context.Inventory.Items == nil {
		context.Inventory.Items = []string{}
	}

	// Create a map to store unique items
	uniqueItems := make(map[string]bool)

	// Add existing items to the map
	for _, item := range context.Inventory.Items {
		uniqueItems[common.MungeIdentifier(item)] = true
	}

	// Add new items to the map
	itemsFromIndex := query.QueryExactMatch("inventory.container", context.Identifier)
	for _, item := range itemsFromIndex {
		// If there was an item that existed as a title in the list of items, remove it.
		// This is to support the workflow of items first being listed directly on the inventory container,
		// but later getting their own page and being linked to the inventory container through the inventory.container frontmatter key.
		item_title := query.GetValue(item, "title")
		delete(uniqueItems, item_title)

		uniqueItems[item] = true
	}

	// Convert the map back to a slice
	context.Inventory.Items = make([]string, 0, len(uniqueItems))
	for item := range uniqueItems {
		context.Inventory.Items = append(context.Inventory.Items, item)
	}

	sort.Strings(context.Inventory.Items)

	return context, nil
}

func BuildShowInventoryContentsOf(site common.IReadPages, query index.IQueryFrontmatterIndex, indent int) func(string) string {
	isContainer := BuildIsContainer(query)

	return func(containerIdentifier string) string {
		containerIdentifier, containerFrontmatter, err := site.ReadFrontMatter(containerIdentifier)
		if err != nil {
			return err.Error()
		}
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
		funcs := template.FuncMap{
			"LinkTo":                  BuildLinkTo(site, containerTemplateContext, query),
			"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site, query, indent+1),
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

func BuildLinkTo(site common.IReadPages, currentPageTemplateContext TemplateContext, query index.IQueryFrontmatterIndex) func(string) string {
	isContainer := BuildIsContainer(query)
	return func(identifierToLink string) string {
		if identifierToLink == "" {
			return "N/A"
		}

		identifierToLink, frontmatterForLinkedPage, err := site.ReadFrontMatter(identifierToLink)
		if err != nil {
			titleCaser := cases.Title(language.AmericanEnglish)
			titleCasedTitle := titleCaser.String(strings.ReplaceAll(strcase.SnakeCase(identifierToLink), "_", " "))
			urlEncodedTitle := url.QueryEscape(titleCasedTitle)
			//Doesnt look like it exists yet, return a link.
			//It'll render and let the page get created.
			if isContainer(currentPageTemplateContext.Identifier) {
				//special inventory item link with attributes
				return "[" + titleCasedTitle + "](/" + identifierToLink + "?tmpl=inv_item&inventory.container=" + currentPageTemplateContext.Identifier + "&title=" + urlEncodedTitle + ")"
			}

			return "[" + titleCasedTitle + "](/" + identifierToLink + "?title=" + urlEncodedTitle + ")"
		}

		//Linked Page Exists
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

func BuildIsContainer(query index.IQueryFrontmatterIndex) func(string) bool {
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

func ExecuteTemplate(templateString string, frontmatter common.FrontMatter, site common.IReadPages, query index.IQueryFrontmatterIndex) ([]byte, error) {
	templateContext, err := ConstructTemplateContextFromFrontmatter(frontmatter, query)
	if err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site, query, 0),
		"LinkTo":                  BuildLinkTo(site, templateContext, query),
		"IsContainer":             BuildIsContainer(query),
		"FindBy":                  query.QueryExactMatch,
		"FindByPrefix":            query.QueryPrefixMatch,
		"FindByKeyExistence":      query.QueryKeyExistence,
	}

	tmpl, err := template.New("page").Funcs(funcs).Parse(templateString)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, templateContext)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
