package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/static"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

const (
	// HTTP status codes
	httpStatusFound = 302
	httpStatusOK    = 200

	// Magic numbers
	maxSecretAttempts = 8
	maxContentLength  = 3

	// String constants
	rootPath             = "/"
	uploadFailureMessage = "Failed to upload: %s"
	uploadsPage          = "uploads"
)

var (
	hotTemplateReloading bool
	LogLevel             int = lumber.WARN
)

// GinRouter returns a new Gin router configured for the site.
// Optional middleware handlers are added before routes are registered.
func (s *Site) GinRouter(middleware ...gin.HandlerFunc) *gin.Engine {
	if s.Logger == nil {
		s.Logger = lumber.NewConsoleLogger(lumber.TRACE)
	}

	if hotTemplateReloading {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.SetFuncMap(template.FuncMap{
		"sniffContentType": s.sniffContentType,
	})

	if hotTemplateReloading {
		router.LoadHTMLGlob("server/templates/*.tmpl")
	} else {
		router.HTMLRender = s.loadTemplate()
	}

	// Add provided middleware before routes (important for observability)
	for _, mw := range middleware {
		router.Use(mw)
	}

	router.Use(sessions.Sessions("_session", s.SessionStore))

	s.registerRoutes(router)
	return router
}

func (s *Site) registerRoutes(router *gin.Engine) {
	router.GET("/", func(c *gin.Context) {
		c.Redirect(httpStatusFound, "/"+s.DefaultPage+"/view")
	})

	router.POST("/uploads", s.handleUpload)
	router.GET("/cli/:binary", serveCLIBinary)
	router.GET("/extensions/:file", serveExtensionFile)

	router.GET("/:page", func(c *gin.Context) {
		page := sanitizePageName(c.Param("page"))
		c.Redirect(httpStatusFound, "/"+page+"/view?"+c.Request.URL.RawQuery)
	})
	router.GET("/:page/*command", s.handlePageRequest)
	router.POST("/update", s.handlePageUpdate)
	router.POST("/api/print_label", s.handlePrintLabel)
	router.GET("/api/find_by", s.handleFindBy)
	router.GET("/api/find_by_prefix", s.handleFindByPrefix)
	router.GET("/api/find_by_key_existence", s.handleFindByKeyExistence)
}

// serveCLIBinary serves pre-built wiki-cli binaries so consumers always
// download the version that matches the running wiki. Binaries are embedded
// in static.StaticContent at build time (see cmd/wiki-cli/generate.go).
func serveCLIBinary(c *gin.Context) {
	// path.Base prevents directory traversal; the pattern check guards against
	// requests for arbitrary files in the embedded FS.
	binary := path.Base(c.Param("binary"))
	if !strings.HasPrefix(binary, "wiki-cli-") {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := static.StaticContent.ReadFile("cli/" + binary)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, binary))
	c.Data(http.StatusOK, "application/octet-stream", data)
}

// serveExtensionFile serves browser extension files (XPI packages and update
// manifests). Built by extensions/online-order-recorder/generate.go and
// embedded in static.StaticContent at build time.
func serveExtensionFile(c *gin.Context) {
	file := path.Base(c.Param("file"))

	switch file {
	case "simple-wiki-companion.xpi":
		data, err := static.StaticContent.ReadFile("extensions/" + file)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file))
		c.Data(http.StatusOK, "application/x-xpinstall", data)

	case "updates.json":
		versionBytes, err := static.StaticContent.ReadFile("extensions/version.txt")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		version := strings.TrimSpace(string(versionBytes))
		baseURL := requestBaseURL(c)

		updatesJSON := fmt.Sprintf(`{
  "addons": {
    "simple-wiki-companion@simple-wiki": {
      "updates": [
        {
          "version": %q,
          "update_link": "%s/extensions/simple-wiki-companion.xpi"
        }
      ]
    }
  }
}`, version, baseURL)

		c.Data(http.StatusOK, "application/json", []byte(updatesJSON))

	default:
		c.Status(http.StatusNotFound)
	}
}

