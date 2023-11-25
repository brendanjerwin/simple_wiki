package utils

import (
	"bytes"
	"encoding/base32"
	"encoding/json"
	"errors"
	"math/rand"
	"mime"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/brendanjerwin/simple_wiki/static"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/shurcooL/github_flavored_markdown"
)

var animals []string
var adjectives []string
var AllowInsecureHtml bool

type IReadFrontMatter interface {
	ReadFrontMatter(identifier string) (map[string]interface{}, error)
}

func init() {
	animalsText, _ := static.StaticContent.ReadFile("text/animals")
	animals = strings.Split(string(animalsText), ",")
	adjectivesText, _ := static.StaticContent.ReadFile("text/adjectives")
	adjectives = strings.Split(string(adjectivesText), "\n")
}

func randomAnimal() string {
	return strings.Replace(strings.Title(animals[rand.Intn(len(animals)-1)]), " ", "", -1)
}

func randomAdjective() string {
	return strings.Replace(strings.Title(adjectives[rand.Intn(len(adjectives)-1)]), " ", "", -1)
}

func RandomAlliterateCombo() (combo string) {
	combo = ""
	// generate random alliteration thats not been used
	for {
		animal := randomAnimal()
		adjective := randomAdjective()
		if animal[0] == adjective[0] && len(animal)+len(adjective) < 18 { //&& stringInSlice(strings.ToLower(adjective+animal), takenNames) == false {
			combo = adjective + animal
			break
		}
	}
	return
}

// is there a string in a slice?
func StringInSlice(s string, strings []string) bool {
	for _, k := range strings {
		if s == k {
			return true
		}
	}
	return false
}

func ContentTypeFromName(filename string) string {
	mime.AddExtensionType(".md", "text/markdown")
	mime.AddExtensionType(".heic", "image/heic")
	mime.AddExtensionType(".heif", "image/heif")

	nameParts := strings.Split(filename, ".")
	mimeType := mime.TypeByExtension("." + nameParts[len(nameParts)-1])
	return mimeType
}

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandomStringOfLength prints a random string
func RandomStringOfLength(l int) (string, error) {
	if l <= 0 {
		return "", errors.New("length must be greater than 0")
	}
	b := make([]byte, l)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := l-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b), nil
}

// Exists returns whether the given file or directory Exists or not
func Exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type DoesntMatter struct{}

func StripFrontmatter(s string) string {
	doesnt_matter := &DoesntMatter{}
	unsafe, _ := frontmatter.Parse(strings.NewReader(s), &doesnt_matter)
	return string(unsafe)
}

func MarkdownToHtmlAndJsonFrontmatter(s string, handleFrontMatter bool, site IReadFrontMatter) ([]byte, []byte) {
	var unsafe []byte
	var err error
	var matterBytes []byte

	matter := &map[string]interface{}{}
	if handleFrontMatter {
		unsafe, err = frontmatter.Parse(strings.NewReader(s), &matter)
		if err != nil {
			panic(err)
		}
		matterBytes, _ = json.Marshal(matter)

		unsafe, err = ExecuteTemplate(string(unsafe), matterBytes, site)
		if err != nil {
			return []byte(err.Error()), nil
		}
	} else {
		unsafe = []byte(s)
	}

	r := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{
		Flags: blackfriday.CommonHTMLFlags, //& blackfriday.Smartypants,
	})
	unsafe = blackfriday.Run(unsafe, blackfriday.WithRenderer(r))
	if AllowInsecureHtml {
		return unsafe, matterBytes
	}

	sanitizerPolicy := bluemonday.UGCPolicy()
	sanitizerPolicy.AllowElements("img")
	sanitizerPolicy.AllowElements("center")
	sanitizerPolicy.AllowAttrs("alt").OnElements("img")
	sanitizerPolicy.AllowAttrs("src").OnElements("img")
	sanitizerPolicy.AllowAttrs("class").OnElements("a")
	sanitizerPolicy.AllowAttrs("href").OnElements("a")
	sanitizerPolicy.AllowAttrs("id").OnElements("a")
	sanitizerPolicy.AllowDataURIImages()
	html := sanitizerPolicy.SanitizeBytes(unsafe)
	return html, matterBytes
}

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

func ConstructTemplateContextFromFrontmatter(frontmatter []byte) (*TemplateContext, error) {
	context := &TemplateContext{}
	err := json.Unmarshal(frontmatter, &context)
	if err != nil {
		return nil, err
	}

	unstructured := make(map[string]interface{})
	err = json.Unmarshal(frontmatter, &unstructured)
	if err != nil {
		return nil, err
	}

	context.Map = unstructured

	return context, nil
}

func BuildShowInventoryContentsOf(site IReadFrontMatter) func(string) string {
	linkTo := BuildLinkTo(site)
	isContainer := BuildIsContainer(site)
	var showInventoryContentsOf (func(string) string)
	showInventoryContentsOf = func(containerIdentifier string) string {
		frontmatter, err := site.ReadFrontMatter(containerIdentifier)
		if err != nil {
			return `
	Not Setup for Inventory
			`
		}

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
		err = tmpl.Execute(buf, frontmatter)
		if err != nil {
			return err.Error()
		}

		return buf.String()
	}

	return showInventoryContentsOf
}

func BuildLinkTo(site IReadFrontMatter) func(string) string {
	return func(identifier string) string {
		if identifier == "" {
			return "N/A"
		}
		frontmatter, err := site.ReadFrontMatter(identifier)
		if err != nil {
			return "[" + identifier + "](/" + identifier + ")"
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

func BuildIsContainer(site IReadFrontMatter) func(string) bool {
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
func ExecuteTemplate(templateHtml string, frontmatter []byte, site IReadFrontMatter) ([]byte, error) {
	funcs := template.FuncMap{
		"ShowInventoryContentsOf": BuildShowInventoryContentsOf(site),
		"LinkTo":                  BuildLinkTo(site),
		"IsContainer":             BuildIsContainer(site),
	}

	tmpl, err := template.New("page").Funcs(funcs).Parse(templateHtml)
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

func MarkdownToHTML(s string) []byte {
	return github_flavored_markdown.Markdown([]byte(s))
}

func EncodeToBase32(s string) string {
	return EncodeBytesToBase32([]byte(s))
}

func EncodeBytesToBase32(s []byte) string {
	return base32.StdEncoding.EncodeToString(s)
}

func DecodeFromBase32(s string) (s2 string, err error) {
	bString, err := base32.StdEncoding.DecodeString(s)
	s2 = string(bString)
	return
}

func ReverseSliceInt64(s []int64) []int64 {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func ReverseSliceString(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func ReverseSliceInt(s []int) []int {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
