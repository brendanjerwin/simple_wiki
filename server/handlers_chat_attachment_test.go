//revive:disable:dot-imports
package server_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/brendanjerwin/simple_wiki/pkg/chatattachment"
	"github.com/brendanjerwin/simple_wiki/server"
	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// fakeChatAttachmentStore is a controllable AttachmentStore for handler testing.
type fakeChatAttachmentStore struct {
	storeFunc  func(filename, mimeType string, content io.Reader) (string, error)
	getFunc    func(id string) (*chatattachment.Attachment, error)
	deleteFunc func(id string) error
}

func (f *fakeChatAttachmentStore) Store(filename, mimeType string, content io.Reader) (string, error) {
	return f.storeFunc(filename, mimeType, content)
}

func (f *fakeChatAttachmentStore) Get(id string) (*chatattachment.Attachment, error) {
	return f.getFunc(id)
}

func (f *fakeChatAttachmentStore) Delete(id string) error {
	if f.deleteFunc != nil {
		return f.deleteFunc(id)
	}
	return nil
}

// buildChatUploadRequest creates a multipart/form-data POST request with a "file" field.
func buildChatUploadRequest(filename string, data []byte) *http.Request {
	GinkgoHelper()
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	Expect(err).NotTo(HaveOccurred())
	_, err = part.Write(data)
	Expect(err).NotTo(HaveOccurred())
	Expect(writer.Close()).To(Succeed())

	req := httptest.NewRequest(http.MethodPost, "/api/chat-attachments", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

var _ = Describe("Chat Attachment Handlers", func() {
	var (
		site   *server.Site
		router *gin.Engine
		tmpDir string
	)

	BeforeEach(func() {
		var err error
		tmpDir, err = os.MkdirTemp("", "simple_wiki_chat_attachment_handler_test")
		Expect(err).NotTo(HaveOccurred())
		logger := lumber.NewConsoleLogger(lumber.WARN)
		site, err = server.NewSite(tmpDir, "testpage", 0, "secret", logger)
		Expect(err).NotTo(HaveOccurred())
		router = site.GinRouter()
	})

	AfterEach(func() {
		_ = os.RemoveAll(tmpDir)
	})

	Describe("POST /api/chat-attachments", func() {
		When("ChatAttachmentStore is nil", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = nil
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/chat-attachments", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 503", func() {
				Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})

		When("the request is not a valid multipart form", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/chat-attachments",
					bytes.NewReader([]byte("not multipart")))
				req.Header.Set("Content-Type", "application/json")
				router.ServeHTTP(rec, req)
			})

			It("should respond 400", func() {
				Expect(rec.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the 'file' field is missing from the multipart form", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				body := new(bytes.Buffer)
				writer := multipart.NewWriter(body)
				Expect(writer.WriteField("other", "value")).To(Succeed())
				Expect(writer.Close()).To(Succeed())

				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/chat-attachments", body)
				req.Header.Set("Content-Type", writer.FormDataContentType())
				router.ServeHTTP(rec, req)
			})

			It("should respond 400", func() {
				Expect(rec.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the store returns an error", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					storeFunc: func(_, _ string, _ io.Reader) (string, error) {
						return "", errors.New("file exceeds maximum size")
					},
				}
				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, buildChatUploadRequest("test.png", []byte("image data")))
			})

			It("should respond 422", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			})
		})

		When("a valid file is uploaded", func() {
			var (
				rec      *httptest.ResponseRecorder
				response map[string]any
			)

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					storeFunc: func(_, _ string, _ io.Reader) (string, error) {
						return "test-attach-id", nil
					},
				}
				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, buildChatUploadRequest("photo.png", []byte("fake image data")))

				response = map[string]any{}
				Expect(json.Unmarshal(rec.Body.Bytes(), &response)).To(Succeed())
			})

			It("should respond 201", func() {
				Expect(rec.Code).To(Equal(http.StatusCreated))
			})

			It("should return the attachment ID", func() {
				Expect(response["id"]).To(Equal("test-attach-id"))
			})

			It("should return the URL", func() {
				Expect(response["url"]).To(Equal("/api/chat-attachments/test-attach-id"))
			})

			It("should return the filename", func() {
				Expect(response["filename"]).To(Equal("photo.png"))
			})
		})
	})

	Describe("GET /api/chat-attachments/:id", func() {
		When("ChatAttachmentStore is nil", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = nil
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/chat-attachments/some-id", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 503", func() {
				Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
			})
		})

		When("the attachment is not found", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					getFunc: func(_ string) (*chatattachment.Attachment, error) {
						return nil, chatattachment.ErrNotFound
					},
				}
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/chat-attachments/nonexistent", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 404", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("the attachment ID is invalid", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					getFunc: func(_ string) (*chatattachment.Attachment, error) {
						return nil, chatattachment.ErrInvalidID
					},
				}
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/chat-attachments/bad..id", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 400", func() {
				Expect(rec.Code).To(Equal(http.StatusBadRequest))
			})
		})

		When("the store returns an unexpected error", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					getFunc: func(_ string) (*chatattachment.Attachment, error) {
						return nil, errors.New("storage backend unavailable")
					},
				}
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/chat-attachments/some-id", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 500", func() {
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		When("the attachment exists", func() {
			var rec *httptest.ResponseRecorder

			BeforeEach(func() {
				site.ChatAttachmentStore = &fakeChatAttachmentStore{
					getFunc: func(id string) (*chatattachment.Attachment, error) {
						return &chatattachment.Attachment{
							ID:       id,
							Filename: "photo.png",
							MIMEType: "image/png",
							Data:     []byte("fake image data"),
						}, nil
					},
				}
				rec = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/api/chat-attachments/test-id", nil)
				router.ServeHTTP(rec, req)
			})

			It("should respond 200", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))
			})

			It("should return the attachment data", func() {
				Expect(rec.Body.Bytes()).To(Equal([]byte("fake image data")))
			})

			It("should set the Content-Type header", func() {
				Expect(rec.Header().Get("Content-Type")).To(Equal("image/png"))
			})

			It("should set the Content-Disposition header", func() {
				Expect(rec.Header().Get("Content-Disposition")).To(ContainSubstring("photo.png"))
			})
		})
	})
})
