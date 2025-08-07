// Package server implements the web server and API endpoints for simple_wiki.
package server

import (
	"fmt"
	"net/http"

	"github.com/brendanjerwin/simple_wiki/labels"
	"github.com/gin-gonic/gin"
)

// PageReference represents a reference to a wiki page.
type PageReference struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

// handlePrintLabel handles requests to print a label.
func (s *Site) handlePrintLabel(c *gin.Context) {
	if s.FrontmatterIndexQueryer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Frontmatter index is not available"})
		return
	}

	type QueryJSON struct {
		TemplateIdentifier string `json:"template_identifier" binding:"required"`
		DataIdentifier     string `json:"data_identifier" binding:"required"`
	}

	var json QueryJSON
	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("Problem binding keys: %v", err)})
		return
	}

	err := labels.PrintLabel(json.TemplateIdentifier, json.DataIdentifier, s, s.FrontmatterIndexQueryer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("Failed to print label: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Label printed."})
}

// handleFindBy handles requests to find pages by exact frontmatter match.
func (s *Site) handleFindBy(c *gin.Context) {
	type Req struct {
		DottedKeyPath string `form:"k" binding:"required"`
		Value         string `form:"v" binding:"required"`
	}

	var req Req
	s.executeFrontmatterQuery(c, &req, func() []string {
		return s.FrontmatterIndexQueryer.QueryExactMatch(req.DottedKeyPath, req.Value)
	})
}

// handleFindByPrefix handles requests to find pages by frontmatter prefix match.
func (s *Site) handleFindByPrefix(c *gin.Context) {
	type Req struct {
		DottedKeyPath string `form:"k" binding:"required"`
		ValuePrefix   string `form:"v" binding:"required"`
	}

	var req Req
	s.executeFrontmatterQuery(c, &req, func() []string {
		return s.FrontmatterIndexQueryer.QueryPrefixMatch(req.DottedKeyPath, req.ValuePrefix)
	})
}

// handleFindByKeyExistence handles requests to find pages by frontmatter key existence.
func (s *Site) handleFindByKeyExistence(c *gin.Context) {
	type Req struct {
		DottedKeyPath string `form:"k" binding:"required"`
	}

	var req Req
	s.executeFrontmatterQuery(c, &req, func() []string {
		return s.FrontmatterIndexQueryer.QueryKeyExistence(req.DottedKeyPath)
	})
}

func (s *Site) executeFrontmatterQuery(c *gin.Context, req any, queryExecutor func() []string) {
	if s.FrontmatterIndexQueryer == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Frontmatter index is not available"})
		return
	}

	if err := c.ShouldBindQuery(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("Problem binding keys: %v", err)})
		return
	}

	ids := queryExecutor()
	results := s.createPageReferences(ids)
	c.JSON(http.StatusOK, gin.H{"success": true, "ids": results})
}

func (s *Site) createPageReferences(ids []string) []PageReference {
	results := make([]PageReference, len(ids))
	for idx, id := range ids {
		results[idx] = PageReference{
			Identifier: id,
			Title:      s.FrontmatterIndexQueryer.GetValue(id, "title"),
		}
	}
	return results
}

