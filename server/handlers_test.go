//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
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
		site = server.NewSite(tmpDir, "", "testpage", "password", 0, "secret", "", true, 1024, 1024, logger)
		router = site.GinRouter()
		w = httptest.NewRecorder()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("handlePageRelinquish", func() {
		When("the request has bad json", func() {
			BeforeEach(func() {
				req, _ := http.NewRequest(http.MethodPost, "/relinquish", bytes.NewBuffer([]byte("bad json")))
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
				req, _ := http.NewRequest(http.MethodPost, "/relinquish", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the page is relinquished successfully", func() {
			var response map[string]any
			var pageName string

			BeforeEach(func() {
				pageName = "test-relinquish"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				body, _ := json.Marshal(map[string]string{"page": pageName})
				req, _ := http.NewRequest(http.MethodPost, "/relinquish", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a success message", func() {
				Expect(response["success"]).To(BeTrue())
				Expect(response["message"]).To(Equal("Relinquished and erased"))
			})

			It("should erase the page", func() {
				p := site.Open(pageName)
				Expect(p.Text.GetCurrent()).To(BeEmpty())
			})
		})

		When("the page erase fails during relinquish", func() {
			var response map[string]any
			var pageName string

			BeforeEach(func() {
				pageName = "test-relinquish-fail"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				// Make the data directory read-only to cause erase to fail
				_ = os.Chmod(tmpDir, 0444)

				body, _ := json.Marshal(map[string]string{"page": pageName})
				req, _ := http.NewRequest(http.MethodPost, "/relinquish", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)

				// Restore permissions for cleanup
				_ = os.Chmod(tmpDir, 0755)
			})

			It("should return a 500 status code", func() {
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return an error message", func() {
				Expect(response["success"]).To(BeFalse())
				Expect(response["message"]).To(Equal("Failed to erase page"))
			})
		})
	})

	Describe("handlePageExists", func() {
		When("the request has bad json", func() {
			BeforeEach(func() {
				req, _ := http.NewRequest(http.MethodPost, "/exists", bytes.NewBuffer([]byte("bad json")))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the page exists", func() {
			var response map[string]any
			var pageName string

			BeforeEach(func() {
				pageName = "test-exists"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				body, _ := json.Marshal(map[string]string{"page": pageName})
				req, _ := http.NewRequest(http.MethodPost, "/exists", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a success message", func() {
				Expect(response["success"]).To(BeTrue())
				Expect(response["exists"]).To(BeTrue())
			})
		})

		When("the page does not exist", func() {
			var response map[string]any
			var pageName string

			BeforeEach(func() {
				pageName = "test-does-not-exist"

				body, _ := json.Marshal(map[string]string{"page": pageName})
				req, _ := http.NewRequest(http.MethodPost, "/exists", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a success message", func() {
				Expect(response["success"]).To(BeTrue())
				Expect(response["exists"]).To(BeFalse())
			})
		})
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

		When("the page update fails due to save error", func() {
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
				customSite = server.NewSite(tmpDir, "", "testpage", "password", 0, "secret", "", true, 1024, 1024, customLogger)
				customRouter = customSite.GinRouter()

				// Create and save a page first
				p := customSite.Open(pageName)
				_ = p.Update("some content")

				// Make the data directory read-only to force save failure
				dataDir := customSite.PathToData
				info, _ := os.Stat(dataDir)
				originalMode = info.Mode()
				_ = os.Chmod(dataDir, 0444) // read-only

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
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

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
				p := site.Open(pageName)
				Expect(p.Text.GetCurrent()).To(Equal(newText))
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
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				// Create a read-only directory to cause save failure
				readOnlyDir, err := os.MkdirTemp("", "readonly")
				Expect(err).NotTo(HaveOccurred())
				err = os.Chmod(readOnlyDir, 0444) // Read-only directory
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
				Expect(response["message"]).To(Equal("Failed to save page"))
			})
		})
	})

	Describe("handleLock", func() {
		When("the request has bad json", func() {
			BeforeEach(func() {
				req, _ := http.NewRequest(http.MethodPost, "/lock", bytes.NewBuffer([]byte("bad json")))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
			})

			It("should return a 400 error", func() {
				Expect(w.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the page is locked successfully", func() {
			var response map[string]any
			var pageName string

			BeforeEach(func() {
				pageName = "test-lock"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"passphrase": "testpass",
				})
				req, _ := http.NewRequest(http.MethodPost, "/lock", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			It("should return a 200 status code", func() {
				Expect(w.Code).To(Equal(http.StatusOK))
			})

			It("should return a success message", func() {
				Expect(response["success"]).To(BeTrue())
				Expect(response["message"]).To(Equal("Locked"))
			})

			It("should lock the page", func() {
				p := site.Open(pageName)
				Expect(p.IsLocked).To(BeTrue())
			})
		})

		When("the lock save fails due to filesystem error", func() {
			var response map[string]any
			var pageName string
			var originalDataPath string

			BeforeEach(func() {
				pageName = "test-lock-fail"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				// Create a read-only directory to cause save failure
				readOnlyDir, err := os.MkdirTemp("", "readonly")
				Expect(err).NotTo(HaveOccurred())
				err = os.Chmod(readOnlyDir, 0444) // Read-only directory
				Expect(err).NotTo(HaveOccurred())

				// Change data path to read-only directory
				originalDataPath = site.PathToData
				site.PathToData = readOnlyDir

				body, _ := json.Marshal(map[string]any{
					"page":       pageName,
					"passphrase": "password", // Use the default password
				})
				req, _ := http.NewRequest(http.MethodPost, "/lock", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(w, req)
				_ = json.Unmarshal(w.Body.Bytes(), &response)
			})

			AfterEach(func() {
				// Restore original data path
				site.PathToData = originalDataPath
			})

			It("should return a 500 status code", func() {
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should return a failure response", func() {
				Expect(response["success"]).To(BeFalse())
				Expect(response["message"]).To(Equal("Failed to save lock information"))
			})
		})
	})

	Describe("handlePageRequest erase command", func() {
		When("the erase command is successful", func() {
			var pageName string

			BeforeEach(func() {
				pageName = "test-erase"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				req, _ := http.NewRequest(http.MethodGet, "/"+pageName+"/erase", nil)
				router.ServeHTTP(w, req)
			})

			It("should redirect to root path", func() {
				Expect(w.Code).To(Equal(http.StatusFound))
				Expect(w.Header().Get("Location")).To(Equal("/"))
			})

			It("should erase the page", func() {
				p := site.Open(pageName)
				Expect(p.Text.GetCurrent()).To(BeEmpty())
			})
		})

		When("the erase command fails", func() {
			var pageName string

			BeforeEach(func() {
				pageName = "test-erase-fail"
				p := site.Open(pageName)
				_ = p.Update("some content")
				_ = p.Save()

				// Make the specific files read-only to cause erase to fail
				// (directory can still be read, but files can't be deleted)
				jsonFile := path.Join(tmpDir, base32tools.EncodeToBase32(strings.ToLower(pageName))+".json")
				mdFile := path.Join(tmpDir, base32tools.EncodeToBase32(strings.ToLower(pageName))+".md")
				_ = os.Chmod(jsonFile, 0444)
				_ = os.Chmod(mdFile, 0444)
				_ = os.Chmod(tmpDir, 0555) // Read-only directory (can read, but can't modify/delete files)

				req, _ := http.NewRequest(http.MethodGet, "/"+pageName+"/erase", nil)
				router.ServeHTTP(w, req)

				// Restore permissions for cleanup
				_ = os.Chmod(tmpDir, 0755)
				_ = os.Chmod(jsonFile, 0644)
				_ = os.Chmod(mdFile, 0644)
			})

			It("should return internal server error", func() {
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
			})

			It("should not erase the page", func() {
				p := site.Open(pageName)
				Expect(p.Text.GetCurrent()).To(Equal("some content"))
			})
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

		site = server.NewSite(tmpDir, "", "testpage", "password", 0, "secret", "", true, 1024, 1024, logger)
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

			It("should generate a new session ID when type assertion fails", func() {
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

			It("should return an empty list when type assertion fails", func() {
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

			It("should not add duplicate when page already exists", func() {
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

	Describe("pageIsLocked", func() {
		When("calling with logger parameter", func() {
			var isLocked bool
			var page *server.Page
			var c *gin.Context

			BeforeEach(func() {
				w := httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
				c.Request, _ = http.NewRequest("GET", "/", nil)

				// Set up session middleware with cookie store
				store := cookie.NewStore([]byte("test-secret"))
				sessions.Sessions("_session", store)(c)

				// Create a test page
				page = site.Open("test-page")

				// Call the function under test
				isLocked = server.PageIsLockedForTesting(page, c, logger)
			})

			It("should return false for unlocked page", func() {
				Expect(isLocked).To(BeFalse())
			})

			It("should not log any errors for normal operation", func() {
				Expect(logBuffer.String()).To(BeEmpty())
			})
		})
	})

	Describe("Integration with handlers", func() {
		When("handler methods call session functions", func() {
			var w *httptest.ResponseRecorder

			BeforeEach(func() {
				w = httptest.NewRecorder()

				// Create a page to work with
				p := site.Open("test-integration")
				_ = p.Update("test content")
				_ = p.Save()
			})

			It("should pass logger to session functions in handlePageRelinquish", func() {
				body, _ := json.Marshal(map[string]string{"page": "test-integration"})
				req, _ := http.NewRequest(http.MethodPost, "/relinquish", bytes.NewBuffer(body))
				req.Header.Set("Content-Type", "application/json")

				router.ServeHTTP(w, req)

				// Should succeed without session logging errors
				Expect(w.Code).To(Equal(http.StatusOK))
				Expect(logBuffer.String()).NotTo(ContainSubstring("Failed to save session"))
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
