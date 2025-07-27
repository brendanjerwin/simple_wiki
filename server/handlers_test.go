//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"time"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/brendanjerwin/simple_wiki/utils/base32tools"
	"github.com/gin-gonic/gin"
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
