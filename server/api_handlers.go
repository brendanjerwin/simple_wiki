package server

import (
	"net/http"

	"github.com/brendanjerwin/simple_wiki/labels"
	"github.com/gin-gonic/gin"
)

type PageReference struct {
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
}

func (s *Site) handlePrintLabel(c *gin.Context) {
	type QueryJSON struct {
		TemplateIdentifier string `json:"template_identifier" binding:"required"`
		DataIdentifier     string `json:"data_identifier" binding:"required"`
	}

	var json QueryJSON
	if err := c.BindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	err := labels.PrintLabel(json.TemplateIdentifier, json.DataIdentifier, s, s.FrontmatterIndexQueryer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to print label: " + err.Error()})
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
	if err := c.BindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	ids := s.FrontmatterIndexQueryer.QueryExactMatch(req.DottedKeyPath, req.Value)
	results := s.createPageReferences(ids)
	c.JSON(http.StatusOK, gin.H{"success": true, "ids": results})
}

func (s *Site) handleFindByPrefix(c *gin.Context) {
	type Req struct {
		DottedKeyPath string `form:"k" binding:"required"`
		ValuePrefix   string `form:"v" binding:"required"`
	}

	var req Req
	if err := c.BindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	ids := s.FrontmatterIndexQueryer.QueryPrefixMatch(req.DottedKeyPath, req.ValuePrefix)
	results := s.createPageReferences(ids)
	c.JSON(http.StatusOK, gin.H{"success": true, "ids": results})
}

func (s *Site) handleFindByKeyExistence(c *gin.Context) {
	type Req struct {
		DottedKeyPath string `form:"k" binding:"required"`
	}

	var req Req
	if err := c.BindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	ids := s.FrontmatterIndexQueryer.QueryKeyExistence(req.DottedKeyPath)
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

func (s *Site) handleSearch(c *gin.Context) {
	type Req struct {
		Query string `form:"q" binding:"required"`
	}

	var req Req
	if err := c.BindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	results, err := s.BleveIndexQueryer.Query(req.Query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem querying index: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "results": results})
}
