//revive:disable:dot-imports
package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/brendanjerwin/simple_wiki/index/bleve"
	"github.com/brendanjerwin/simple_wiki/wikipage"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("API Handlers", func() {
	Describe("Helpers", func() {
		Describe("createPageReferences", func() {
			When("creating page references from a list of IDs", func() {
				var s *Site
				var ids []string
				var actual []PageReference

				BeforeEach(func() {
					s = &Site{
						FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{
							data: map[string]map[string]string{
								"id1": {"title": "Title 1"},
								"id2": {"title": "Title 2"},
								"id3": {},
							},
						},
					}
					ids = []string{"id1", "id2", "id3", "id4"}

					actual = s.createPageReferences(ids)
				})

				It("should create PageReference objects from a list of IDs", func() {
					expected := []PageReference{
						{Identifier: "id1", Title: "Title 1"},
						{Identifier: "id2", Title: "Title 2"},
						{Identifier: "id3", Title: ""},
						{Identifier: "id4", Title: ""},
					}

					Expect(actual).To(Equal(expected))
				})
			})
		})

		Describe("executeFrontmatterQuery", func() {
			var s *Site
			var w *httptest.ResponseRecorder
			var c *gin.Context

			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				s = &Site{
					FrontmatterIndexQueryer: &mockFrontmatterIndexQueryer{
						data: map[string]map[string]string{
							"id1": {"title": "Title A"},
							"id2": {"title": "Title B"},
						},
					},
				}
				w = httptest.NewRecorder()
				c, _ = gin.CreateTestContext(w)
			})

			When("the FrontmatterIndexQueryer is not initialized", func() {
				BeforeEach(func() {
					s.FrontmatterIndexQueryer = nil
					req, err := http.NewRequest(http.MethodGet, "/?k=key&v=val", nil)
					Expect(err).NotTo(HaveOccurred())
					c.Request = req
					type TestReq struct {
						Key string `form:"k" binding:"required"`
						Val string `form:"v"`
					}
					var testReq TestReq

					executor := func() []string {
						Fail("executor should not be called")
						return nil
					}
					s.executeFrontmatterQuery(c, &testReq, executor)
				})

				It("returns a server error", func() {
					Expect(w.Code).To(Equal(http.StatusInternalServerError))
				})

				It("returns a helpful error message in JSON", func() {
					var response map[string]any
					err := json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
					Expect(response["success"]).To(BeFalse())
					Expect(response["message"]).To(Equal("Frontmatter index is not available"))
				})
			})

			When("binding the request fails", func() {
				BeforeEach(func() {
					req, err := http.NewRequest(http.MethodGet, "/?v=val", nil)
					Expect(err).NotTo(HaveOccurred())
					c.Request = req

					type TestReq struct {
						Key string `form:"k" binding:"required"`
						Val string `form:"v"`
					}
					var testReq TestReq

					executor := func() []string {
						Fail("executor should not be called")
						return nil
					}
					s.executeFrontmatterQuery(c, &testReq, executor)
				})

				It("returns a bad request error", func() {
					Expect(w.Code).To(Equal(http.StatusBadRequest))
				})

				It("returns a helpful error message", func() {
					Expect(w.Body.String()).To(ContainSubstring("Problem binding keys"))
				})
			})

			When("the query is successful", func() {
				var response struct {
					Success bool            `json:"success"`
					IDs     []PageReference `json:"ids"`
				}
				BeforeEach(func() {
					req, err := http.NewRequest(http.MethodGet, "/?k=key&v=val", nil)
					Expect(err).NotTo(HaveOccurred())
					c.Request = req

					type TestReq struct {
						Key string `form:"k" binding:"required"`
						Val string `form:"v"`
					}
					var testReq TestReq

					executor := func() []string {
						return []string{"id1", "id2"}
					}

					s.executeFrontmatterQuery(c, &testReq, executor)

					err = json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns http.StatusOK", func() {
					Expect(w.Code).To(Equal(http.StatusOK))
				})

				It("returns success=true", func() {
					Expect(response.Success).To(BeTrue())
				})

				It("returns a list of page references", func() {
					expectedIDs := []PageReference{
						{Identifier: "id1", Title: "Title A"},
						{Identifier: "id2", Title: "Title B"},
					}
					Expect(response.IDs).To(Equal(expectedIDs))
				})
			})
		})
	})

	Describe("Handlers", func() {
		var s *Site
		var w *httptest.ResponseRecorder
		var router *gin.Engine

		Describe("handleFindBy", func() {
			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				mockFmIndex := &mockFrontmatterIndexQueryer{
					data: map[string]map[string]string{
						"id1": {"title": "Title A"},
					},
					QueryExactMatchFunc: func(key, value string) []string {
						if key == "test.key" && value == "test_value" {
							return []string{"id1"}
						}
						return nil
					},
				}
				s = &Site{FrontmatterIndexQueryer: mockFmIndex}
				w = httptest.NewRecorder()
				router = gin.Default()
				router.GET("/api/find_by", s.handleFindBy)
			})

			When("a valid request is made", func() {
				var response struct {
					Success bool            `json:"success"`
					IDs     []PageReference `json:"ids"`
				}
				BeforeEach(func() {
					req, err := http.NewRequest(http.MethodGet, "/api/find_by?k=test.key&v=test_value", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)

					err = json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns http.StatusOK", func() {
					Expect(w.Code).To(Equal(http.StatusOK))
				})

				It("returns success=true", func() {
					Expect(response.Success).To(BeTrue())
				})

				It("returns the correct page reference", func() {
					expectedIDs := []PageReference{
						{Identifier: "id1", Title: "Title A"},
					}
					Expect(response.IDs).To(Equal(expectedIDs))
				})
			})
		})

		Describe("handleFindByPrefix", func() {
			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				mockFmIndex := &mockFrontmatterIndexQueryer{
					data: map[string]map[string]string{
						"id1": {"title": "Title A"},
						"id2": {"title": "Title B"},
					},
					QueryPrefixMatchFunc: func(key, prefix string) []string {
						if key == "test.key" && prefix == "test_val" {
							return []string{"id1", "id2"}
						}
						return nil
					},
				}
				s = &Site{FrontmatterIndexQueryer: mockFmIndex}
				w = httptest.NewRecorder()
				router = gin.Default()
				router.GET("/api/find_by_prefix", s.handleFindByPrefix)
			})

			When("a valid request is made", func() {
				var response struct {
					Success bool            `json:"success"`
					IDs     []PageReference `json:"ids"`
				}
				BeforeEach(func() {
					req, err := http.NewRequest(http.MethodGet, "/api/find_by_prefix?k=test.key&v=test_val", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)

					err = json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns http.StatusOK", func() {
					Expect(w.Code).To(Equal(http.StatusOK))
				})

				It("returns success=true", func() {
					Expect(response.Success).To(BeTrue())
				})

				It("returns the correct page references", func() {
					expectedIDs := []PageReference{
						{Identifier: "id1", Title: "Title A"},
						{Identifier: "id2", Title: "Title B"},
					}
					Expect(response.IDs).To(Equal(expectedIDs))
				})
			})
		})

		Describe("handleFindByKeyExistence", func() {
			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				mockFmIndex := &mockFrontmatterIndexQueryer{
					data: map[string]map[string]string{
						"id1": {"title": "Title A"},
					},
					QueryKeyExistenceFunc: func(key string) []string {
						if key == "test.key" {
							return []string{"id1"}
						}
						return nil
					},
				}
				s = &Site{FrontmatterIndexQueryer: mockFmIndex}
				w = httptest.NewRecorder()
				router = gin.Default()
				router.GET("/api/find_by_key_existence", s.handleFindByKeyExistence)
			})

			When("a valid request is made", func() {
				var response struct {
					Success bool            `json:"success"`
					IDs     []PageReference `json:"ids"`
				}
				BeforeEach(func() {
					req, err := http.NewRequest(http.MethodGet, "/api/find_by_key_existence?k=test.key", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)

					err = json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns http.StatusOK", func() {
					Expect(w.Code).To(Equal(http.StatusOK))
				})

				It("returns success=true", func() {
					Expect(response.Success).To(BeTrue())
				})

				It("returns the correct page reference", func() {
					expectedIDs := []PageReference{
						{Identifier: "id1", Title: "Title A"},
					}
					Expect(response.IDs).To(Equal(expectedIDs))
				})
			})
		})

		Describe("handleSearch", func() {
			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				s = &Site{}
				w = httptest.NewRecorder()
				router = gin.Default()
				router.GET("/api/search", s.handleSearch)
			})

			When("the BleveIndexQueryer is not initialized", func() {
				BeforeEach(func() {
					s.BleveIndexQueryer = nil // ensure it's nil
					req, err := http.NewRequest(http.MethodGet, "/api/search?q=searchterm", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)
				})

				It("returns http.StatusInternalServerError", func() {
					Expect(w.Code).To(Equal(http.StatusInternalServerError))
				})

				It("returns a helpful error message", func() {
					var response map[string]any
					err := json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
					Expect(response["success"]).To(BeFalse())
					Expect(response["message"]).To(Equal("Search index is not available"))
				})
			})

			When("a valid request is made", func() {
				var response struct {
					Success bool                       `json:"success"`
					Results []bleve.SearchResult `json:"results"`
				}

				BeforeEach(func() {
					mockBleveIndex := &mockBleveIndexQueryer{
						QueryFunc: func(query string) ([]bleve.SearchResult, error) {
							if query == "searchterm" {
								return []bleve.SearchResult{
									{
										Identifier:   wikipage.PageIdentifier("id1"),
										Title:        "Title 1",
										FragmentHTML: "fragment",
									},
								}, nil
							}
							return nil, nil
						},
					}
					s.BleveIndexQueryer = mockBleveIndex
					req, err := http.NewRequest(http.MethodGet, "/api/search?q=searchterm", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)

					err = json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
				})

				It("returns http.StatusOK", func() {
					Expect(w.Code).To(Equal(http.StatusOK))
				})

				It("returns success=true", func() {
					Expect(response.Success).To(BeTrue())
				})

				It("returns the search results", func() {
					Expect(response.Results).To(Equal(
						[]bleve.SearchResult{{Identifier: wikipage.PageIdentifier("id1"), Title: "Title 1", FragmentHTML: "fragment"}},
					))
				})
			})

			When("the query fails", func() {
				BeforeEach(func() {
					mockBleveIndex := &mockBleveIndexQueryer{
						QueryFunc: func(_ string) ([]bleve.SearchResult, error) {
							return nil, errors.New("index is borked")
						},
					}
					s.BleveIndexQueryer = mockBleveIndex
					req, err := http.NewRequest(http.MethodGet, "/api/search?q=searchterm", nil)
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)
				})

				It("returns http.StatusInternalServerError", func() {
					Expect(w.Code).To(Equal(http.StatusInternalServerError))
				})

				It("returns a helpful error message", func() {
					Expect(w.Body.String()).To(ContainSubstring("Problem querying index"))
				})
			})
		})

		Describe("handlePrintLabel", func() {
			BeforeEach(func() {
				gin.SetMode(gin.TestMode)
				s = &Site{}
				w = httptest.NewRecorder()
				router = gin.Default()
				router.POST("/api/print_label", s.handlePrintLabel)
			})

			When("the FrontmatterIndexQueryer is not initialized", func() {
				BeforeEach(func() {
					s.FrontmatterIndexQueryer = nil
					body := strings.NewReader(`{"template_identifier": "t1", "data_identifier": "d1"}`)
					req, err := http.NewRequest(http.MethodPost, "/api/print_label", body)
					req.Header.Set("Content-Type", "application/json")
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)
				})

				It("returns http.StatusInternalServerError", func() {
					Expect(w.Code).To(Equal(http.StatusInternalServerError))
				})

				It("returns a helpful error message", func() {
					var response map[string]any
					err := json.Unmarshal(w.Body.Bytes(), &response)
					Expect(err).NotTo(HaveOccurred())
					Expect(response["success"]).To(BeFalse())
					Expect(response["message"]).To(Equal("Frontmatter index is not available"))
				})
			})

			When("binding the request fails", func() {
				BeforeEach(func() {
					s.FrontmatterIndexQueryer = &mockFrontmatterIndexQueryer{}
					body := strings.NewReader(`not a valid json`)
					req, err := http.NewRequest(http.MethodPost, "/api/print_label", body)
					req.Header.Set("Content-Type", "application/json")
					Expect(err).NotTo(HaveOccurred())
					router.ServeHTTP(w, req)
				})

				It("returns http.StatusBadRequest", func() {
					Expect(w.Code).To(Equal(http.StatusBadRequest))
				})

				It("returns a helpful error message", func() {
					Expect(w.Body.String()).To(ContainSubstring("Problem binding keys"))
				})
			})
		})
	})
})
