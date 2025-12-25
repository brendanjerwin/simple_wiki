//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	gorillasessions "github.com/gorilla/sessions"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// writeCloserBuffer wraps a bytes.Buffer to implement io.WriteCloser
type writeCloserBuffer struct {
	*bytes.Buffer
}

func (*writeCloserBuffer) Close() error {
	return nil
}

var _ = Describe("Handlers", func() {
	var site *server.Site
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.TRACE)
		site, err = server.NewSite(tmpDir, "", "testpage", 0, "secret", true, 1024, 1024, logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})


	Describe("handlePageUpdate", func() {
		When("the request has bad json", func() {
			BeforeEach(func() {
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer([]byte("bad json")))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the request is missing the page field", func() {
			BeforeEach(func() {
				body, _ := json.Marshal(map[string]string{})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the document is too large", func() {
			BeforeEach(func() {
				body, _ := json.Marshal(map[string]any{
					"page":     "test-update",
					"new_text": string(make([]byte, 2048)),
				})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		PWhen("the page update fails due to save error", func() {
			var response map[string]any
			var pageName string
			var originalMode os.FileMode
			var logBuffer *bytes.Buffer
			var customSite *server.Site
			var customRouter *gin.Engine
			var logOutput string

			BeforeEach(func() {
				pageName = "test-update-fail"
				newText := "new content"

				// Create a custom logger that writes to a buffer for this test
				logBuffer = &bytes.Buffer{}
				customLogger := lumber.NewBasicLogger(&writeCloserBuffer{logBuffer}, lumber.TRACE)
				var err error
				customSite, err = server.NewSite(tmpDir, "", "testpage", 0, "secret", true, 1024, 1024, customLogger)
				Expect(err).NotTo(HaveOccurred())
				customRouter = customSite.GinRouter()

				// Create and save a page first
				p, err := customSite.ReadPage(pageName)
				Expect(err).NotTo(HaveOccurred())
				_ = customSite.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "some content")

				// Make the data directory read-only to force save failure
				dataDir := customSite.PathToData
				info, _ := os.Stat(dataDir)
				originalMode = info.Mode()
				_ = os.Chmod(dataDir, 0555) // read + execute, but no write

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"new_text":   newText,
					"fetched_at": time.Now().Unix(),
				})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				customRouter.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)

				// Capture log output after action is performed
				logOutput = logBuffer.String()

				// Restore permissions for cleanup
				_ = os.Chmod(dataDir, originalMode)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a failure message", func() {
				Expect(response["success"]).To(BeFalse())
				Expect(response["message"]).To(Equal("Failed to save page"))
			})

			It("should log the error", func() {
				Expect(logOutput).To(ContainSubstring("Failed to save page 'test-update-fail'"))
				Expect(logOutput).To(ContainSubstring("ERROR"))
			})
		})

		When("the page is updated successfully", func() {
			var response map[string]any
			var pageName string
			var newText string

			BeforeEach(func() {
				pageName = "test-update"
				newText = "new content"
				p, err := site.ReadPage(pageName)
				Expect(err).NotTo(HaveOccurred())
				_ = site.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "some content")

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"new_text":   newText,
					"fetched_at": time.Now().Unix(),
				})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a success message", func() {
				Expect(response["success"]).To(BeTrue())
				Expect(response["message"]).To(Equal("Saved"))
			})

			It("should update the page content", func() {
				p, err := site.ReadPage(pageName)
				Expect(err).NotTo(HaveOccurred())
				Expect(p.Text).To(Equal(newText))
			})
		})

		PWhen("the page save fails during update", func() {
			var response map[string]any
			var pageName string
			var newText string
			var originalPermissions os.FileMode

			BeforeEach(func() {
				pageName = "test-save-fail"
				newText = "new content that should fail to save"
				
				// Create the page first
				p, err := site.ReadPage(pageName)
				Expect(err).NotTo(HaveOccurred())
				_ = site.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "initial content")
				_ = site.UpdatePageContent(p.Identifier, p.Text)

				// Make the data directory read-only to simulate save failure
				dirInfo, err := os.Stat(tmpDir)
				Expect(err).NotTo(HaveOccurred())
				originalPermissions = dirInfo.Mode()
				err = os.Chmod(tmpDir, 0555)
				Expect(err).NotTo(HaveOccurred())

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"new_text":   newText,
					"fetched_at": time.Now().Unix(),
				})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			AfterEach(func() {
				// Restore permissions for cleanup
				_ = os.Chmod(tmpDir, originalPermissions)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a failure response", func() {
				Expect(response["success"]).To(BeFalse())
				Expect(response["message"]).To(ContainSubstring("Failed to save"))
			})
		})

		When("the page save fails due to filesystem error", func() {
			var response map[string]any
			var pageName string
			var newText string
			var originalDataPath string

			BeforeEach(func() {
				pageName = "test-update-fail"
				newText = "new content"
				p, err := site.ReadPage(pageName)
				Expect(err).NotTo(HaveOccurred())
				_ = site.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "some content")

				// Create a read-only directory to cause save failure
				readOnlyDir, err := os.MkdirTemp("", "readonly")
				Expect(err).NotTo(HaveOccurred())
				err = os.Chmod(readOnlyDir, 0555) // Read + execute, but no write
				Expect(err).NotTo(HaveOccurred())

				// Change data path to read-only directory
				originalDataPath = site.PathToData
				site.PathToData = readOnlyDir

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"new_text":   newText,
					"fetched_at": time.Now().Unix(),
				})
				req, _ := http.NewRequest(http.MethodPost, "/update", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			AfterEach(func() {
				// Restore original data path
				site.PathToData = originalDataPath
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a failure response", func() {
				Expect(response["success"]).To(BeFalse())
				Expect(response["message"]).To(ContainSubstring("permission denied"))
			})
		})
	})
})

