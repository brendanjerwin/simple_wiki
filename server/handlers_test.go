//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	gorillasessions "github.com/gorilla/sessions"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

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
		logger = lumber.NewBasicLogger(&testWriteCloser{logBuffer}, lumber.ERROR)

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

// testWriteCloser wraps a buffer to implement io.WriteCloser for logger testing
type testWriteCloser struct {
	*bytes.Buffer
}

func (*testWriteCloser) Close() error {
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
