package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/gorilla/websocket"
	"github.com/puyokura/cmppchat/model"
)

type Network struct {
	conn *websocket.Conn
	send chan []byte
}

func NewNetwork() *Network {
	return &Network{
		send: make(chan []byte, 256),
	}
}

func (n *Network) Connect(host string) error {
	if n.conn != nil {
		n.conn.Close()
	}

	// Handle URL schemes and ports
	var u url.URL
	if strings.Contains(host, "://") {
		parsed, err := url.Parse(host)
		if err != nil {
			return err
		}
		u = *parsed
		if u.Scheme == "http" {
			u.Scheme = "ws"
		} else if u.Scheme == "https" {
			u.Scheme = "wss"
		}
	} else {
		// No scheme provided
		u.Scheme = "ws"
		u.Host = host
		// Add default port if not present
		if !strings.Contains(host, ":") {
			u.Host = host + ":8999"
		}
	}

	// Ensure path is /ws
	if u.Path == "" || u.Path == "/" {
		u.Path = "/ws"
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	n.conn = c
	return nil
}

func (n *Network) Disconnect() {
	if n.conn != nil {
		n.conn.Close()
		n.conn = nil
	}
}

func (n *Network) Close() {
	n.conn.Close()
}

// WaitForMessage is a tea.Cmd that waits for the next message from the websocket
func (n *Network) WaitForMessage() tea.Msg {
	if n.conn == nil {
		// Wait a bit to avoid busy loop if disconnected
		// Or return a special "Disconnected" msg?
		// Actually, if we are disconnected, we shouldn't be polling this.
		// But the main loop calls this.
		// Let's return nil which stops the loop? No, tea.Cmd returns Msg.
		// We can return a special DisconnectedMsg or just sleep.
		// For now, let's just return nil (no message).
		return nil
	}

	_, message, err := n.conn.ReadMessage()
	if err != nil {
		// If error, we might be disconnected
		n.Disconnect()
		return errMsg(err)
	}

	var event model.Event
	if err := json.Unmarshal(message, &event); err != nil {
		return errMsg(err)
	}

	return event
}

func (n *Network) SendMessage(content string) tea.Cmd {
	return func() tea.Msg {
		if n.conn == nil {
			return errMsg(fmt.Errorf("not connected"))
		}

		event := model.Event{
			Type:    model.EventMessage,
			Payload: content,
		}

		bytes, err := json.Marshal(event)
		if err != nil {
			return errMsg(err)
		}

		if err := n.conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
			return errMsg(err)
		}
		return nil
	}
}

type errMsg error

func (n *Network) FetchMessages(host string) ([]model.Message, error) {
	// Handle URL schemes
	if !strings.Contains(host, "://") {
		host = "http://" + host
	} else {
		host = strings.Replace(host, "ws://", "http://", 1)
		host = strings.Replace(host, "wss://", "https://", 1)
	}

	// Ensure port if not present (and not standard ports)
	if !strings.Contains(host, ":") && !strings.Contains(host, "http") { // simplistic check
		host = host + ":8999"
	} else if !strings.Contains(host, ":") && strings.Contains(host, "localhost") {
		host = host + ":8999"
	}

	// Construct API URL
	// Remove /ws if present
	host = strings.Replace(host, "/ws", "", 1)
	// Ensure no trailing slash
	host = strings.TrimSuffix(host, "/")

	apiURL := host + "/api/messages"

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", resp.Status)
	}

	var messages []model.Message
	if err := json.NewDecoder(resp.Body).Decode(&messages); err != nil {
		return nil, err
	}

	return messages, nil
}
