package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/puyokura/cmppchat/model"
	"golang.org/x/crypto/bcrypt"
)

type Store struct {
	Users    map[string]*model.User     // Key: Username
	Messages map[string][]model.Message // Key: Room
	mu       sync.RWMutex
	userFile string
	msgDir   string
}

func NewStore(userFile, msgDir string) *Store {
	return &Store{
		Users:    make(map[string]*model.User),
		Messages: make(map[string][]model.Message),
		userFile: userFile,
		msgDir:   msgDir,
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load Users
	if _, err := os.Stat(s.userFile); err == nil {
		data, err := os.ReadFile(s.userFile)
		if err != nil {
			return err
		}
		var usersList []*model.User
		if err := json.Unmarshal(data, &usersList); err != nil {
			return err
		}
		for _, u := range usersList {
			s.Users[u.Username] = u
		}
	}

	// Ensure msgDir exists
	if err := os.MkdirAll(s.msgDir, 0755); err != nil {
		return err
	}

	// Load Messages from msgDir
	files, err := os.ReadDir(s.msgDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if !f.IsDir() && len(f.Name()) > 5 && f.Name()[len(f.Name())-5:] == ".json" {
			roomName := f.Name()[:len(f.Name())-5]
			data, err := os.ReadFile(fmt.Sprintf("%s/%s", s.msgDir, f.Name()))
			if err != nil {
				continue // Skip bad files
			}
			var msgs []model.Message
			if err := json.Unmarshal(data, &msgs); err != nil {
				continue
			}
			s.Messages[roomName] = msgs
		}
	}
	return nil
}

func (s *Store) SaveUsers() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var usersList []*model.User
	for _, u := range s.Users {
		usersList = append(usersList, u)
	}

	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.userFile, data, 0644)
}

func (s *Store) RegisterUser(username, password, ipid string) (*model.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.Users[username]; exists {
		return nil, fmt.Errorf("user already exists")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	newUser := model.User{
		Username:     username,
		PasswordHash: string(hash),
		IPID:         ipid,
		Clans:        []string{},
		IsAdmin:      false,
	}

	s.Users[username] = &newUser

	if err := s.saveUsersInternal(); err != nil {
		delete(s.Users, username) // Rollback
		return nil, err
	}

	return &newUser, nil
}

// Helper to save users without locking (must be called with lock held)
func (s *Store) saveUsersInternal() error {
	var usersList []*model.User
	for _, u := range s.Users {
		usersList = append(usersList, u)
	}
	data, err := json.MarshalIndent(usersList, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.userFile, data, 0644)
}

func (s *Store) Authenticate(username, password string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, exists := s.Users[username]
	if !exists {
		return nil, fmt.Errorf("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	return user, nil
}

func (s *Store) SaveRoomMessages(room string) error {
	// Assumes lock is held by caller if needed, but here we use RLock for reading messages?
	// No, this is internal helper usually.
	// Let's make it public but careful about locks.
	// Actually, AddMessage holds lock.
	// We need an internal save.

	msgs := s.Messages[room]
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/%s.json", s.msgDir, room), data, 0644)
}

func (s *Store) AddMessage(msg model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	room := msg.Room
	if room == "" {
		room = "general"
	}

	s.Messages[room] = append(s.Messages[room], msg)

	return s.saveRoomMessagesInternal(room)
}

func (s *Store) saveRoomMessagesInternal(room string) error {
	msgs := s.Messages[room]
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(fmt.Sprintf("%s/%s.json", s.msgDir, room), data, 0644)
}

func (s *Store) GetMessages(room string) []model.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if room == "" {
		room = "general"
	}

	// Return copy to avoid race conditions if caller modifies?
	// Slice is reference.
	// But we usually just read.
	// Let's return the slice directly for now, or copy if needed.
	// Copy is safer.
	src := s.Messages[room]
	dest := make([]model.Message, len(src))
	copy(dest, src)
	return dest
}
