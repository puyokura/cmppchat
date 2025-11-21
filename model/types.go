package model

import "time"

// User represents a chat user.
type User struct {
	Username     string   `json:"username"`
	PasswordHash string   `json:"password_hash"` // Stored as hash
	IPID         string   `json:"ip_id"`         // 15 chars fixed length like "XXX.XXX.XXX.XXX"
	Clans        []string `json:"clans"`         // List of clan tags
	IsAdmin      bool     `json:"is_admin"`      // Persistent admin status
}

// Message represents a chat message.
type Message struct {
	Sender        string    `json:"sender"`         // Username
	SenderDisplay string    `json:"sender_display"` // Username with clan tags/colors
	SenderID      string    `json:"sender_id"`      // IPID
	Content       string    `json:"content"`
	Timestamp     time.Time `json:"timestamp"`
	IsSystem      bool      `json:"is_system"` // True if it's a system message
}

// EventType represents the type of websocket event.
type EventType string

const (
	EventMessage EventType = "message"
	EventLogin   EventType = "login"
	EventError   EventType = "error"
)

// Event is the wrapper for websocket messages.
type Event struct {
	Type    EventType   `json:"type"`
	Payload interface{} `json:"payload"`
}

// LoginPayload is the payload for login/register requests.
type LoginPayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}