func (s *Site) loadTemplate() multitemplate.Render {
	r := multitemplate.New()

	tmplMessage, err := template.New("index.tmpl").Funcs(template.FuncMap{
		"sniffContentType": s.sniffContentType,
	}).Parse(string(static.IndexTemplate))
	if err != nil {
		panic(err)
	}

	r.Add("index.tmpl", tmplMessage)

	return r
}

func getSetSessionID(c *gin.Context, logger *lumber.ConsoleLogger) (sid string) {
	var (
		session = sessions.Default(c)
		v       = session.Get("sid")
	)
	if v != nil {
		if sidStr, ok := v.(string); ok {
			sid = sidStr
		}
	}
	if v == nil || sid == "" {
		// Each byte becomes two hex characters, so we need half the number of bytes as the desired string length.
		// We add 1 and divide by 2 to handle both even and odd lengths correctly.
		byteLength := (maxSecretAttempts + 1) / 2
		b := make([]byte, byteLength)
		if _, err := rand.Read(b); err != nil {
			panic(fmt.Sprintf("failed to generate random string: %v", err))
		}
		sid = hex.EncodeToString(b)
		// Trim the string to the exact desired length, as we may have generated one extra character for odd lengths.
		if len(sid) > maxSecretAttempts {
			sid = sid[:maxSecretAttempts]
		}
		session.Set("sid", sid)
		if err := session.Save(); err != nil {
			logger.Error("Failed to save session: %v", err)
		}
	}
	return sid
}

func (s *Site) handlePageRequest(c *gin.Context) {
	page := sanitizePageName(c.Param("page"))
	command := c.Param("command")

	// Handle special pages (favicon, static, uploads)
	if s.handleSpecialPages(c, page, command) {
		return
	}

	if len(command) < 2 {
		c.Redirect(httpStatusFound, "/"+page+"/view")
		return
	}

	p, err := s.readOrInitPage(page, c.Request)
	if err != nil {
		s.Logger.Error("Failed to initialize page '%s': %v", page, err)
		c.String(http.StatusInternalServerError, "Failed to initialize page")
		return
	}

	// Render the page content
	s.renderPageContent(c, page, command, p)
}

// handleSpecialPages handles special page routes (favicon, static, uploads)
// Returns true if the request was handled
func (s *Site) handleSpecialPages(c *gin.Context, page, command string) bool {
	switch page {
	case "favicon.ico":
		s.handleFavicon(c)
		return true
	case "static":
		s.handleStatic(c, command)
		return true
	case uploadsPage:
		s.handleUploads(c, command)
		return true
	default:
		return false
	}
}

// renderPageContent handles the final page content rendering
func (s *Site) renderPageContent(c *gin.Context, page, command string, p *wikipage.Page) {
	rawText, contentHTML := s.getPageContent(p)
	contentHTML = fmt.Appendf(nil, "<article class='content' id='%s'>%s</article>", page, string(contentHTML))

	if s.handleRawContent(c, command, p, rawText) {
		return
	}

	if s.handleFrontmatter(c, command, p) {
		return
	}

	directoryEntries, command, err := s.getDirectoryEntries(page, command)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}

	templateData := s.buildTemplateData(page, command, directoryEntries, contentHTML, rawText, c)
	c.HTML(http.StatusOK, "index.tmpl", templateData)
}

func (*Site) getPageContent(p *wikipage.Page) (rawText string, contentHTML []byte) {
	rawText = p.Text
	contentHTML = p.RenderedPage
	return rawText, contentHTML
}

func (s *Site) getDirectoryEntries(page, command string) ([]os.FileInfo, string, error) {
	var directoryEntries []os.FileInfo
	if page == "ls" {
		command = "/view"
		directoryEntries = s.DirectoryList()
	}
	if page == uploadsPage {
		command = "/view"
		var err error
		directoryEntries, err = s.UploadList()
		if err != nil {
			return nil, command, err
		}
	}
	return directoryEntries, command, nil
}

