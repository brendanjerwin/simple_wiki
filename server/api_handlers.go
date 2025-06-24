// Package server implements the web server and API endpoints for simple_wiki.
package server

import (
	"fmt"
	"net/http"

	"github.com/brendanjerwin/simple_wiki/labels"
	"github.com/gin-gonic/gin"
)

type PageReference struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

func (s *Site) handlePrintLabel(c *gin.Context) {
	s.requireFrontmatterIndex()

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
	s.requireFrontmatterIndex()

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

func (s *Site) require(component any, message string) {
	if component == nil {
		panic(message)
	}
}

func (s *Site) requireFrontmatterIndex() {
	s.require(s.FrontmatterIndexQueryer, "Frontmatter index is not available")
}

func (s *Site) requireBleveIndex() {
	s.require(s.BleveIndexQueryer, "Search index is not available")
}

func (s *Site) handleSearch(c *gin.Context) {
	s.requireBleveIndex()

	type Req struct {
		Query string `form:"q" binding:"required"`
	}

	var req Req
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("Problem binding keys: %v", err)})
		return
	}

	results, err := s.BleveIndexQueryer.Query(req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": fmt.Sprintf("Problem querying index: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "results": results})
}
