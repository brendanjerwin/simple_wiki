package server

import (
	"errors"
	"net/http"

	"github.com/brendanjerwin/simple_wiki/pkg/chatattachment"
	"github.com/gin-gonic/gin"
)

const (
	chatAttachmentMaxUploadBytes = 20 * 1024 * 1024 // 20 MiB
)

// handleChatAttachmentUpload handles POST /api/chat-attachments.
// Accepts a multipart "file" field and stores the attachment transiently.
// Returns JSON: {"id": "...", "url": "...", "filename": "...", "mimeType": "..."}
func (s *Site) handleChatAttachmentUpload(c *gin.Context) {
	if s.ChatAttachmentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat attachments are disabled"})
		return
	}

	if err := c.Request.ParseMultipartForm(chatAttachmentMaxUploadBytes); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form"})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file field is required"})
		return
	}
	defer func() { _ = file.Close() }()

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" || mimeType == "application/octet-stream" {
		// Use the uploaded filename extension as a hint — the browser normally
		// sends the correct MIME type, so this is only a fallback.
		mimeType = "application/octet-stream"
	}

	id, err := s.ChatAttachmentStore.Store(header.Filename, mimeType, file)
	if err != nil {
		s.Logger.Error("[chat-attachment] store error: %v", err)
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	url := "/api/chat-attachments/" + id
	c.JSON(http.StatusCreated, gin.H{
		"id":       id,
		"url":      url,
		"filename": header.Filename,
		"mimeType": mimeType,
	})
}

// handleChatAttachmentServe handles GET /api/chat-attachments/:id.
// Serves the attachment content with the appropriate Content-Type header.
func (s *Site) handleChatAttachmentServe(c *gin.Context) {
	if s.ChatAttachmentStore == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "chat attachments are disabled"})
		return
	}

	id := c.Param("id")
	attachment, err := s.ChatAttachmentStore.Get(id)
	if err != nil {
		if errors.Is(err, chatattachment.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "attachment not found"})
			return
		}
		if errors.Is(err, chatattachment.ErrInvalidID) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid attachment ID"})
			return
		}
		s.Logger.Error("[chat-attachment] get error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve attachment"})
		return
	}

	c.Header("Content-Disposition", `inline; filename="`+attachment.Filename+`"`)
	c.Header("Cache-Control", "private, max-age=86400")
	c.Data(http.StatusOK, attachment.MIMEType, attachment.Data)
}
