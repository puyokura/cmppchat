package main

import (
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
	default:
		c.sendSystemMessage("Unknown command: " + cmd)
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
	if len(args) != 1 {
		c.sendSystemMessage("Usage: /name <new_name>")
		return
	}
	c.sendSystemMessage("Name change is not supported in this version (Username is your ID).")
}

func (c *Client) handleHelp() {
	help := `Available commands:
/register <user> <pass> - Register new account
/login <user> <pass> - Login
/logout - Logout
/help - Show this help
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
	if len(args) < 2 {
		c.sendSystemMessage("Usage: /clan <create|add|remove> ...")
		return
	}

	subCmd := args[0]
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
