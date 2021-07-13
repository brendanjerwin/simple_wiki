package server

import (
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"math/rand"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/shurcooL/github_flavored_markdown"
	"golang.org/x/crypto/bcrypt"
)

var animals []string
var adjectives []string
var aboutPageText string
var allowInsecureHtml bool

func init() {
	rand.Seed(time.Now().Unix())
	animalsText, _ := StaticContent.ReadFile("static/text/animals")
	animals = strings.Split(string(animalsText), ",")
	adjectivesText, _ := StaticContent.ReadFile("static/text/adjectives")
	adjectives = strings.Split(string(adjectivesText), "\n")
}

func randomAnimal() string {
	return strings.Replace(strings.Title(animals[rand.Intn(len(animals)-1)]), " ", "", -1)
}

func randomAdjective() string {
	return strings.Replace(strings.Title(adjectives[rand.Intn(len(adjectives)-1)]), " ", "", -1)
}

func randomAlliterateCombo() (combo string) {
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
func stringInSlice(s string, strings []string) bool {
	for _, k := range strings {
		if s == k {
			return true
		}
	}
	return false
}

func contentType(filename string) string {
	nameParts := strings.Split(filename, ".")
	mime.AddExtensionType(".md", "text/markdown")
	mimeType := mime.TypeByExtension(nameParts[len(nameParts)-1])
	return mimeType
}

func (s *Site) sniffContentType(name string) (string, error) {
	file, err := os.Open(path.Join(s.PathToData, name))
	if err != nil {
		return "", err

	}
	defer file.Close()

	// Only the first 512 bytes are used to sniff the content type.
	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	// Always returns a valid content-type and "application/octet-stream" if no others seemed to match.
	return http.DetectContentType(buffer), nil
}

var src = rand.NewSource(time.Now().UnixNano())

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandStringBytesMaskImprSrc prints a random string
func RandStringBytesMaskImprSrc(n int) string {
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
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

	return string(b)
}

// HashPassword generates a bcrypt hash of the password using work factor 14.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
func HashPassword(password string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 14)
	return hex.EncodeToString(hash)
}

// CheckPassword securely compares a bcrypt hashed password with its possible
// plaintext equivalent.  Returns nil on success, or an error on failure.
// https://github.com/gtank/cryptopasta/blob/master/hash.go
func CheckPasswordHash(password, hashedString string) error {
	hash, err := hex.DecodeString(hashedString)
	if err != nil {
		return err
	}
	return bcrypt.CompareHashAndPassword(hash, []byte(password))
}

// exists returns whether the given file or directory exists or not
func exists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

type DoesntMatter struct{}

func StripFrontmatter(s string) string {
	doesnt_matter := &DoesntMatter{}
	unsafe, _ := frontmatter.Parse(strings.NewReader(s), &doesnt_matter)
	return string(unsafe)
}

func MarkdownToHtmlAndJsonFrontmatter(s string, handleFrontMatter bool) ([]byte, []byte) {
	var unsafe []byte
	var err error
	matter := &map[string]interface{}{}
	if handleFrontMatter {
		unsafe, err = frontmatter.Parse(strings.NewReader(s), &matter)
		if err != nil {
			panic(err)
		}
	} else {
		unsafe = []byte(s)
	}
	matterBytes, _ := json.Marshal(matter)
	unsafe = blackfriday.Run(unsafe)
	if allowInsecureHtml {
		return unsafe, matterBytes
	}

	pClean := bluemonday.UGCPolicy()
	pClean.AllowElements("img")
	pClean.AllowElements("center")
	pClean.AllowAttrs("alt").OnElements("img")
	pClean.AllowAttrs("src").OnElements("img")
	pClean.AllowAttrs("class").OnElements("a")
	pClean.AllowAttrs("href").OnElements("a")
	pClean.AllowAttrs("id").OnElements("a")
	pClean.AllowDataURIImages()
	html := pClean.SanitizeBytes(unsafe)
	return html, matterBytes
}

func GithubMarkdownToHTML(s string) []byte {
	return github_flavored_markdown.Markdown([]byte(s))
}

func encodeToBase32(s string) string {
	return encodeBytesToBase32([]byte(s))
}

func encodeBytesToBase32(s []byte) string {
	return base32.StdEncoding.EncodeToString(s)
}

func decodeFromBase32(s string) (s2 string, err error) {
	bString, err := base32.StdEncoding.DecodeString(s)
	s2 = string(bString)
	return
}

func reverseSliceInt64(s []int64) []int64 {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func reverseSliceString(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

func reverseSliceInt(s []int) []int {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