var _ = Describe("handleUploads Path Injection Prevention", func() {
	var router *gin.Engine
	var w *httptest.ResponseRecorder
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.TRACE)
		site, err := server.NewSite(tmpDir, "", "testpage", 0, "secret", true, 1024, 1024, logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	When("requesting a path with directory traversal (..) sequence", func() {
		BeforeEach(func() {
			req, _ := http.NewRequest(http.MethodGet, "/uploads/../etc/passwd", nil)
			router.ServeHTTP(w, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("requesting a path with encoded directory traversal", func() {
		BeforeEach(func() {
			req, _ := http.NewRequest(http.MethodGet, "/uploads/..%2f..%2fetc/passwd", nil)
			router.ServeHTTP(w, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("requesting a path with null byte injection", func() {
		BeforeEach(func() {
			req, _ := http.NewRequest(http.MethodGet, "/uploads/safe%00.evil", nil)
			router.ServeHTTP(w, req)
		})

		It("should return 400 Bad Request", func() {
			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})
	})

	When("requesting a valid upload path", func() {
		BeforeEach(func() {
			// Create a test upload file
			uploadDir := tmpDir
			err := os.MkdirAll(uploadDir, 0755)
			Expect(err).NotTo(HaveOccurred())
			err = os.WriteFile(uploadDir+"/testfile.upload", []byte("test content"), 0644)
			Expect(err).NotTo(HaveOccurred())

			req, _ := http.NewRequest(http.MethodGet, "/uploads/testfile", nil)
			router.ServeHTTP(w, req)
		})

		It("should return 200 OK", func() {
			Expect(w.Code).To(Equal(http.StatusOK))
		})

		It("should serve the file content", func() {
			Expect(w.Body.String()).To(Equal("test content"))
		})
	})
})

var _ = Describe("Session Logging Functions", func() {
	var logBuffer *bytes.Buffer
	var logger *lumber.ConsoleLogger
	var site *server.Site
	var router *gin.Engine
	var tmpDir string

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_test")
		Expect(err).NotTo(HaveOccurred())

		// Create a buffer-backed logger to capture log output
		logBuffer = &bytes.Buffer{}
		logger = lumber.NewBasicLogger(&handlerTestWriteCloser{logBuffer}, lumber.ERROR)

		site, err = server.NewSite(tmpDir, "", "testpage", 0, "secret", true, 1024, 1024, logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("getSetSessionID", func() {
		When("session save succeeds", func() {
			var sessionID string
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Call the function under test
				sessionID = server.GetSetSessionIDForTesting(c, logger)
			})

			It("should return a non-empty session ID", func() {
				Expect(sessionID).NotTo(BeEmpty())
			})

			It("should not log any errors", func() {
				Expect(logBuffer.String()).To(BeEmpty())
			})

			It("should generate a consistent session ID on subsequent calls", func() {
				secondID := server.GetSetSessionIDForTesting(c, logger)
				Expect(secondID).To(Equal(sessionID))
			})
		})

		When("session save fails", func() {
			var sessionID string
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with a faulty store that will fail on save
				store := &failingSessionStore{}
				sessions.Sessions("_session", store)(c)

				// Call the function under test
				sessionID = server.GetSetSessionIDForTesting(c, logger)
			})

			It("should return a non-empty session ID", func() {
				Expect(sessionID).NotTo(BeEmpty())
			})

			It("should log session save failure", func() {
				logOutput := logBuffer.String()
				Expect(logOutput).To(ContainSubstring("Failed to save session"))
				Expect(logOutput).To(ContainSubstring("session save failed"))
			})
		})

		When("session contains invalid sid type", func() {
			var sessionID string
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Set invalid type in session (not a string)
				session := sessions.Default(c)
				session.Set("sid", 12345) // integer instead of string
				_ = session.Save()

				// Call the function under test
				sessionID = server.GetSetSessionIDForTesting(c, logger)
			})

			It("should generate a new session ID", func() {
				Expect(sessionID).NotTo(BeEmpty())
			})

			It("should not log any errors", func() {
				Expect(logBuffer.String()).To(BeEmpty())
			})
		})
	})

	Describe("getRecentlyEdited", func() {
		When("session save succeeds", func() {
			var recentPages []string
			var c *gin.Context
			var pageName string

			BeforeEach(func() {
				pageName = "test-page"
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Call the function under test
				recentPages = server.GetRecentlyEditedForTesting(pageName, c, logger)
			})

			It("should return an empty list for first call", func() {
				Expect(recentPages).To(BeEmpty())
			})

			It("should not log any errors", func() {
				Expect(logBuffer.String()).To(BeEmpty())
			})

			It("should track multiple pages", func() {
				// Add another page
				recentPages = server.GetRecentlyEditedForTesting("second-page", c, logger)
				Expect(recentPages).To(ContainElement("test-page"))
				Expect(recentPages).NotTo(ContainElement("second-page")) // Current page is excluded
			})
		})

		When("session save fails", func() {
			var recentPages []string
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with a faulty store that will fail on save
				store := &failingSessionStore{}
				sessions.Sessions("_session", store)(c)

				// Call the function under test
				recentPages = server.GetRecentlyEditedForTesting("test-page", c, logger)
			})

			It("should return an empty list", func() {
				Expect(recentPages).To(BeEmpty())
			})

			It("should log session save failure", func() {
				logOutput := logBuffer.String()
				Expect(logOutput).To(ContainSubstring("Failed to save session"))
				Expect(logOutput).To(ContainSubstring("session save failed"))
			})
		})

		When("session contains invalid recentlyEdited type", func() {
			var recentPages []string
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Set invalid type in session (not a string)
				session := sessions.Default(c)
				session.Set("recentlyEdited", []string{"invalid", "type"}) // slice instead of string
				_ = session.Save()

				// Call the function under test
				recentPages = server.GetRecentlyEditedForTesting("test-page", c, logger)
			})

			It("should return an empty list", func() {
				Expect(recentPages).To(BeEmpty())
			})

			It("should not log any errors", func() {
				Expect(logBuffer.String()).To(BeEmpty())
			})
		})

		When("handling special cases", func() {
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Set up some initial recent pages including icons and duplicates
				session := sessions.Default(c)
				session.Set("recentlyEdited", "page1|||icon-test|||duplicate-page|||page2")
				_ = session.Save()
			})

			It("should filter out icon pages", func() {
				recentPages := server.GetRecentlyEditedForTesting("new-page", c, logger)
				Expect(recentPages).To(ContainElement("page1"))
				Expect(recentPages).To(ContainElement("duplicate-page"))
				Expect(recentPages).To(ContainElement("page2"))
				Expect(recentPages).NotTo(ContainElement("icon-test"))
			})

			It("should exclude current page from results", func() {
				recentPages := server.GetRecentlyEditedForTesting("duplicate-page", c, logger)
				Expect(recentPages).To(ContainElement("page1"))
				Expect(recentPages).To(ContainElement("page2"))
				Expect(recentPages).NotTo(ContainElement("duplicate-page"))
				Expect(recentPages).NotTo(ContainElement("icon-test"))
			})

			It("should not add duplicate pages", func() {
				// First call adds the page
				server.GetRecentlyEditedForTesting("page1", c, logger)
				
				// Second call should not duplicate it
				_ = server.GetRecentlyEditedForTesting("different-page", c, logger)
				
				// Count occurrences of "page1" in the session
				session := sessions.Default(c)
				recentStrVal := session.Get("recentlyEdited")
				recentStr, ok := recentStrVal.(string)
				Expect(ok).To(BeTrue())
				count := strings.Count(recentStr, "page1")
				Expect(count).To(Equal(1)) // Should only appear once
			})
		})
	})

	Describe("Integration with handlers", func() {
		When("handler methods call session functions", func() {
			var w *httptest.ResponseRecorder

			BeforeEach(func() {
				w = httptest.NewRecorder()

				// Create a page to work with
				p, err := site.ReadPage("test-integration")
				Expect(err).NotTo(HaveOccurred())
				_ = site.UpdatePageContent(wikipage.PageIdentifier(p.Identifier), "test content")
				_ = site.UpdatePageContent(p.Identifier, p.Text)
			})

			It("should pass logger to session functions in handlePageRequest", func() {
				req, _ := http.NewRequest(http.MethodGet, "/test-integration", nil)
				router.ServeHTTP(w, req)

				// Should return either 200 or 302 (redirect to edit if new page)
				Expect(w.Code).To(SatisfyAny(Equal(http.StatusOK), Equal(http.StatusFound)))
				Expect(logBuffer.String()).NotTo(ContainSubstring("Failed to save session"))
			})
		})
	})
})

// handlerTestWriteCloser wraps a buffer to implement io.WriteCloser for logger testing
type handlerTestWriteCloser struct {
	*bytes.Buffer
}

func (*handlerTestWriteCloser) Close() error {
	return nil
}

// failingSessionStore is a mock session store that fails on Save operations
type failingSessionStore struct {
	cookie.Store
}

func (*failingSessionStore) Options(options sessions.Options) {
	// No-op
}

func (f *failingSessionStore) Get(r *http.Request, name string) (*gorillasessions.Session, error) {
	session := gorillasessions.NewSession(f, name)
	session.Values = make(map[any]any)
	session.IsNew = true
	return session, nil
}

func (f *failingSessionStore) New(r *http.Request, name string) (*gorillasessions.Session, error) {
	return f.Get(r, name)
}

func (*failingSessionStore) Save(r *http.Request, w http.ResponseWriter, s *gorillasessions.Session) error {
	return errors.New("session save failed")
}
