package main

import (
	"encoding/json"
	"fmt"
	"log"
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

	// Default port 8999 if not specified
	if !strings.Contains(host, ":") {
		host = host + ":8999"
	}

	u := url.URL{Scheme: "ws", Host: host, Path: "/ws"}
	log.Printf("connecting to %s", u.String())

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