func (s *Site) buildTemplateData(page, command string, directoryEntries []os.FileInfo, contentHTML []byte, rawText string, c *gin.Context) gin.H {
	return gin.H{
		"EditPage": command[0:2] == "/e", // /edit
		"ViewPage": command[0:2] == "/v", // /view
		"ReadPage": command[0:2] == "/r", // /read
		"DontKnowPage": command[0:2] != "/e" &&
			command[0:2] != "/v" &&
			command[0:2] != "/l" &&
			command[0:2] != "/r",
		"DirectoryPage":    page == "ls" || page == uploadsPage,
		"UploadPage":       page == uploadsPage,
		"DirectoryEntries": directoryEntries,
		"Page":             page,
		"RenderedPage":     template.HTML([]byte(contentHTML)),
		"RawPage":          rawText,
		"Route":            "/" + page + command,
		"HasDotInName":     strings.Contains(page, "."),
		"RecentlyEdited":   getRecentlyEdited(page, c, s.Logger),
		"CustomCSS":        len(s.CSS) > 0,
		"Debounce":         s.Debounce,
		"Date":             time.Now().Format("2006-01-02"),
		"UnixTime":         time.Now().Unix(),
		"AllowFileUploads": s.Fileuploads,
		"MaxUploadMB":      s.MaxUploadSize,
		"WikiBaseURL":       requestBaseURL(c),
	}
}

func getRecentlyEdited(title string, c *gin.Context, logger *lumber.ConsoleLogger) []string {
	session := sessions.Default(c)
	var recentlyEdited string
	v := session.Get("recentlyEdited")
	editedThings := []string{}
	if v == nil {
		recentlyEdited = title
	} else {
		if recentlyEditedStr, ok := v.(string); ok {
			editedThings = strings.Split(recentlyEditedStr, "|||")
			if !slices.Contains(editedThings, title) {
				recentlyEdited = recentlyEditedStr + "|||" + title
			} else {
				recentlyEdited = recentlyEditedStr
			}
		}
	}
	session.Set("recentlyEdited", recentlyEdited)
	if err := session.Save(); err != nil {
		logger.Error("Failed to save session: %v", err)
	}
	editedThingsWithoutCurrent := make([]string, len(editedThings))
	i := 0
	for _, thing := range editedThings {
		if thing == title {
			continue
		}
		if strings.Contains(thing, "icon-") {
			continue
		}
		editedThingsWithoutCurrent[i] = thing
		i++
	}
	return editedThingsWithoutCurrent[:i]
}

// PageUpdateRequest represents the JSON structure for page update requests
type PageUpdateRequest struct {
	Page      string `json:"page"`
	NewText   string `json:"new_text"`
	FetchedAt int64  `json:"fetched_at"`
}

func (s *Site) handlePageUpdate(c *gin.Context) {
	var json PageUpdateRequest
	err := c.BindJSON(&json)
	if err != nil {
		s.Logger.Trace("Failed to bind JSON in handlePageUpdate: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Wrong JSON"})
		return
	}
	if uint(len(json.NewText)) > s.MaxDocumentSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Too much"})
		return
	}
	if len(json.Page) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Must specify `page`"})
		return
	}
	s.Logger.Trace("Update: %v", json)
	p, err := s.ReadPage(json.Page)
	if err != nil {
		s.Logger.Error("Failed to open page %s for update: %v", json.Page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to open page"})
		return
	}
	var message string
	success := false
	if json.FetchedAt > 0 && p.IsModifiedSince(json.FetchedAt) {
		message = "Refusing to overwrite others work"
		success = false  // Explicitly set to make error handling clear
	} else {
		p.Text = json.NewText
		err := s.savePageAndIndex(p)
		if err != nil {
			message = err.Error()
			success = false  // Explicitly set to make error handling clear
		} else {
			message = "Saved"
			success = true
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": success, "message": message, "unix_time": time.Now().Unix()})
}


func (s *Site) handleUpload(c *gin.Context) {
	if !s.Fileuploads {
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New("uploads are disabled on this server"))
		return
	}

	file, info, err := c.Request.FormFile("file")
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		s.Logger.Error(uploadFailureMessage, err.Error())
		return
	}
	defer func() { _ = file.Close() }()

	fileInfo, err := s.FileStorer.Store(file)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		s.Logger.Error(uploadFailureMessage, err.Error())
		return
	}

	c.Header("Location", "/uploads/"+fileInfo.Hash+"?filename="+url.QueryEscape(info.Filename))
}

func (*Site) handleFavicon(c *gin.Context) {
	data, _ := static.StaticContent.ReadFile("img/favicon/favicon.ico")
	c.Data(http.StatusOK, contentTypeFromName("img/favicon/favicon.ico"), data)
}

