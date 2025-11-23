package main

import (
	"encoding/json"
	"os"
	"sync"
)

type Config struct {
	AdminPassword   string            `json:"admin_password"`
	Port            string            `json:"port"`
	Host            string            `json:"host"`
	WelcomeMessage  string            `json:"welcome_message"`
	BannedIPIDs     []string          `json:"banned_ip_ids"`
	Clans           map[string]string `json:"clans"` // Tag -> HexColor
	AdminIPIDSuffix string            `json:"admin_ipid_suffix"`
	ServerName      string            `json:"server_name"`
	Rooms           []string          `json:"rooms"`
	mu              sync.RWMutex
	configFile      string
}

func NewConfig(filename string) *Config {
	if filename == "" {
		filename = "server_config.json"
	}
	return &Config{
		configFile: filename,
		// Defaults
		Port:            "8999",
		Host:            "localhost",
		AdminPassword:   "admin",
		WelcomeMessage:  "Welcome to CMPPChat! Type /help for commands.",
		BannedIPIDs:     []string{},
		Clans:           make(map[string]string),
		AdminIPIDSuffix: "1",
		ServerName:      "CMPPChat Server",
		Rooms:           []string{"general"},
	}
}

func (c *Config) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, err := os.Stat(c.configFile); os.IsNotExist(err) {
		// Create default config if not exists
		return c.saveInternal()
	}

	data, err := os.ReadFile(c.configFile)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, c); err != nil {
		return err
	}

	// Auto-update config file with any missing fields (defaults)
	return c.saveInternal()
}

func (c *Config) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.saveInternal()
}

func (c *Config) saveInternal() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configFile, data, 0644)
}

func (c *Config) IsBanned(ipid string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, banned := range c.BannedIPIDs {
		if banned == ipid {
			return true
		}
	}
	return false
}

func (c *Config) Ban(ipid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already banned
	for _, banned := range c.BannedIPIDs {
		if banned == ipid {
			return nil
		}
	}

	c.BannedIPIDs = append(c.BannedIPIDs, ipid)
	return c.saveInternal()
}

func (c *Config) Unban(ipid string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	newBanned := []string{}
	for _, banned := range c.BannedIPIDs {
		if banned != ipid {
			newBanned = append(newBanned, banned)
		}
	}
	c.BannedIPIDs = newBanned
	return c.saveInternal()
}

func (c *Config) SetClan(tag, color string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Clans == nil {
		c.Clans = make(map[string]string)
	}
	c.Clans[tag] = color
	return c.saveInternal()
}

func (c *Config) GetClanColor(tag string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if color, ok := c.Clans[tag]; ok {
		return color
	}
	return "#FFFFFF" // Default white
}

func (c *Config) AddRoom(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, r := range c.Rooms {
		if r == name {
			return nil // Already exists
		}
	}
	c.Rooms = append(c.Rooms, name)
	return c.saveInternal()
}

func (c *Config) RemoveRoom(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var newRooms []string
	for _, r := range c.Rooms {
		if r != name {
			newRooms = append(newRooms, r)
		}
	}
	c.Rooms = newRooms
	return c.saveInternal()
}

func (c *Config) RoomExists(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, r := range c.Rooms {
		if r == name {
			return true
		}
	}
	return false
}
