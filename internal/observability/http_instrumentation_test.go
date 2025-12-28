//revive:disable:dot-imports
package observability_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/brendanjerwin/simple_wiki/internal/observability"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("HTTPInstrumentation", func() {
	Describe("NewHTTPInstrumentation", func() {
		When("creating with metrics", func() {
			var instrumentation *observability.HTTPInstrumentation

			BeforeEach(func() {
				metrics, err := observability.NewHTTPMetrics()
				Expect(err).ToNot(HaveOccurred())
				instrumentation = observability.NewHTTPInstrumentation(metrics, nil)
			})

			It("should return a non-nil instrumentation", func() {
				Expect(instrumentation).ToNot(BeNil())
			})
		})

		When("creating without metrics (nil)", func() {
			var instrumentation *observability.HTTPInstrumentation

			BeforeEach(func() {
				instrumentation = observability.NewHTTPInstrumentation(nil, nil)
			})

			It("should return a non-nil instrumentation", func() {
				Expect(instrumentation).ToNot(BeNil())
			})
		})
	})

	Describe("GinMiddleware", func() {
		var router *gin.Engine
		var instrumentation *observability.HTTPInstrumentation
		var recorder *httptest.ResponseRecorder

		BeforeEach(func() {
			gin.SetMode(gin.TestMode)
			router = gin.New()
			metrics, err := observability.NewHTTPMetrics()
			Expect(err).ToNot(HaveOccurred())
			instrumentation = observability.NewHTTPInstrumentation(metrics, nil)
			router.Use(instrumentation.GinMiddleware())
			recorder = httptest.NewRecorder()
		})

		When("handling a successful request", func() {
			BeforeEach(func() {
				router.GET("/test", func(c *gin.Context) {
					c.String(http.StatusOK, "success")
				})
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should return status OK", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})

			It("should return the response body", func() {
				Expect(recorder.Body.String()).To(Equal("success"))
			})
		})

		When("handling an error request", func() {
			BeforeEach(func() {
				router.GET("/error", func(c *gin.Context) {
					c.String(http.StatusInternalServerError, "error")
				})
				req := httptest.NewRequest(http.MethodGet, "/error", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should return the error status code", func() {
				Expect(recorder.Code).To(Equal(http.StatusInternalServerError))
			})
		})

		When("handling a request without registered route", func() {
			BeforeEach(func() {
				req := httptest.NewRequest(http.MethodGet, "/unregistered", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should return 404", func() {
				Expect(recorder.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("middleware is created without metrics", func() {
			BeforeEach(func() {
				router = gin.New()
				instrumentation = observability.NewHTTPInstrumentation(nil, nil)
				router.Use(instrumentation.GinMiddleware())
				router.GET("/test", func(c *gin.Context) {
					c.String(http.StatusOK, "success")
				})
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				router.ServeHTTP(recorder, req)
			})

			It("should still process requests successfully", func() {
				Expect(recorder.Code).To(Equal(http.StatusOK))
			})
		})
	})
})
