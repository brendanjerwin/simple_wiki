package server

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/brendanjerwin/simple_wiki/common"
	"github.com/brendanjerwin/simple_wiki/index"
	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/index/frontmatter"
	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/gabriel-vasile/mimetype"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/jcelliott/lumber"
	"gopkg.in/yaml.v2"
)

type Site struct {
	PathToData              string
	CSS                     []byte
	DefaultPage             string
	DefaultPassword         string
	Debounce                int
	SessionStore            cookie.Store
	SecretCode              string
	Fileuploads             bool
	MaxUploadSize           uint
	MaxDocumentSize         uint // in runes; about a 10mb limit by default
	Logger                  *lumber.ConsoleLogger
	MarkdownRenderer        utils.IRenderMarkdownToHtml
	IndexMaintainer         index.IMaintainIndex
	FrontmatterIndexQueryer frontmatter.IQueryFrontmatterIndex
	BleveIndexQueryer       bleve.IQueryBleveIndex
	saveMut                 sync.Mutex
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

	// The mimetype library reads up to 3072 bytes by default.
	buffer := make([]byte, 3072)
	n, err := file.Read(buffer)
	if err != nil && err != io.EOF {
		return "", err
	}

	// Use mimetype library to detect content type. It is more accurate and
	// can detect charsets.
	mtype := mimetype.Detect(buffer[:n])
	return mtype.String(), nil
}

func (s *Site) InitializeIndexing() error {
	frontmatterIndex := frontmatter.NewFrontmatterIndex(s)
	bleveIndex, err := bleve.NewBleveIndex(s, frontmatterIndex)
	if err != nil {
		return err
	}
	multiMaintainer := index.NewMultiMaintainer(frontmatterIndex, bleveIndex)

	s.FrontmatterIndexQueryer = frontmatterIndex
	s.BleveIndexQueryer = bleveIndex
	s.IndexMaintainer = multiMaintainer

	files := s.DirectoryList()
	for _, file := range files {
		s.IndexMaintainer.AddPageToIndex(file.Name())
	}

	s.Logger.Info("Indexing complete. Added %v pages.", len(files))

	return nil
}

// --- PageReadWriter implementation ---

type pageData struct {
	FrontMatter common.FrontMatter
	Markdown    common.Markdown
}

func (s *Site) getFullPath(identifier common.PageIdentifier) string {
	name := string(identifier)
	if !strings.HasSuffix(name, ".md") {
		name += ".md"
	}
	return path.Join(s.PathToData, name)
}

func (s *Site) readPage(identifier common.PageIdentifier) (*pageData, error) {
	filepath := s.getFullPath(identifier)
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	// check for frontmatter
	parts := bytes.SplitN(data, []byte("---\n"), 3)
	if len(parts) < 3 {
		return &pageData{
			FrontMatter: make(common.FrontMatter),
			Markdown:    common.Markdown(data),
		}, nil
	}

	var fm common.FrontMatter
	if err = yaml.Unmarshal(parts[1], &fm); err != nil {
		return nil, fmt.Errorf("failed to unmarshal frontmatter for %s: %v", identifier, err)
	}

	return &pageData{
		FrontMatter: fm,
		Markdown:    common.Markdown(parts[2]),
	}, nil
}

func (s *Site) writePage(identifier common.PageIdentifier, p *pageData) error {
	s.saveMut.Lock()
	defer s.saveMut.Unlock()

	fmBytes, err := yaml.Marshal(p.FrontMatter)
	if err != nil {
		return fmt.Errorf("failed to marshal frontmatter for %s: %v", identifier, err)
	}

	var content bytes.Buffer
	content.WriteString("---\n")
	content.Write(fmBytes)
	content.WriteString("---\n")
	content.WriteString(string(p.Markdown))

	filepath := s.getFullPath(identifier)
	if err := os.WriteFile(filepath, content.Bytes(), 0644); err != nil {
		return err
	}

	if s.IndexMaintainer != nil {
		s.IndexMaintainer.AddPageToIndex(identifier)
	}
	return nil
}

func (s *Site) WriteFrontMatter(identifier common.PageIdentifier, fm common.FrontMatter) error {
	page, err := s.readPage(identifier)
	if err != nil {
		if os.IsNotExist(err) {
			page = &pageData{Markdown: ""} // Create a new page if it doesn't exist
		} else {
			return err
		}
	}

	page.FrontMatter = fm
	return s.writePage(identifier, page)
}

func (s *Site) WriteMarkdown(identifier common.PageIdentifier, md common.Markdown) error {
	page, err := s.readPage(identifier)
	if err != nil {
		if os.IsNotExist(err) {
			page = &pageData{FrontMatter: make(common.FrontMatter)} // Create a new page if it doesn't exist
		} else {
			return err
		}
	}

	page.Markdown = md
	return s.writePage(identifier, page)
}

func (s *Site) ReadFrontMatter(identifier common.PageIdentifier) (common.PageIdentifier, common.FrontMatter, error) {
	page, err := s.readPage(identifier)
	if err != nil {
		return "", nil, err
	}
	return identifier, page.FrontMatter, nil
}

func (s *Site) ReadMarkdown(identifier common.PageIdentifier) (common.PageIdentifier, common.Markdown, error) {
	page, err := s.readPage(identifier)
	if err != nil {
		return "", "", err
	}
	return identifier, page.Markdown, nil
}
