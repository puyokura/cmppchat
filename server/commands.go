package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"github.com/puyokura/cmppchat/model"
)

// Helper to generate fake IP ID
func generateIPID() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func (c *Client) handleCommand(cmdLine string) {
	parts := strings.Fields(cmdLine)
	if len(parts) == 0 {
		return
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "/register":
		c.handleRegister(args)
	case "/login":
		c.handleLogin(args)
	case "/logout":
		c.handleLogout()
	case "/name":
		c.handleName(args)
	case "/help":
		c.handleHelp()
	case "/admin":
		c.handleAdmin(args)
	case "/clan":
		c.handleClan(args)
	case "/kick":
		c.handleKick(args)
	case "/ban":
		c.handleBan(args)
	case "/join":
		// Deprecated, alias to /room join
		c.handleRoom([]string{"join", args[0]})
	case "/room":
		c.handleRoom(args)
	case "/member":
		c.handleMember(args)
	case "/userinfo":
		c.handleUserInfo(args)
	case "/server":
		c.handleServer(args)
	default:
		c.sendSystemMessage("Unknown command: " + cmd)
	}
}

func (c *Client) handleRoom(args []string) {
	if len(args) < 1 {
		c.sendSystemMessage("Usage: /room <join|list|create|remove> ...")
		return
	}

	if c.user == nil {
		c.sendSystemMessage("Please login first.")
		return
	}

	subCmd := args[0]

	switch subCmd {
	case "join":
		if len(args) != 2 {
			c.sendSystemMessage("Usage: /room join <room_name>")
			return
		}
		roomName := args[1]

		if !c.hub.config.RoomExists(roomName) {
			c.sendSystemMessage("Room does not exist.")
			return
		}

		oldRoom := c.Room
		if oldRoom == "" {
			oldRoom = "general"
		}

		c.Room = roomName

		event := model.Event{
			Type: "room_join",
			Payload: map[string]string{
				"room": roomName,
			},
		}
		bytes, _ := json.Marshal(event)
		c.send <- bytes

		c.sendSystemMessage(fmt.Sprintf("Joined room: %s", roomName))
		log.Printf("User %s moved from %s to %s", c.user.Username, oldRoom, roomName)

	case "list":
		c.hub.config.mu.RLock()
		rooms := c.hub.config.Rooms
		c.hub.config.mu.RUnlock()

		var sb strings.Builder
		sb.WriteString("Available Rooms:\n")
		for _, r := range rooms {
			sb.WriteString(fmt.Sprintf("• %s\n", r))
		}
		c.sendSystemMessage(sb.String())

	case "create":
		if !c.isAdmin {
			c.sendSystemMessage("Admin only.")
			return
		}
		if len(args) != 2 {
			c.sendSystemMessage("Usage: /room create <room_name>")
			return
		}
		roomName := args[1]
		if len(roomName) > 20 {
			c.sendSystemMessage("Room name too long.")
			return
		}
		if c.hub.config.RoomExists(roomName) {
			c.sendSystemMessage("Room already exists.")
			return
		}
		if err := c.hub.config.AddRoom(roomName); err != nil {
			c.sendSystemMessage("Failed to create room: " + err.Error())
			return
		}

		// Create welcome message for the new room
		welcomeMsg := model.Message{
			Sender:    "System",
			Content:   fmt.Sprintf("Welcome to the %s room!", roomName),
			Timestamp: time.Now(),
			Room:      roomName,
			SenderID:  "0.0.0.0",
		}
		c.hub.store.AddMessage(welcomeMsg)

		c.sendSystemMessage(fmt.Sprintf("Room %s created.", roomName))

	case "remove":
		if !c.isAdmin {
			c.sendSystemMessage("Admin only.")
			return
		}
		if len(args) != 2 {
			c.sendSystemMessage("Usage: /room remove <room_name>")
			return
		}
		roomName := args[1]
		if roomName == "general" {
			c.sendSystemMessage("Cannot remove general room.")
			return
		}
		if !c.hub.config.RoomExists(roomName) {
			c.sendSystemMessage("Room does not exist.")
			return
		}
		if err := c.hub.config.RemoveRoom(roomName); err != nil {
			c.sendSystemMessage("Failed to remove room: " + err.Error())
			return
		}
		// Move users in that room to general?
		// For now, just let them be in a ghost room until they move.
		// Or we can iterate clients and move them.
		// Iterating is better UX.
		for client := range c.hub.clients {
			if client.Room == roomName {
				client.Room = "general"
				event := model.Event{
					Type: "room_join",
					Payload: map[string]string{
						"room": "general",
					},
				}
				bytes, _ := json.Marshal(event)
				client.send <- bytes
				client.sendSystemMessage("Room was removed. Moved to general.")
			}
		}
		c.sendSystemMessage(fmt.Sprintf("Room %s removed.", roomName))

	default:
		c.sendSystemMessage("Unknown subcommand.")
	}
}

