package server

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/brendanjerwin/simple_wiki/utils"
	"github.com/jcelliott/lumber"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Page Suite")
}

var _ = Describe("Page Functions", func() {
	var (
		pathToData string
		s          *Site
	)

	BeforeEach(func() {
		pathToData = "testdata_page"
		err := os.MkdirAll(pathToData, 0755)
		Expect(err).NotTo(HaveOccurred())
		s = &Site{
			PathToData:       pathToData,
			MarkdownRenderer: &utils.GoldmarkRenderer{},
			Logger:           lumber.NewConsoleLogger(lumber.INFO),
		}
		err = s.InitializeIndexing()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		os.RemoveAll(pathToData)
	})

	Describe("Site.DirectoryList", func() {
		When("there are pages", func() {
			var pages []os.FileInfo

			BeforeEach(func() {
				req, _ := http.NewRequest("GET", "/", nil)
				p := s.OpenOrInit("testpage", req)
				err := p.Update("Some data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p = s.OpenOrInit("testpage2", req)
				err = p.Update("A different bunch of data")
				Expect(err).ToNot(HaveOccurred())
				time.Sleep(10 * time.Millisecond)

				p = s.OpenOrInit("testpage3", req)
				err = p.Update("Not much else")
				Expect(err).ToNot(HaveOccurred())

				pages = s.DirectoryList()
			})

			It("should return all the pages", func() {
				Expect(pages).To(HaveLen(3))
			})

			It("should sort pages by most recently modified", func() {
				Expect(pages[0].Name()).To(Equal("testpage"))
				Expect(pages[1].Name()).To(Equal("testpage2"))
				Expect(pages[2].Name()).To(Equal("testpage3"))
			})
		})
	})

	Describe("Page update and render", func() {
		var p *Page

		BeforeEach(func() {
			req, _ := http.NewRequest("GET", "/", nil)
			p = s.OpenOrInit("testpage", req)
		})

		When("A page is updated", func() {
			BeforeEach(func() {
				err := p.Update("**bold**")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should render correctly", func() {
				Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong></p>"))
			})

			When("the page is updated again", func() {
				BeforeEach(func() {
					err := p.Update("**bold** and *italic*")
					Expect(err).ToNot(HaveOccurred())
					err = p.Save()
					Expect(err).ToNot(HaveOccurred())
				})

				It("should render the new content", func() {
					Expect(string(p.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
				})

				When("the page is retrieved from disk", func() {
					var p2 *Page

					BeforeEach(func() {
						p2 = s.Open("testpage")
					})

					It("should have its content preserved", func() {
						Expect(p2.Text.GetCurrent()).To(Equal("**bold** and *italic*"))
					})

					When("the retrieved page is rendered", func() {
						BeforeEach(func() {
							p2.Render()
						})

						It("should render correctly", func() {
							Expect(string(p2.RenderedPage)).To(ContainSubstring("<p><strong>bold</strong> and <em>italic</em></p>"))
						})
					})
				})
			})
		})
	})
})
