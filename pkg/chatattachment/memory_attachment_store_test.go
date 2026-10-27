package chatattachment_test

import (
	"bytes"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/brendanjerwin/simple_wiki/pkg/chatattachment"
)

var _ = Describe("MemoryAttachmentStore", func() {
	var store *chatattachment.MemoryAttachmentStore

	BeforeEach(func() {
		store = chatattachment.NewMemoryAttachmentStore(chatattachment.DefaultTTL)
	})

	Describe("Store", func() {
		When("called with valid parameters", func() {
			var (
				id  string
				err error
			)

			BeforeEach(func() {
				id, err = store.Store("photo.png", "image/png", bytes.NewReader([]byte("fakeimagedata")))
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return a non-empty ID", func() {
				Expect(id).NotTo(BeEmpty())
			})
		})

		When("filename is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = store.Store("", "image/png", bytes.NewReader([]byte("data")))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("mimeType is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = store.Store("photo.png", "", bytes.NewReader([]byte("data")))
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("content exceeds MaxAttachmentBytes", func() {
			var err error

			BeforeEach(func() {
				big := strings.NewReader(strings.Repeat("x", chatattachment.MaxAttachmentBytes+1))
				_, err = store.Store("big.bin", "application/octet-stream", big)
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("exceeds maximum size"))
			})
		})
	})

	Describe("Get", func() {
		When("the attachment exists", func() {
			var (
				id         string
				attachment *chatattachment.Attachment
				err        error
			)

			BeforeEach(func() {
				var storeErr error
				id, storeErr = store.Store("photo.png", "image/png", bytes.NewReader([]byte("fakeimagedata")))
				Expect(storeErr).NotTo(HaveOccurred())

				attachment, err = store.Get(id)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the correct filename", func() {
				Expect(attachment.Filename).To(Equal("photo.png"))
			})

			It("should return the correct MIME type", func() {
				Expect(attachment.MIMEType).To(Equal("image/png"))
			})

			It("should return the correct data", func() {
				Expect(attachment.Data).To(Equal([]byte("fakeimagedata")))
			})

			It("should return the matching ID", func() {
				Expect(attachment.ID).To(Equal(id))
			})
		})

		When("the attachment does not exist", func() {
			var err error

			BeforeEach(func() {
				_, err = store.Get("nonexistent-id")
			})

			It("should return ErrNotFound", func() {
				Expect(err).To(MatchError(chatattachment.ErrNotFound))
			})
		})

		When("the ID contains path traversal characters", func() {
			var err error

			BeforeEach(func() {
				_, err = store.Get("../etc/passwd")
			})

			It("should return ErrInvalidID", func() {
				Expect(err).To(MatchError(chatattachment.ErrInvalidID))
			})
		})

		When("the ID is empty", func() {
			var err error

			BeforeEach(func() {
				_, err = store.Get("")
			})

			It("should return ErrInvalidID", func() {
				Expect(err).To(MatchError(chatattachment.ErrInvalidID))
			})
		})

		When("the attachment has expired", func() {
			var err error

			BeforeEach(func() {
				shortTTLStore := chatattachment.NewMemoryAttachmentStore(1 * time.Nanosecond)
				id, storeErr := shortTTLStore.Store("photo.png", "image/png", bytes.NewReader([]byte("data")))
				Expect(storeErr).NotTo(HaveOccurred())

				// Wait for TTL to expire
				time.Sleep(2 * time.Millisecond)

				_, err = shortTTLStore.Get(id)
			})

			It("should return ErrNotFound", func() {
				Expect(err).To(MatchError(chatattachment.ErrNotFound))
			})
		})
	})

	Describe("Delete", func() {
		When("the attachment exists", func() {
			var (
				id  string
				err error
			)

			BeforeEach(func() {
				var storeErr error
				id, storeErr = store.Store("photo.png", "image/png", bytes.NewReader([]byte("data")))
				Expect(storeErr).NotTo(HaveOccurred())

				err = store.Delete(id)
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should make the attachment unavailable", func() {
				_, getErr := store.Get(id)
				Expect(getErr).To(MatchError(chatattachment.ErrNotFound))
			})
		})

		When("the attachment does not exist", func() {
			var err error

			BeforeEach(func() {
				err = store.Delete("nonexistent-id")
			})

			It("should return ErrNotFound", func() {
				Expect(err).To(MatchError(chatattachment.ErrNotFound))
			})
		})

		When("the ID contains path traversal characters", func() {
			var err error

			BeforeEach(func() {
				err = store.Delete("../etc/passwd")
			})

			It("should return ErrInvalidID", func() {
				Expect(err).To(MatchError(chatattachment.ErrInvalidID))
			})
		})
	})
})