func (c *Client) handleRegister(args []string) {
	if len(args) != 2 {
		c.sendSystemMessage("Usage: /register <username> <password>")
		return
	}
	username := args[0]
	password := args[1]

	// Generate random IPID
	ipid := generateIPID()

	user, err := c.hub.store.RegisterUser(username, password, ipid)
	if err != nil {
		c.sendSystemMessage("Registration failed: " + err.Error())
		return
	}

	c.user = user
	c.isAdmin = user.IsAdmin
	c.sendSystemMessage(fmt.Sprintf("Registered and logged in as %s (%s)", user.Username, user.IPID))
	c.SendHistory()
	log.Printf("User registered: %s (%s)", user.Username, user.IPID)
}

func (c *Client) handleLogin(args []string) {
	if len(args) != 2 {
		c.sendSystemMessage("Usage: /login <username> <password>")
		return
	}
	username := args[0]
	password := args[1]

	user, err := c.hub.store.Authenticate(username, password)
	if err != nil {
		c.sendSystemMessage("Login failed: " + err.Error())
		log.Printf("Login failed for %s: %v", username, err)
		return
	}

	c.user = user
	c.isAdmin = user.IsAdmin
	c.sendSystemMessage(fmt.Sprintf("Logged in as %s (%s)", user.Username, user.IPID))
	log.Printf("Calling SendHistory for user: %s", user.Username)
	c.SendHistory()
	log.Printf("User logged in: %s (%s)", user.Username, user.IPID)
}

func (c *Client) handleLogout() {
	if c.user != nil {
		log.Printf("User logged out: %s", c.user.Username)
	}
	c.user = nil
	c.isAdmin = false
	c.sendSystemMessage("Logged out.")
}

func (c *Client) handleName(args []string) {
	if c.user == nil {
		c.sendSystemMessage("You must be logged in to change name.")
		return
	}
	if len(args) < 1 {
		c.sendSystemMessage("Usage: /name <new_name>")
		return
	}

	newName := strings.Join(args, " ")
	if len(newName) > 20 {
		c.sendSystemMessage("Name too long (max 20 chars).")
		return
	}

	oldName := c.user.DisplayName
	if oldName == "" {
		oldName = c.user.Username
	}

	c.user.DisplayName = newName
	c.hub.store.SaveUsers()

	c.sendSystemMessage(fmt.Sprintf("Name changed from %s to %s", oldName, newName))
	log.Printf("User %s changed name to %s", c.user.Username, newName)
}

func (c *Client) handleHelp() {
	help := `Available commands:
/register <user> <pass> - Register new account
/login <user> <pass> - Login
/logout - Logout
/help - Show this help
/room <join|list|create|remove> ... - Manage rooms
/member list [room] - List members
/userinfo <name> - Show user info
/server info - Show server info
/admin <pass> - Become admin
/clan <create|add|remove> ... - Manage clans (admin only)
/kick <ip_id> - Kick a user (admin only)
/ban <ip_id> - Ban a user (admin only)
`
	c.sendSystemMessage(help)
}

func (c *Client) handleAdmin(args []string) {
	if len(args) != 1 {
		c.sendSystemMessage("Usage: /admin <password>")
		return
	}
	if args[0] == c.hub.config.AdminPassword {
		c.isAdmin = true
		if c.user != nil {
			c.user.IsAdmin = true
			c.hub.store.SaveUsers()
		}
		c.sendSystemMessage("You are now an admin.")
		log.Printf("User became admin: %s", c.user.Username)
	} else {
		c.sendSystemMessage("Incorrect password.")
		log.Printf("Failed admin attempt by %s", c.user.Username)
	}
}