func (s *Site) handleStatic(c *gin.Context, command string) {
	filename := strings.TrimPrefix(command, "/")
	var data []byte
	if filename == "css/custom.css" {
		data = s.CSS
	} else {
		var errAssset error
		data, errAssset = static.StaticContent.ReadFile(filename)
		if errAssset != nil {
			c.String(http.StatusNotFound, "Could not find data")
			return
		}
	}
	c.Data(http.StatusOK, contentTypeFromName(filename), data)
}

func (s *Site) handleUploads(c *gin.Context, command string) {
	if len(command) != 0 && command != rootPath && command != "/edit" {
		command = command[1:]

		// Reject path traversal and other malicious input
		if strings.Contains(command, "\x00") || strings.Contains(command, "..") {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		if !strings.HasSuffix(command, ".upload") {
			command = command + ".upload"
		}
		pathname := path.Join(s.PathToData, command)

		// Verify the resolved path is within PathToData (not the directory itself)
		cleanPath := filepath.Clean(pathname)
		cleanDataPath := filepath.Clean(s.PathToData)
		if !strings.HasPrefix(cleanPath, cleanDataPath+string(filepath.Separator)) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}

		c.Header(
			"Content-Disposition",
			"inline; filename=\""+c.DefaultQuery("filename", "upload")+"\"",
		)
		c.File(pathname)
		return
	}
	if !s.Fileuploads {
		_ = c.AbortWithError(http.StatusInternalServerError, errors.New("uploads are disabled on this server"))
		return
	}
}

func (*Site) handleRawContent(c *gin.Context, command string, p *wikipage.Page, rawText string) bool {
	if len(command) > maxContentLength && command[0:maxContentLength] == "/ra" {
		c.Writer.Header().Set("Content-Type", "text/plain")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, X-Max")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Data(httpStatusOK, contentTypeFromName(p.Identifier), []byte(rawText))
		return true
	}
	return false
}

func (*Site) handleFrontmatter(c *gin.Context, command string, p *wikipage.Page) bool {
	if strings.HasPrefix(command, "/frontmatter") {
		c.Data(http.StatusOK, gin.MIMEJSON, p.FrontmatterJSON)
		return true
	}
	return false
}

func contentTypeFromName(filename string) string {
	_ = mime.AddExtensionType(".md", "text/markdown")
	_ = mime.AddExtensionType(".heic", "image/heic")
	_ = mime.AddExtensionType(".heif", "image/heif")

	nameParts := strings.Split(filename, ".")
	mimeType := mime.TypeByExtension("." + nameParts[len(nameParts)-1])
	return mimeType
}

// sanitizePageName strips leading slashes from a wiki page name to prevent
// open redirect attacks. A page name that begins with a slash would cause
// "/"+page to produce a protocol-relative URL (e.g. "//evil.com") that
// browsers follow as an external redirect.
func sanitizePageName(page string) string {
	return strings.TrimLeft(page, "/")
}

// requestBaseURL derives the base URL (scheme://host) from the request context.
// It checks TLS state and the X-Forwarded-Proto header to determine the scheme.
func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if fwdProto := c.GetHeader("X-Forwarded-Proto"); fwdProto != "" {
		scheme = fwdProto
	}
	return scheme + "://" + c.Request.Host
}

// Test helper functions to expose private functions for testing

// GetSetSessionIDForTesting exposes getSetSessionID for testing
func GetSetSessionIDForTesting(c *gin.Context, logger *lumber.ConsoleLogger) string {
	return getSetSessionID(c, logger)
}

// GetRecentlyEditedForTesting exposes getRecentlyEdited for testing
func GetRecentlyEditedForTesting(title string, c *gin.Context, logger *lumber.ConsoleLogger) []string {
	return getRecentlyEdited(title, c, logger)
}

// RequestBaseURLForTesting exposes requestBaseURL for testing
func RequestBaseURLForTesting(c *gin.Context) string {
	return requestBaseURL(c)
}

// SanitizePageNameForTesting exposes sanitizePageName for testing
func SanitizePageNameForTesting(page string) string {
	return sanitizePageName(page)
}
