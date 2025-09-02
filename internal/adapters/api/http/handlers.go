package http

import (
	"context"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/username/hexarag/internal/adapters/llm/ollama"
	"github.com/username/hexarag/internal/domain/entities"
	"github.com/username/hexarag/internal/domain/ports"
	"github.com/username/hexarag/internal/domain/services"
	"github.com/username/hexarag/internal/domain/metrics"
	"github.com/username/hexarag/internal/adapters/websocket"
)

// APIHandlers contains all HTTP API handlers
type APIHandlers struct {
	storage            ports.StoragePort
	messaging          ports.MessagingPort
	contextConstructor *services.ContextConstructor
	inferenceEngine    *services.InferenceEngine
	modelManager       *services.ModelManager
	metricsCollector   *metrics.Collector
	wsHub              *websocket.Hub
}

// NewAPIHandlers creates a new API handlers instance
func NewAPIHandlers(storage ports.StoragePort, messaging ports.MessagingPort, cc *services.ContextConstructor, ie *services.InferenceEngine, mm *services.ModelManager, mc *metrics.Collector, hub *websocket.Hub) *APIHandlers {
	return &APIHandlers{
		storage:            storage,
		messaging:          messaging,
		contextConstructor: cc,
		inferenceEngine:    ie,
		modelManager:       mm,
		metricsCollector:   mc,
		wsHub:              hub,
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
		
		// Model management
		api.GET("/models", h.listModels)
		api.POST("/models/pull", h.pullModel)
		api.GET("/models/:id", h.getModelInfo)
		api.PUT("/models/current", h.switchModel)
		api.DELETE("/models/:id", h.deleteModel)
		api.GET("/models/status", h.getModelStatus)

		// System metrics and health for developer dashboard
		api.GET("/system/health", h.getSystemHealth)
		api.GET("/system/metrics", h.getSystemMetrics)
		api.GET("/system/connections", h.getSystemConnections)
	}

	// Developer dashboard routes
	dev := r.Group("/dev")
	{
		dev.Static("/static", "./web/dev")
		dev.StaticFile("/", "./web/dev/index.html")
		dev.GET("/events", h.handleDevEvents)
		dev.POST("/scripts/execute", h.executeScript)
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

// Model management handlers

func (h *APIHandlers) listModels(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	models, err := h.modelManager.GetAvailableModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *APIHandlers) pullModel(c *gin.Context) {
	var req struct {
		Model string `json:"model" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second) // 5 minute timeout for model pull
	defer cancel()

	// Start async pull with progress tracking
	go func() {
		progressFn := func(progress ollama.PullProgress) {
			// Broadcast progress via messaging system
			progressMsg := map[string]interface{}{
				"type":        "model_pull_progress",
				"model":       req.Model,
				"status":      progress.Status,
				"completed":   progress.Completed,
				"total":       progress.Total,
				"percent":     0,
				"timestamp":   time.Now(),
			}

			if progress.Total > 0 {
				progressMsg["percent"] = float64(progress.Completed) / float64(progress.Total) * 100
			}

			if err := h.messaging.PublishJSON(context.Background(), "model.pull.progress", progressMsg); err != nil {
				log.Printf("Failed to broadcast pull progress: %v", err)
			}
		}

		if err := h.modelManager.PullModel(ctx, req.Model, progressFn); err != nil {
			// Broadcast error
			errorMsg := map[string]interface{}{
				"type":      "model_pull_error",
				"model":     req.Model,
				"error":     err.Error(),
				"timestamp": time.Now(),
			}
			h.messaging.PublishJSON(context.Background(), "model.pull.error", errorMsg)
		} else {
			// Broadcast completion
			completeMsg := map[string]interface{}{
				"type":      "model_pull_complete",
				"model":     req.Model,
				"timestamp": time.Now(),
			}
			h.messaging.PublishJSON(context.Background(), "model.pull.complete", completeMsg)
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"status":  "pulling", 
		"model":   req.Model,
		"message": "Model download started",
	})
}

func (h *APIHandlers) getModelInfo(c *gin.Context) {
	modelID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	model, err := h.modelManager.GetModelInfo(ctx, modelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Model not found"})
		return
	}

	c.JSON(http.StatusOK, model)
}

func (h *APIHandlers) switchModel(c *gin.Context) {
	var req struct {
		Model          string `json:"model" binding:"required"`
		ConversationID string `json:"conversation_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Validate model exists
	if err := h.modelManager.ValidateModel(ctx, req.Model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Model not available"})
		return
	}

	// If conversation ID provided, update conversation's preferred model
	if req.ConversationID != "" {
		conversation, err := h.storage.GetConversation(ctx, req.ConversationID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Conversation not found"})
			return
		}

		// Set model preference on conversation
		conversation.SetModel(req.Model)
		if err := h.storage.UpdateConversation(ctx, conversation); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Broadcast model switch event
		switchMsg := map[string]interface{}{
			"type":            "model_switched",
			"conversation_id": req.ConversationID,
			"model":           req.Model,
			"timestamp":       time.Now(),
		}
		h.messaging.PublishJSON(ctx, "model.switch", switchMsg)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":   "switched",
		"model":    req.Model,
		"message":  "Model switched successfully",
	})
}

func (h *APIHandlers) deleteModel(c *gin.Context) {
	modelID := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := h.modelManager.DeleteModel(ctx, modelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Broadcast model deletion event
	deleteMsg := map[string]interface{}{
		"type":      "model_deleted",
		"model":     modelID,
		"timestamp": time.Now(),
	}
	h.messaging.PublishJSON(ctx, "model.delete", deleteMsg)

	c.JSON(http.StatusOK, gin.H{
		"status":  "deleted",
		"model":   modelID,
		"message": "Model deleted successfully",
	})
}

func (h *APIHandlers) getModelStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	runningModels, err := h.modelManager.GetRunningModels(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	status := gin.H{
		"running_models": runningModels,
		"timestamp":      time.Now(),
	}

	c.JSON(http.StatusOK, status)
}

// Developer dashboard handlers

func (h *APIHandlers) getSystemHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	health := gin.H{
		"api":      "healthy",
		"ollama":   "unknown",
		"nats":     "unknown", 
		"database": "unknown",
		"uptime":   0,
		"memory_usage": "--",
		"cpu_usage": "--",
		"requests_per_minute": 0,
		"timestamp": time.Now(),
	}

	// Check storage connectivity
	if err := h.storage.Ping(ctx); err != nil {
		health["database"] = "error"
	} else {
		health["database"] = "healthy"
	}

	// Check messaging connectivity
	if err := h.messaging.Ping(); err != nil {
		health["nats"] = "error"
	} else {
		health["nats"] = "healthy"
	}

	// Check Ollama connectivity by trying to list models
	if _, err := h.modelManager.GetAvailableModels(ctx); err != nil {
		health["ollama"] = "error"
	} else {
		health["ollama"] = "healthy"
	}

	c.JSON(http.StatusOK, health)
}

func (h *APIHandlers) getSystemMetrics(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	metrics := gin.H{
		"avg_response_time": 250,
		"p95_response_time": 500,
		"p99_response_time": 800,
		"response_times": []int{200, 180, 250, 300, 220, 190, 280, 260, 240, 210},
		"message_flow": gin.H{
			"sent": 150,
			"received": 145,
			"processing": 5,
		},
		"timestamp": time.Now(),
	}

	// If metrics collector is available, get real metrics
	if h.metricsCollector != nil {
		if realMetrics := h.metricsCollector.GetSystemMetrics(ctx); realMetrics != nil {
			metrics = gin.H(realMetrics)
		}
	}

	c.JSON(http.StatusOK, metrics)
}

func (h *APIHandlers) getSystemConnections(c *gin.Context) {
	stats := h.wsHub.GetStats()
	
	connections := gin.H{
		"websocket": stats["total_connections"],
		"active_conversations": 0, // TODO: Implement conversation tracking
		"details": []gin.H{},
		"rooms": stats["rooms"],
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, connections)
}

func (h *APIHandlers) handleDevEvents(c *gin.Context) {
	// Set room to "dev-dashboard" for developer dashboard connections
	c.Request.URL.RawQuery = "room=dev-dashboard"
	h.wsHub.HandleWebSocket(c)
}

func (h *APIHandlers) executeScript(c *gin.Context) {
	var req struct {
		Script string   `json:"script" binding:"required"`
		Args   []string `json:"args"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate script path for security
	allowedScripts := map[string]string{
		"health-check": "./scripts/health-check.sh",
		"test-chat":    "./scripts/test-chat.sh",
		"pull-model":   "./scripts/pull-model.sh",
		"benchmark":    "./scripts/benchmark.sh",
		"reset-db":     "./scripts/reset-db.sh",
		"dev-setup":    "./scripts/dev-setup.sh",
	}

	scriptPath, exists := allowedScripts[req.Script]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Script not allowed"})
		return
	}

	// Prepare command
	args := append([]string{}, req.Args...)
	cmd := exec.Command(scriptPath, args...)

	// Capture output
	output, err := cmd.CombinedOutput()
	success := err == nil

	result := gin.H{
		"script":    req.Script,
		"output":    string(output),
		"success":   success,
		"timestamp": time.Now(),
	}

	if err != nil {
		result["error"] = err.Error()
	}

	// Broadcast script execution event via WebSocket
	if h.wsHub != nil {
		event := websocket.Event{
			Type: "script_executed",
			Data: map[string]interface{}{
				"script":  req.Script,
				"success": success,
				"output":  string(output),
			},
			Timestamp: time.Now(),
		}
		h.wsHub.BroadcastToRoom("dev-dashboard", event)
	}

	c.JSON(http.StatusOK, result)
}