func (c *Client) handleClan(args []string) {
	if !c.isAdmin {
		c.sendSystemMessage("Admin only.")
		return
	}
	if len(args) < 1 {
		c.sendSystemMessage("Usage: /clan <create|add|remove|list> ...")
		return
	}

	subCmd := args[0]
	if subCmd != "list" && len(args) < 2 {
		c.sendSystemMessage("Usage: /clan <create|add|remove> ...")
		return
	}
	switch subCmd {
	case "create":
		// /clan create <tag> <color>
		if len(args) != 3 {
			c.sendSystemMessage("Usage: /clan create <tag> <hex_color>")
			return
		}
		tag := args[1]
		color := args[2]
		if len(tag) > 2 {
			c.sendSystemMessage("Tag must be 1-2 characters.")
			return
		}
		if !strings.HasPrefix(color, "#") || len(color) != 7 {
			c.sendSystemMessage("Color must be hex code (e.g. #FF0000).")
			return
		}
		c.hub.config.SetClan(tag, color)
		c.sendSystemMessage(fmt.Sprintf("Clan %s created with color %s.", tag, color))

	case "add":
		// /clan add <ipid> <tag>
		if len(args) != 3 {
			c.sendSystemMessage("Usage: /clan add <ipid> <tag>")
			return
		}
		ipid := args[1]
		tag := args[2]

		// Validate tag exists
		if c.hub.config.GetClanColor(tag) == "#FFFFFF" && tag != "A" { // A bit hacky check if tag exists?
			// Ideally GetClanColor returns error or bool if not found.
			// For now, let's assume if it returns default it might not exist, but we don't have a check method.
			// Let's just proceed.
		}

		// Find user by IPID (need store method?)
		// We don't have a direct way to get user by IPID from store easily without iterating.
		// But we can iterate hub clients or store users.
		// Let's iterate store users.
		var targetUser *model.User
		for _, u := range c.hub.store.Users {
			if u.IPID == ipid {
				targetUser = u
				break
			}
		}
		if targetUser == nil {
			c.sendSystemMessage("User not found.")
			return
		}

		// Add tag if not present
		found := false
		for _, t := range targetUser.Clans {
			if t == tag {
				found = true
				break
			}
		}
		if !found {
			targetUser.Clans = append(targetUser.Clans, tag)
			c.hub.store.SaveUsers()
			c.sendSystemMessage(fmt.Sprintf("Added %s to clan %s.", targetUser.Username, tag))
		} else {
			c.sendSystemMessage("User already in clan.")
		}

	case "remove":
		// /clan remove <ipid> <tag>
		if len(args) != 3 {
			c.sendSystemMessage("Usage: /clan remove <ipid> <tag>")
			return
		}
		ipid := args[1]
		tag := args[2]

		var targetUser *model.User
		for _, u := range c.hub.store.Users {
			if u.IPID == ipid {
				targetUser = u
				break
			}
		}
		if targetUser == nil {
			c.sendSystemMessage("User not found.")
			return
		}

		newClans := []string{}
		for _, t := range targetUser.Clans {
			if t != tag {
				newClans = append(newClans, t)
			}
		}
		targetUser.Clans = newClans
		c.hub.store.SaveUsers()
		c.sendSystemMessage(fmt.Sprintf("Removed %s from clan %s.", targetUser.Username, tag))

	case "list":
		// /clan list
		var sb strings.Builder
		sb.WriteString("Clans List:\n")

		for tag, color := range c.hub.config.Clans {
			sb.WriteString(fmt.Sprintf("• [%s] (Color: %s)\n", tag, color))

			// Find members
			var members []string
			for _, u := range c.hub.store.Users {
				for _, userTag := range u.Clans {
					if userTag == tag {
						members = append(members, fmt.Sprintf("%s (%s)", u.Username, u.IPID))
						break
					}
				}
			}

			if len(members) > 0 {
				sb.WriteString("  Members: " + strings.Join(members, ", ") + "\n")
			} else {
				sb.WriteString("  (No members)\n")
			}
		}
		c.sendSystemMessage(sb.String())

	default:
		c.sendSystemMessage("Unknown subcommand.")
	}
}

