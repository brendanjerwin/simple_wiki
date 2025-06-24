package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jcelliott/lumber"
)

// NewRouter creates and configures a Gin router.
// This function was created by refactoring the old `server.Serve` function's presumed responsibilities.
// You will need to add your existing HTTP handlers (for reading pages, etc.) here.
func NewRouter(
	pathToData string,
	css string,
	defaultPage string,
	lock string,
	debounce int,
	cookieSecret string,
	accessCode string,
	allowFileuploads bool,
	maxUploadMb uint,
	maxDocumentLength uint,
	log *lumber.ConsoleLogger,
) *gin.Engine {
	// gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	// Serve static files (like JS, CSS)
	router.Static("/static", "./static")

	// Placeholder for the root handler
	router.GET("/", func(c *gin.Context) {
		// This should redirect to your default page or another landing page.
		c.Redirect(http.StatusFound, "/"+defaultPage)
	})

	// TODO: Add your existing simple_wiki routes here.
	// Based on the supplied files, you likely have handlers for:
	// - router.POST("/update", ...)
	// - router.POST("/lock", ...)
	// - router.GET("/:page", ...)
	// You will need to re-implement them here, possibly in their own handler files.

	return router
}
