package utils

import (
	"encoding/base32"
	"encoding/json"
	"errors"
	"math/rand"
	"mime"
	"os"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/adrg/frontmatter"
	"github.com/brendanjerwin/simple_wiki/common"
	frontmatterIdx "github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/static"
	"github.com/brendanjerwin/simple_wiki/templating"
)

var (
	animals    []string
	adjectives []string
)

// IRenderMarkdownToHtml is an interface that abstracts the rendering process
type IRenderMarkdownToHtml interface {
	Render(input []byte) ([]byte, error)
}

func init() {
	animalsText, _ := static.StaticContent.ReadFile("text/animals")
	animals = strings.Split(string(animalsText), ",")
	adjectivesText, _ := static.StaticContent.ReadFile("text/adjectives")
	adjectives = strings.Split(string(adjectivesText), "\n")
}

func randomAnimal() string {
	return strings.ReplaceAll(cases.Title(language.English).String(animals[rand.Intn(len(animals)-1)]), " ", "")
}

func randomAdjective() string {
	return strings.ReplaceAll(cases.Title(language.English).String(adjectives[rand.Intn(len(adjectives)-1)]), " ", "")
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
	_ = mime.AddExtensionType(".md", "text/markdown")
	_ = mime.AddExtensionType(".heic", "image/heic")
	_ = mime.AddExtensionType(".heif", "image/heif")

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

func MarkdownToHtmlAndJsonFrontmatter(s string, handleFrontMatter bool, site common.PageReader, renderer IRenderMarkdownToHtml, query frontmatterIdx.IQueryFrontmatterIndex) ([]byte, []byte, error) {
	var markdownBytes []byte
	var matterBytes []byte
	var err error

	matter := &map[string]any{}
	if handleFrontMatter {
		markdownBytes, err = frontmatter.Parse(strings.NewReader(s), &matter)
		if err != nil {
			return []byte(err.Error()), nil, err
		}
		matterBytes, _ = json.Marshal(matter)

		markdownBytes, err = templating.ExecuteTemplate(string(markdownBytes), *matter, site, query)
		if err != nil {
			return []byte(err.Error()), nil, err
		}
	} else {
		markdownBytes = []byte(s)
	}

	html, err := renderer.Render(markdownBytes)
	if err != nil {
		return nil, nil, err
	}

	return html, matterBytes, nil
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

// ReverseSliceInt reverses a slice of ints.
func ReverseSliceInt(s []int) []int {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
