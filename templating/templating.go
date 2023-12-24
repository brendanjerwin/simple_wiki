package templating

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"text/template"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/stoewer/go-strcase"
)

type InventoryFrontmatter struct {
	Container string   `json:"container"`
	Items     []string `json:"items"`
}

type TemplateContext struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Map        map[string]interface{}
	Inventory  *InventoryFrontmatter `json:"inventory"`
}

func ConstructTemplateContextFromFrontmatter(frontmatter common.FrontMatter) (*TemplateContext, error) {
	bytes, err := json.Marshal(frontmatter)
	if err != nil {
		return nil, err
	}

	context := &TemplateContext{}
	err = json.Unmarshal(bytes, &context)
	if err != nil {
		return nil, err
	}

	context.Map = frontmatter
	if context.Inventory == nil {
		context.Inventory = &InventoryFrontmatter{}
	}

	return context, nil
}

func BuildShowInventoryContentsOf(site common.IReadPages, query index.IQueryFrontmatterIndex, indent int) func(string) string {
	isContainer := BuildIsContainer(query)

	return func(containerIdentifier string) string {
		containerFrontmatter, err := site.ReadFrontMatter(containerIdentifier)
		if err != nil {
			return err.Error()
		}
		containerTemplateContext, err := ConstructTemplateContextFromFrontmatter(containerFrontmatter)
		if err != nil {
			return err.Error()
		}

		if containerTemplateContext.Inventory == nil {
			containerTemplateContext.Inventory = &InventoryFrontmatter{}
		}
		if containerTemplateContext.Inventory.Items == nil {
			containerTemplateContext.Inventory.Items = []string{}
		}

		itemsFromIndex := query.QueryExactMatch("inventory.container", containerIdentifier)

		// Create a map to store unique items
		uniqueItems := make(map[string]bool)

		// Add existing items to the map
		for _, item := range containerTemplateContext.Inventory.Items {
			uniqueItems[item] = true
		}

		// Add new items to the map
		for _, item := range itemsFromIndex {
			uniqueItems[item] = true

			// If there was an item that existed as a title in the list of items, remove it.
			// This is to support the workflow of items first being listed directly on the inventory container,
			// but later getting their own page and being linked to the inventory container through the inventory.container frontmatter key.
			item_title := query.GetValue(item, "title")
			delete(uniqueItems, item_title)
		}

		// Convert the map back to a slice
		containerTemplateContext.Inventory.Items = make([]string, 0, len(uniqueItems))
		for item := range uniqueItems {
			containerTemplateContext.Inventory.Items = append(containerTemplateContext.Inventory.Items, item)
		}

		tmplString := `
{{ range .Inventory.Items }}
{{ Indent }} - {{ LinkTo . }}
{{ if IsContainer . }}
{{ ShowInventoryContentsOf . }}
{{ end }}
{{ end }}
`
		funcs := template.FuncMap{
			"LinkTo":                  BuildLinkTo(site, containerTemplateContext),
			"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site, query, indent+1),
			"IsContainer":             isContainer,
			"FindBy":                  query.QueryExactMatch,
			"FindByPrefix":            query.QueryPrefixMatch,
			"FindByKeyExistence":      query.QueryKeyExistence,
			"Indent":                  func() string { return strings.Repeat(" ", indent*2) },
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

func BuildLinkTo(site common.IReadPages, currentPageTemplateContext *TemplateContext) func(string) string {
	return func(identifier string) string {
		if identifier == "" {
			return "N/A"
		}

		var frontmatter, err = site.ReadFrontMatter(identifier)
		if err != nil {
			//Try again with a snake case identifier
			snake_identifier := strcase.SnakeCase(identifier)
			url_encoded_identifier := url.QueryEscape(identifier)
			frontmatter, err = site.ReadFrontMatter(snake_identifier)
			if err != nil {
				//Doesnt look like it exists yet, return a link.
				//It'll render and let the page get created.
				if _, ok := currentPageTemplateContext.Map["inventory"]; ok {
					//special inventory item link with attributes
					return "[" + identifier + "](/" + snake_identifier + "?tmpl=inv_item&inventory.container=" + currentPageTemplateContext.Identifier + "&title=" + url_encoded_identifier + ")"
				}

				return "[" + identifier + "](/" + snake_identifier + "?title=" + url_encoded_identifier + ")"
			}
		}

		tmplString := "{{if index . \"title\"}}[{{ index . \"title\" }}](/{{ index . \"identifier\" }}){{else}}[{{ index . \"identifier\" }}](/{{ index . \"identifier\" }}){{end}}"
		tmpl, err := template.New("content").Parse(tmplString)
		if err != nil {
			return err.Error()
		}

		buf := &bytes.Buffer{}
		err = tmpl.Execute(buf, frontmatter)
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
	templateContext, err := ConstructTemplateContextFromFrontmatter(frontmatter)
	if err != nil {
		return nil, err
	}

	funcs := template.FuncMap{
		"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site, query, 0),
		"LinkTo":                  BuildLinkTo(site, templateContext),
		"IsContainer":             BuildIsContainer(query),
		"FindBy":                  query.QueryExactMatch,
		"FindByPrefix":            query.QueryPrefixMatch,
		"FindByKeyExistence":      query.QueryKeyExistence,
	}

	tmpl, err := template.New("page").Funcs(funcs).Parse(templateString)
	if err != nil {
		return nil, err
	}

	context, err := ConstructTemplateContextFromFrontmatter(frontmatter)
	if err != nil {
		return nil, err
	}

	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, context)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