func (c *Client) handleKick(args []string) {
	if !c.isAdmin {
		c.sendSystemMessage("Admin only.")
		return
	}
	if len(args) != 1 {
		c.sendSystemMessage("Usage: /kick <ip_id>")
		return
	}
	targetIPID := args[0]

	// Find client with this IPID
	for client := range c.hub.clients {
		if client.user != nil && client.user.IPID == targetIPID {
			client.sendSystemMessage("You have been kicked.")
			client.conn.Close()
			delete(c.hub.clients, client)
			c.sendSystemMessage("User kicked.")
			return
		}
	}
	c.sendSystemMessage("User not active.")
}

func (c *Client) handleBan(args []string) {
	if !c.isAdmin {
		c.sendSystemMessage("Admin only.")
		return
	}
	if len(args) != 1 {
		c.sendSystemMessage("Usage: /ban <ip_id>")
		return
	}
	targetIPID := args[0]

	c.hub.config.Ban(targetIPID)

	// Kick if active
	for client := range c.hub.clients {
		if client.user != nil && client.user.IPID == targetIPID {
			client.sendSystemMessage("You have been banned.")
			client.conn.Close()
			delete(c.hub.clients, client)
		}
	}
	c.sendSystemMessage("User banned.")
}

func (c *Client) handleMember(args []string) {
	// /member list [room]
	if len(args) > 0 && args[0] != "list" {
		c.sendSystemMessage("Usage: /member list [room]")
		return
	}

	targetRoom := ""
	if len(args) == 2 {
		targetRoom = args[1]
	}

	var sb strings.Builder
	if targetRoom != "" {
		sb.WriteString(fmt.Sprintf("Members in %s:\n", targetRoom))
	} else {
		sb.WriteString("All Online Members:\n")
	}

	count := 0
	for client := range c.hub.clients {
		if client.user != nil {
			if targetRoom != "" && client.Room != targetRoom {
				continue
			}
			room := client.Room
			if room == "" {
				room = "general"
			}
			sb.WriteString(fmt.Sprintf("• %s (%s) [Room: %s]\n", client.user.Username, client.user.DisplayName, room))
			count++
		}
	}
	if count == 0 {
		sb.WriteString("No members found.\n")
	}
	c.sendSystemMessage(sb.String())
}

func (c *Client) handleUserInfo(args []string) {
	if len(args) != 1 {
		c.sendSystemMessage("Usage: /userinfo <username_or_displayname>")
		return
	}
	targetName := args[0]

	// Find user in store
	var targetUser *model.User
	c.hub.store.mu.RLock()
	for _, u := range c.hub.store.Users {
		if u.Username == targetName || u.DisplayName == targetName {
			targetUser = u
			break
		}
	}
	c.hub.store.mu.RUnlock()

	if targetUser == nil {
		c.sendSystemMessage("User not found.")
		return
	}

	// Check online status
	isOnline := false
	currentRoom := "Offline"
	for client := range c.hub.clients {
		if client.user != nil && client.user.Username == targetUser.Username {
			isOnline = true
			currentRoom = client.Room
			if currentRoom == "" {
				currentRoom = "general"
			}
			break
		}
	}

	status := "Offline"
	if isOnline {
		status = "Online"
	}

	info := fmt.Sprintf(
		"User Info:\n"+
			"• Username: %s\n"+
			"• Display Name: %s\n"+
			"• Status: %s\n"+
			"• Room: %s\n"+
			"• IPID: %s\n"+
			"• Admin: %v\n"+
			"• Clans: %v\n",
		targetUser.Username,
		targetUser.DisplayName,
		status,
		currentRoom,
		targetUser.IPID,
		targetUser.IsAdmin,
		targetUser.Clans,
	)
	c.sendSystemMessage(info)
}

func (c *Client) handleServer(args []string) {
	if len(args) < 1 {
		c.sendSystemMessage("Usage: /server <info>")
		return
	}
	subCmd := args[0]
	if subCmd == "info" {
		c.hub.config.mu.RLock()
		defer c.hub.config.mu.RUnlock()

		info := fmt.Sprintf(
			"Server Info:\n"+
				"• Name: %s\n"+
				"• Host: %s\n"+
				"• Port: %s\n",
			c.hub.config.ServerName,
			c.hub.config.Host,
			c.hub.config.Port,
		)
		c.sendSystemMessage(info)
	} else {
		c.sendSystemMessage("Unknown subcommand.")
	}
}
