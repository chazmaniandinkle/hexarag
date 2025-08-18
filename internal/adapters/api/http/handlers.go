package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/username/hexarag/internal/domain/entities"
	"github.com/username/hexarag/internal/domain/ports"
	"github.com/username/hexarag/internal/domain/services"
)

// APIHandlers contains all HTTP API handlers
type APIHandlers struct {
	storage            ports.StoragePort
	messaging          ports.MessagingPort
	contextConstructor *services.ContextConstructor
	inferenceEngine    *services.InferenceEngine
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(storage ports.StoragePort, messaging ports.MessagingPort, cc *services.ContextConstructor, ie *services.InferenceEngine) *APIHandlers {
	return &APIHandlers{
		storage:            storage,
		messaging:          messaging,
		contextConstructor: cc,
		inferenceEngine:    ie,
	}
}

// SetupRoutes configures all API routes
func (h *APIHandlers) SetupRoutes(r *gin.Engine) {
	// Enable CORS
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Health check
	r.GET("/health", h.handleHealth)

	// API routes
	api := r.Group("/api/v1")
	{
		// Conversations
		api.GET("/conversations", h.listConversations)
		api.POST("/conversations", h.createConversation)
		api.GET("/conversations/:id", h.getConversation)
		api.PUT("/conversations/:id", h.updateConversation)
		api.DELETE("/conversations/:id", h.deleteConversation)

		// Messages
		api.GET("/conversations/:id/messages", h.getMessages)
		api.POST("/conversations/:id/messages", h.sendMessage)

		// System prompts
		api.GET("/system-prompts", h.listSystemPrompts)
		api.POST("/system-prompts", h.createSystemPrompt)
		api.GET("/system-prompts/:id", h.getSystemPrompt)
		api.PUT("/system-prompts/:id", h.updateSystemPrompt)
		api.DELETE("/system-prompts/:id", h.deleteSystemPrompt)

		// Analysis and insights
		api.GET("/conversations/:id/analysis", h.analyzeConversation)
		api.GET("/inference/status", h.getInferenceStatus)
	}

	// Serve static files for the web UI
	r.Static("/static", "./web")
	r.StaticFile("/", "./web/index.html")
}

// Health check endpoint
func (h *APIHandlers) handleHealth(c *gin.Context) {
	status := gin.H{
		"status":    "ok",
		"timestamp": time.Now().Unix(),
		"service":   "hexarag",
	}

	// Check storage connectivity
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := h.storage.Ping(ctx); err != nil {
		status["storage"] = "error"
		status["storage_error"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["storage"] = "ok"

	// Check messaging connectivity
	if err := h.messaging.Ping(); err != nil {
		status["messaging"] = "error"
		status["messaging_error"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, status)
		return
	}
	status["messaging"] = "ok"

	c.JSON(http.StatusOK, status)
}

// Conversation handlers

func (h *APIHandlers) listConversations(c *gin.Context) {
	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conversations, err := h.storage.GetConversations(ctx, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"conversations": conversations,
		"limit":         limit,
		"offset":        offset,
	})
}

func (h *APIHandlers) createConversation(c *gin.Context) {
	var req struct {
		Title          string `json:"title"`
		SystemPromptID string `json:"system_prompt_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use default system prompt if not specified
	if req.SystemPromptID == "" {
		req.SystemPromptID = "default"
	}

	conversation := entities.NewConversation(req.Title, req.SystemPromptID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.storage.SaveConversation(ctx, conversation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, conversation)
}

func (h *APIHandlers) getConversation(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conversation, err := h.storage.GetConversation(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	c.JSON(http.StatusOK, conversation)
}

func (h *APIHandlers) updateConversation(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conversation, err := h.storage.GetConversation(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	var req struct {
		Title          string `json:"title"`
		SystemPromptID string `json:"system_prompt_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != "" {
		conversation.SetTitle(req.Title)
	}
	if req.SystemPromptID != "" {
		conversation.SetSystemPrompt(req.SystemPromptID)
	}

	if err := h.storage.UpdateConversation(ctx, conversation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, conversation)
}

func (h *APIHandlers) deleteConversation(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.storage.DeleteConversation(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Conversation deleted"})
}

// Message handlers

func (h *APIHandlers) getMessages(c *gin.Context) {
	conversationID := c.Param("id")
	limit := 50

	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	messages, err := h.storage.GetMessages(ctx, conversationID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"count":    len(messages),
	})
}

func (h *APIHandlers) sendMessage(c *gin.Context) {
	conversationID := c.Param("id")

	var req struct {
		Content              string `json:"content" binding:"required"`
		UseExtendedKnowledge bool   `json:"use_extended_knowledge"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create user message
	userMessage := entities.NewMessage(conversationID, entities.RoleUser, req.Content)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Save user message
	if err := h.storage.SaveMessage(ctx, userMessage); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update conversation with new message
	conversation, err := h.storage.GetConversation(ctx, conversationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
		return
	}

	conversation.AddMessage(userMessage.ID)
	if err := h.storage.UpdateConversation(ctx, conversation); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Trigger context construction
	contextRequest := &services.ContextRequest{
		ConversationID:       conversationID,
		MessageID:            userMessage.ID,
		UseExtendedKnowledge: req.UseExtendedKnowledge,
	}

	if err := h.messaging.PublishJSON(ctx, ports.SubjectContextRequest, contextRequest); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to trigger context construction"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    userMessage,
		"status":     "processing",
		"message_id": userMessage.ID,
	})
}

// System prompt handlers

func (h *APIHandlers) listSystemPrompts(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompts, err := h.storage.GetSystemPrompts(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"system_prompts": prompts})
}

func (h *APIHandlers) createSystemPrompt(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prompt := entities.NewSystemPrompt(req.Name, req.Content)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.storage.SaveSystemPrompt(ctx, prompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, prompt)
}

func (h *APIHandlers) getSystemPrompt(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt, err := h.storage.GetSystemPrompt(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "System prompt not found"})
		return
	}

	c.JSON(http.StatusOK, prompt)
}

func (h *APIHandlers) updateSystemPrompt(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt, err := h.storage.GetSystemPrompt(ctx, id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "System prompt not found"})
		return
	}

	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Name == "" {
		req.Name = prompt.Name
	}
	if req.Content == "" {
		req.Content = prompt.Content
	}

	prompt.Update(req.Name, req.Content)

	if err := h.storage.UpdateSystemPrompt(ctx, prompt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prompt)
}

func (h *APIHandlers) deleteSystemPrompt(c *gin.Context) {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := h.storage.DeleteSystemPrompt(ctx, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "System prompt deleted"})
}

// Analysis and status handlers

func (h *APIHandlers) analyzeConversation(c *gin.Context) {
	conversationID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	analysis, err := h.contextConstructor.AnalyzeConversation(ctx, conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, analysis)
}

func (h *APIHandlers) getInferenceStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := h.inferenceEngine.GetInferenceStatus(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}
