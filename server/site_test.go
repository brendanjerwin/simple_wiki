package server

import (
	"os"
	"path"

	"github.com/brendanjerwin/simple_wiki/sec"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Site Functions", func() {
	var (
		s *Site
	)

	BeforeEach(func() {
		s = &Site{
			Logger: lumber.NewConsoleLogger(lumber.INFO),
		}
	})

	Describe("defaultLock", func() {
		When("DefaultPassword is not set", func() {
			BeforeEach(func() {
				s.DefaultPassword = ""
			})

			It("should return an empty string", func() {
				Expect(s.defaultLock()).To(BeEmpty())
			})
		})

		When("DefaultPassword is set", func() {
			var password string

			BeforeEach(func() {
				password = "test_password"
				s.DefaultPassword = password
			})

			It("should return a valid hash of the password", func() {
				hashedPassword := s.defaultLock()
				Expect(hashedPassword).ToNot(BeEmpty())
				Expect(hashedPassword).ToNot(Equal(password))
				Expect(sec.CheckPasswordHash(password, hashedPassword)).To(Succeed())
			})
		})
	})

	Describe("sniffContentType", func() {
		var (
			pathToData string
		)

		BeforeEach(func() {
			pathToData = "testdata_site_sniff"
			err := os.MkdirAll(pathToData, 0755)
			Expect(err).NotTo(HaveOccurred())
			s.PathToData = pathToData
		})

		AfterEach(func() {
			os.RemoveAll(pathToData)
		})

		When("the file is an image", func() {
			var (
				contentType string
				err         error
			)

			BeforeEach(func() {
				// a minimal png file
				// from https://github.com/mathiasbynens/small/blob/master/png-transparent.png
				pngData := []byte{
					0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00,
					0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01,
					0x00, 0x00, 0x00, 0x01, 0x08, 0x06, 0x00, 0x00, 0x00, 0x1f,
					0x15, 0xc4, 0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
					0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05, 0x00,
					0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
					0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
				}
				err = os.WriteFile(path.Join(pathToData, "test.png"), pngData, 0644)
				Expect(err).NotTo(HaveOccurred())

				contentType, err = s.sniffContentType("test.png")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'image/png'", func() {
				Expect(contentType).To(Equal("image/png"))
			})
		})

		When("the file is plain text", func() {
			var (
				contentType string
				err         error
			)

			BeforeEach(func() {
				err = os.WriteFile(path.Join(pathToData, "test.txt"), []byte("this is plain text"), 0644)
				Expect(err).NotTo(HaveOccurred())

				contentType, err = s.sniffContentType("test.txt")
			})

			It("should not return an error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'text/plain; charset=utf-8'", func() {
				Expect(contentType).To(Equal("text/plain; charset=utf-8"))
			})
		})

		When("the file does not exist", func() {
			var (
				err error
			)

			BeforeEach(func() {
				_, err = s.sniffContentType("nonexistent.file")
			})

			It("should return an error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
