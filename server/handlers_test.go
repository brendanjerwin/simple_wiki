//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/gin-gonic/gin"
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

			It("should return a 500 status code", func() {
				Expect(w.Code).To(Equal(http.StatusInternalServerError))
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
})
