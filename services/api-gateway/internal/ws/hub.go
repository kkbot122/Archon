// services/api-gateway/internal/ws/hub.go
package ws

import (
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all for local dev
	},
}

// NEW: A wrapper to hold both the payload and the target project
type Message struct {
	ProjectID string
	Payload   []byte
}

type Client struct {
	Hub       *Hub
	Conn      *websocket.Conn
	Send      chan []byte
	ProjectID string // NEW: Which project is this tab looking at?
}

type Hub struct {
	Clients    map[*Client]bool
	Broadcast  chan *Message // CHANGED: Now expects a targeted Message
	Register   chan *Client
	Unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan *Message),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

// Run listens for new connections, disconnections, and messages to broadcast
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients[client] = true
			log.Printf("🔌 Client connected to Project: %s\n", client.ProjectID)

		case client := <-h.Unregister:
			if _, ok := h.Clients[client]; ok {
				delete(h.Clients, client)
				close(client.Send)
				log.Printf("🔌 Client disconnected from Project: %s\n", client.ProjectID)
			}

		case message := <-h.Broadcast:
			// NEW: Only send to clients that are viewing THIS specific project!
			for client := range h.Clients {
				if client.ProjectID == message.ProjectID {
					select {
					case client.Send <- message.Payload:
					default:
						close(client.Send)
						delete(h.Clients, client)
					}
				}
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		// When the loop breaks (client disconnects), unregister and close
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	// We must continuously read to process control frames (close/ping/pong).
	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			// Client disconnected (e.g., closed tab, network drop)
			break
		}
		// Note: If the frontend ever needs to send raw WS messages to the server 
		// (instead of GraphQL), we would process them here. For now, we discard.
	}
}

// writePump pumps messages from the hub to the websocket connection
func (c *Client) writePump() {
	defer c.Conn.Close()
	for message := range c.Send {
		err := c.Conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return
		}
	}
}

// ServeWS handles WebSocket requests from the frontend
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	// NEW: Extract project ID from the connection URL (e.g., ws://localhost:4000/ws?projectId=123)
	projectID := r.URL.Query().Get("projectId")
	if projectID == "" {
		log.Println("⚠️ WebSocket connection rejected: Missing projectId")
		http.Error(w, "Missing projectId", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket Upgrade Error:", err)
		return
	}

	// Assign the ProjectID to the client
	client := &Client{Hub: hub, Conn: conn, Send: make(chan []byte, 256), ProjectID: projectID}
	client.Hub.Register <- client

	go client.writePump()
	go client.readPump()
}