package websocket

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"github.com/username/hexarag/internal/domain/ports"
)

// Hub manages all WebSocket connections
type Hub struct {
	clients   map[string]*Client // conversation_id -> client
	clientsMu sync.RWMutex
	messaging ports.MessagingPort
	upgrader  websocket.Upgrader
}

// Client represents a WebSocket client connection
type Client struct {
	conn           *websocket.Conn
	conversationID string
	send           chan []byte
	hub            *Hub
}

// Message types for WebSocket communication
const (
	MessageTypeMessage  = "message"
	MessageTypeResponse = "response"
	MessageTypeError    = "error"
	MessageTypeStatus   = "status"
	MessageTypePing     = "ping"
	MessageTypePong     = "pong"
)

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type           string      `json:"type"`
	ConversationID string      `json:"conversation_id,omitempty"`
	MessageID      string      `json:"message_id,omitempty"`
	Content        string      `json:"content,omitempty"`
	Data           interface{} `json:"data,omitempty"`
	Timestamp      time.Time   `json:"timestamp"`
}

// NewHub creates a new WebSocket hub
func NewHub(messaging ports.MessagingPort) *Hub {
	return &Hub{
		clients:   make(map[string]*Client),
		messaging: messaging,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for development - restrict in production
				return true
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

// Start initializes the WebSocket hub and starts listening for messages
func (h *Hub) Start(ctx context.Context) error {
	// Subscribe to inference responses
	err := h.messaging.Subscribe(ctx, ports.SubjectInferenceResponse, h.handleInferenceResponse)
	if err != nil {
		return err
	}

	// Subscribe to system errors
	err = h.messaging.Subscribe(ctx, ports.SubjectSystemError, h.handleSystemError)
	if err != nil {
		return err
	}

	log.Println("WebSocket hub started and listening for events")
	return nil
}

// HandleWebSocket upgrades HTTP connections to WebSocket
func (h *Hub) HandleWebSocket(c *gin.Context) {
	conversationID := c.Query("conversation_id")
	if conversationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conversation_id parameter required"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &Client{
		conn:           conn,
		conversationID: conversationID,
		send:           make(chan []byte, 256),
		hub:            h,
	}

	h.clientsMu.Lock()
	h.clients[conversationID] = client
	h.clientsMu.Unlock()

	log.Printf("WebSocket client connected for conversation: %s", conversationID)

	// Start client goroutines
	go client.writePump()
	go client.readPump()
}

// handleInferenceResponse processes inference responses and broadcasts to relevant clients
func (h *Hub) handleInferenceResponse(ctx context.Context, subject string, data []byte) error {
	var response struct {
		ConversationID  string      `json:"conversation_id"`
		MessageID       string      `json:"message_id"`
		ResponseMessage interface{} `json:"response_message"`
		FinishReason    string      `json:"finish_reason"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		log.Printf("Failed to unmarshal inference response: %v", err)
		return err
	}

	// Send response to relevant client
	h.clientsMu.RLock()
	client, exists := h.clients[response.ConversationID]
	h.clientsMu.RUnlock()

	if exists {
		wsMsg := WebSocketMessage{
			Type:           MessageTypeResponse,
			ConversationID: response.ConversationID,
			MessageID:      response.MessageID,
			Data:           response.ResponseMessage,
			Timestamp:      time.Now(),
		}

		msgBytes, err := json.Marshal(wsMsg)
		if err != nil {
			log.Printf("Failed to marshal WebSocket message: %v", err)
			return err
		}

		select {
		case client.send <- msgBytes:
		default:
			// Client buffer is full, close the connection
			h.removeClient(response.ConversationID)
		}
	}

	return nil
}

// handleSystemError processes system errors and broadcasts to relevant clients
func (h *Hub) handleSystemError(ctx context.Context, subject string, data []byte) error {
	var errorEvent struct {
		ConversationID string `json:"conversation_id"`
		MessageID      string `json:"message_id"`
		Error          string `json:"error"`
	}

	if err := json.Unmarshal(data, &errorEvent); err != nil {
		log.Printf("Failed to unmarshal system error: %v", err)
		return err
	}

	// Send error to relevant client
	h.clientsMu.RLock()
	client, exists := h.clients[errorEvent.ConversationID]
	h.clientsMu.RUnlock()

	if exists {
		wsMsg := WebSocketMessage{
			Type:           MessageTypeError,
			ConversationID: errorEvent.ConversationID,
			MessageID:      errorEvent.MessageID,
			Content:        errorEvent.Error,
			Timestamp:      time.Now(),
		}

		msgBytes, err := json.Marshal(wsMsg)
		if err != nil {
			log.Printf("Failed to marshal error message: %v", err)
			return err
		}

		select {
		case client.send <- msgBytes:
		default:
			h.removeClient(errorEvent.ConversationID)
		}
	}

	return nil
}

// removeClient removes a client from the hub
func (h *Hub) removeClient(conversationID string) {
	h.clientsMu.Lock()
	defer h.clientsMu.Unlock()

	if client, exists := h.clients[conversationID]; exists {
		close(client.send)
		delete(h.clients, conversationID)
		log.Printf("WebSocket client disconnected for conversation: %s", conversationID)
	}
}

// Broadcast sends a message to all connected clients
func (h *Hub) Broadcast(message WebSocketMessage) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal broadcast message: %v", err)
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for conversationID, client := range h.clients {
		select {
		case client.send <- msgBytes:
		default:
			// Client buffer is full, remove it
			go h.removeClient(conversationID)
		}
	}
}

// GetConnectionCount returns the number of active WebSocket connections
func (h *Hub) GetConnectionCount() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// Client methods

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.removeClient(c.conversationID)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (ping, configuration changes, etc.)
		var wsMsg WebSocketMessage
		if err := json.Unmarshal(message, &wsMsg); err != nil {
			log.Printf("Failed to unmarshal WebSocket message: %v", err)
			continue
		}

		switch wsMsg.Type {
		case MessageTypePing:
			// Respond with pong
			pongMsg := WebSocketMessage{
				Type:      MessageTypePong,
				Timestamp: time.Now(),
			}
			c.sendMessage(pongMsg)

		default:
			log.Printf("Unknown WebSocket message type: %s", wsMsg.Type)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current WebSocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// sendMessage sends a message to the client
func (c *Client) sendMessage(message WebSocketMessage) {
	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal client message: %v", err)
		return
	}

	select {
	case c.send <- msgBytes:
	default:
		c.hub.removeClient(c.conversationID)
	}
}
