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
)

// Event represents a real-time event to be broadcast
type Event struct {
	Type      string                 `json:"type"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// Client represents a WebSocket client connection
type Client struct {
	ID     string
	Conn   *websocket.Conn
	Send   chan Event
	Hub    *Hub
	Room   string // For targeting specific clients (e.g., "dev-dashboard")
}

// Hub manages WebSocket connections and broadcasts
type Hub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan Event
	rooms      map[string]map[*Client]bool
	mu         sync.RWMutex
}

// WebSocketUpgrader configures the WebSocket upgrader
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, you should validate the origin
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan Event),
		rooms:      make(map[string]map[*Client]bool),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			
			// Add to room if specified
			if client.Room != "" {
				if h.rooms[client.Room] == nil {
					h.rooms[client.Room] = make(map[*Client]bool)
				}
				h.rooms[client.Room][client] = true
			}
			h.mu.Unlock()
			
			log.Printf("WebSocket client connected: %s (room: %s)", client.ID, client.Room)
			
			// Send welcome message
			select {
			case client.Send <- Event{
				Type:      "connection_established",
				Data:      map[string]interface{}{"client_id": client.ID},
				Timestamp: time.Now(),
			}:
			default:
				close(client.Send)
				h.unregisterClient(client)
			}

		case client := <-h.unregister:
			h.unregisterClient(client)

		case event := <-h.broadcast:
			h.broadcastEvent(event)

		case <-ctx.Done():
			log.Println("WebSocket hub shutting down")
			return
		}
	}
}

// unregisterClient removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.Send)
		
		// Remove from room
		if client.Room != "" && h.rooms[client.Room] != nil {
			delete(h.rooms[client.Room], client)
			if len(h.rooms[client.Room]) == 0 {
				delete(h.rooms, client.Room)
			}
		}
		
		log.Printf("WebSocket client disconnected: %s", client.ID)
	}
}

// broadcastEvent sends an event to all connected clients
func (h *Hub) broadcastEvent(event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.Send <- event:
		default:
			close(client.Send)
			delete(h.clients, client)
		}
	}
}

// BroadcastToRoom sends an event to clients in a specific room
func (h *Hub) BroadcastToRoom(room string, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[room]; ok {
		for client := range clients {
			select {
			case client.Send <- event:
			default:
				close(client.Send)
				delete(h.clients, client)
				delete(clients, client)
			}
		}
	}
}

// Broadcast sends an event to all connected clients
func (h *Hub) Broadcast(event Event) {
	h.broadcast <- event
}

// GetStats returns connection statistics
func (h *Hub) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	roomStats := make(map[string]int)
	for room, clients := range h.rooms {
		roomStats[room] = len(clients)
	}

	return map[string]interface{}{
		"total_connections": len(h.clients),
		"rooms":            roomStats,
		"timestamp":        time.Now(),
	}
}

// HandleWebSocket handles WebSocket upgrade requests
func (h *Hub) HandleWebSocket(c *gin.Context) {
	room := c.Query("room")
	if room == "" {
		room = "general"
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	clientID := generateClientID()
	client := &Client{
		ID:   clientID,
		Conn: conn,
		Send: make(chan Event, 256),
		Hub:  h,
		Room: room,
	}

	client.Hub.register <- client

	// Start goroutines for handling the connection
	go client.writePump()
	go client.readPump()
}

// readPump handles reading messages from the WebSocket connection
func (c *Client) readPump() {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle incoming messages (ping, subscribe, etc.)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err == nil {
			c.handleMessage(msg)
		}
	}
}

// writePump handles writing messages to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			if err := json.NewEncoder(w).Encode(event); err != nil {
				log.Printf("Failed to encode event: %v", err)
				return
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from clients
func (c *Client) handleMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "ping":
		// Respond with pong
		c.Send <- Event{
			Type:      "pong",
			Data:      map[string]interface{}{"client_id": c.ID},
			Timestamp: time.Now(),
		}

	case "subscribe":
		// Handle subscription to specific event types
		if eventTypes, ok := msg["events"].([]interface{}); ok {
			log.Printf("Client %s subscribed to events: %v", c.ID, eventTypes)
			// Store subscription preferences (implement as needed)
		}
	}
}

// generateClientID creates a unique client identifier
func generateClientID() string {
	return time.Now().Format("20060102150405") + "-" + generateRandomString(6)
}

// generateRandomString creates a random string of specified length
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
	}
	return string(b)
}