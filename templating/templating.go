package templating

import (
	"bytes"
	"encoding/json"
	"net/url"
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

	return context, nil
}

func BuildShowInventoryContentsOf(site common.IReadPages, currentPageFrontMatter *TemplateContext) func(string) string {
	isContainer := BuildIsContainer(site)
	var showInventoryContentsOf (func(string) string)
	showInventoryContentsOf = func(containerIdentifier string) string {
		containerFrontmatter, err := site.ReadFrontMatter(containerIdentifier)
		if err != nil {
			return `
	Not Setup for Inventory
			`
		}
		containerTemplateContext, err := ConstructTemplateContextFromFrontmatter(containerFrontmatter)
		if err != nil {
			return err.Error()
		}
		linkTo := BuildLinkTo(site, containerTemplateContext)
		tmplString := `{{if index . "inventory"}}
{{if index . "inventory" "items"}}
{{ range index . "inventory" "items" }}
{{if IsContainer .}}

**{{LinkTo .}}**

{{ShowInventoryContentsOf . }}
{{else}}
  - {{LinkTo . }}
{{end}}
{{end}}
{{else}}
	No Items
{{end}}
{{else}}
	Not Setup for Inventory
{{end}}
`
		funcs := template.FuncMap{
			"LinkTo":                  linkTo,
			"ShowInventoryContentsOf": showInventoryContentsOf,
			"IsContainer":             isContainer,
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

	return showInventoryContentsOf
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

func BuildIsContainer(site common.IReadPages) func(string) bool {
	return func(identifier string) bool {
		if identifier == "" {
			return false
		}
		frontmatter, err := site.ReadFrontMatter(identifier)
		if err != nil {
			return false
		}

		if inventory, exist := frontmatter["inventory"]; exist {
			switch inv := inventory.(type) {
			case map[string]interface{}:
				if _, exist := inv["items"]; exist {
					return true
				}
			}
		}

		return false

	}
}

func LabelUrlForIdentifier(identifier string) string {
	return "http://wiki.local/" + identifier
}

func ExecuteTemplate(templateString string, frontmatter common.FrontMatter, site common.IReadPages, query index.IQueryFrontmatterIndex) ([]byte, error) {
	templateContext, err := ConstructTemplateContextFromFrontmatter(frontmatter)
	if err != nil {
		return nil, err
	}
	funcs := template.FuncMap{
		"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site, templateContext),
		"LinkTo":                  BuildLinkTo(site, templateContext),
		"IsContainer":             BuildIsContainer(site),
		"FindBy":                  query.QueryExactMatch,
		"FindByPrefix":            query.QueryPrefixMatch,
		"FindByKeyExistence":      query.QueryKeyExistence,
		"LabelUrlFor":             LabelUrlForIdentifier,
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
