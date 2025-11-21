package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/puyokura/cmppchat/model"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for local tool
	},
}

// Client is a middleman between the websocket connection and the hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// Current user info (if logged in)
	user *model.User

	// Admin status
	isAdmin bool
}

// Hub maintains the set of active clients and broadcasts messages to the clients.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	store      *Store
	config     *Config
	mu         sync.Mutex
}

func NewHub(store *Store, config *Config) *Hub {
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
		store:      store,
		config:     config,
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Handle incoming JSON messages
		var event model.Event
		if err := json.Unmarshal(message, &event); err != nil {
			log.Printf("Invalid JSON: %v", err)
			continue
		}

		c.handleEvent(event)
	}
}

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
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(c.send)
			for i := 0; i < n; i++ {
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

func (c *Client) handleEvent(event model.Event) {
	switch event.Type {
	case model.EventLogin:
		// Handle login/register
		// Payload should be LoginPayload
		payloadBytes, _ := json.Marshal(event.Payload)
		var payload model.LoginPayload
		json.Unmarshal(payloadBytes, &payload)

		// For simplicity, auto-register if not exists, else login
		// Wait, the requirements say /register and /login commands.
		// But here we are at the websocket layer.
		// The client TUI will send commands as messages or specific events?
		// The README says "User commands: /register ... /login ..."
		// So the client sends a message starting with /.
		// But we need to be authenticated to send messages?
		// Or maybe we allow anonymous connection and then login?
		// Let's assume the client sends a "login" event for explicit login.
		// But if we follow the README strictly, the user types /login in the input area.
		// So the client sends a "message" event with content "/login ...".
		// The server parses it.

		// However, for TUI, it might be easier to handle auth before joining chat?
		// The README says: "User commands... /register... /login...".
		// This implies you are already connected to the server (websocket established) and typing commands.
		// So we should handle everything as "message" event, and check if it's a command.

	case model.EventMessage:
		// Payload is just string content or Message struct?
		// Let's assume payload is map for now or we re-marshal.
		// Actually, let's simplify. The client sends a Message struct?
		// No, the client sends text. The server wraps it in Message struct.

		content, ok := event.Payload.(string)
		if !ok {
			return
		}

		// Process command or chat
		c.processMessage(content)
	}
}

func (c *Client) processMessage(content string) {
	// Check for commands
	if len(content) > 0 && content[0] == '/' {
		c.handleCommand(content)
		return
	}

	if c.user == nil {
		c.sendSystemMessage("Please login first using /login <user> <pass> or /register <user> <pass>")
		return
	}

	// Set admin status from user
	c.isAdmin = c.user.IsAdmin

	// Send history if this is the first message (login success)
	// But processMessage is called for every message.
	// We need to send history only once upon login.
	// This logic should be in handleEvent for EventLogin or we need a flag.
	// But we are simplifying auth by checking c.user == nil.
	// Let's check if this is the *first* time we see c.user != nil?
	// No, processMessage is for incoming messages.
	// We need to send history right after successful login/register in commands.go.
	// So let's move history sending to commands.go or add a method here.

	// Format sender with clans
	senderName := c.user.Username
	if len(c.user.Clans) > 0 {
		var tagsBuilder strings.Builder
		tagsBuilder.WriteString("[")
		for _, tag := range c.user.Clans {
			color := c.hub.config.GetClanColor(tag)
			// Use a custom format for the client to parse: <#RRGGBB>Tag</>
			tagsBuilder.WriteString(fmt.Sprintf("<%s>%s</>", color, tag))
		}
		tagsBuilder.WriteString("]")
		senderName = tagsBuilder.String() + senderName
	}

	msg := model.Message{
		Sender:        c.user.Username,
		SenderDisplay: senderName, // senderName contains tags
		SenderID:      c.user.IPID,
		Content:       content,
		Timestamp:     time.Now(),
		IsSystem:      false,
	}

	c.hub.store.AddMessage(msg)

	// Broadcast
	broadcastEvent := model.Event{
		Type:    model.EventMessage,
		Payload: msg,
	}
	bytes, _ := json.Marshal(broadcastEvent)
	c.hub.broadcast <- bytes
}

func (c *Client) sendSystemMessage(text string) {
	msg := model.Message{
		Sender:    "System",
		SenderID:  "0.0.0.0",
		Content:   text,
		Timestamp: time.Now(),
		IsSystem:  true,
	}
	event := model.Event{
		Type:    model.EventMessage,
		Payload: msg,
	}
	bytes, _ := json.Marshal(event)
	c.send <- bytes
}

// serveWs handles websocket requests from the peer.
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	go client.writePump()
	go client.readPump()

	// Send welcome message
	client.sendSystemMessage(hub.config.WelcomeMessage)
}

func (h *Hub) KickUser(ipid string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		if client.user != nil && client.user.IPID == ipid {
			client.sendSystemMessage("You have been kicked by admin.")
			client.conn.Close()
			delete(h.clients, client)
			close(client.send)
			return true
		}
	}
	return false
}

func (h *Hub) BroadcastSystemMessage(msg string) {
	message := model.Message{
		Sender:    "System",
		SenderID:  "0.0.0.0",
		Content:   msg,
		Timestamp: time.Now(),
		IsSystem:  true,
	}
	event := model.Event{
		Type:    model.EventMessage,
		Payload: message,
	}
	bytes, _ := json.Marshal(event)
	h.broadcast <- bytes
}

func (c *Client) SendHistory() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered from panic in SendHistory: %v", r)
			}
		}()

		messages := c.hub.store.GetMessages()
		log.Printf("Sending %d history messages to client", len(messages))

		for i, msg := range messages {
			event := model.Event{
				Type:    model.EventMessage,
				Payload: msg,
			}
			bytes, _ := json.Marshal(event)

			// Non-blocking send to avoid deadlock
			select {
			case c.send <- bytes:
				// Sent successfully
			default:
				// Channel full, skip this message
				log.Printf("Skipped history message %d (channel full)", i)
			}

			// Small delay to avoid flooding client
			if i < len(messages)-1 {
				time.Sleep(5 * time.Millisecond)
			}
		}

		log.Printf("Finished sending history to client")
	}()
}
