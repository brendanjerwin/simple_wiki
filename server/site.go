package server

import (
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jcelliott/lumber"
)

type Site struct {
	PathToData       string
	Css              []byte
	DefaultPage      string
	DefaultPassword  string
	Debounce         int
	SessionStore     cookie.Store
	SecretCode       string
	AllowInsecure    bool
	Fileuploads      bool
	MaxUploadSize    uint
	Logger           *lumber.ConsoleLogger
	MarkdownRenderer utils.IRenderMarkdownToHtml
	MaxDocumentSize  uint // in runes; about a 10mb limit by default
	saveMut          sync.Mutex
}

func (s *Site) defaultLock() string {
	if s.DefaultPassword == "" {
		return ""
	}
	return sec.HashPassword(s.DefaultPassword)
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
