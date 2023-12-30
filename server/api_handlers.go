package server

import (
	"fmt"
	"net/http"

	"github.com/brendanjerwin/simple_wiki/labels"
	llmEditor "github.com/brendanjerwin/simple_wiki/llm/editor"
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

func (s *Site) handleContinueLlmEdit(c *gin.Context) {
	type Req struct {
		InteractionID llmEditor.InteractionID `json:"interaction_id" binding:"required"`
		Answer        string                  `json:"answer" binding:"required"`
	}
	type Resp struct {
		InteractionID    llmEditor.InteractionID `json:"interaction_id"`
		OpenQuestions    []string                `json:"open_questions"`
		NewContent       string                  `json:"new_content"`
		SummaryOfChanges string                  `json:"summary_of_changes"`
	}

	var req Req
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}
	interaction, err := llmEditor.RestoreInteractionFromRAM(req.InteractionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem restoring interaction: " + err.Error()})
		return
	}

	interaction, err = interaction.Respond(req.Answer)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem responding to edit: " + err.Error()})
		return
	}

	resp := Resp{
		InteractionID: interaction.InteractionID,
	}

	resp.OpenQuestions = interaction.LastResponse.Memory.OpenQuestions

	resp.NewContent = interaction.LastResponse.NewContent
	resp.SummaryOfChanges = interaction.LastResponse.ResponseToUser

	c.JSON(http.StatusOK, gin.H{"success": true, "response": resp})
}

func (s *Site) handleStartLlmEdit(c *gin.Context) {
	type Req struct {
		PageIdentifier string `json:"page_identifier" binding:"required"`
		EditPrompt     string `json:"edit_prompt" binding:"required"`
	}
	type Resp struct {
		InteractionID    llmEditor.InteractionID `json:"interaction_id"`
		OpenQuestions    []string                `json:"open_questions"`
		NewContent       string                  `json:"new_content"`
		SummaryOfChanges string                  `json:"summary_of_changes"`
	}

	var req Req
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	interaction, err := s.OpenAIEditor.PerformEdit(req.PageIdentifier, req.EditPrompt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem performing edit: " + err.Error()})
		return
	}

	resp := Resp{
		InteractionID: interaction.InteractionID,
	}

	resp.OpenQuestions = interaction.LastResponse.Memory.OpenQuestions
	resp.NewContent = interaction.LastResponse.NewContent
	resp.SummaryOfChanges = interaction.LastResponse.ResponseToUser

	c.JSON(http.StatusOK, gin.H{"success": true, "response": resp})
}

func (s *Site) handleSaveLlmEdit(c *gin.Context) {
	type Req struct {
		InteractionID  llmEditor.InteractionID `json:"interaction_id" binding:"required"`
		PageIdentifier string                  `json:"page_identifier" binding:"required"`
	}
	type Resp struct {
		Success bool `json:"success"`
	}

	var req Req
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem binding keys: " + err.Error()})
		return
	}

	interaction, err := llmEditor.RestoreInteractionFromRAM(req.InteractionID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Problem restoring interaction: " + err.Error()})
		return
	}

	if interaction.PageIdentifier != req.PageIdentifier {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Interaction is not for this page."})
		return
	}

	identifier, frontmatter, err := s.ReadFrontMatter(req.PageIdentifier)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem reading frontmatter: " + err.Error()})
	}

	frontmatter["llm_memory"] = interaction.LastResponse.Memory.ToFrontmatterMap()
	tomlString, err := frontmatter.Marshal()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem converting frontmatter to toml: " + err.Error()})
	}

	newPageContent := fmt.Sprintf("+++\n%s\n+++\n\n%s", string(tomlString), interaction.LastResponse.NewContent)

	err = s.updatePageContent(identifier, newPageContent, interaction.PageAccessedTimestamp.Unix())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Problem saving edit: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "response": Resp{Success: true}})
}
