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
	Users    map[string]*model.User // Key: Username
	Messages []model.Message
	mu       sync.RWMutex
	userFile string
	msgFile  string
}

func NewStore(userFile, msgFile string) *Store {
	return &Store{
		Users:    make(map[string]*model.User),
		Messages: make([]model.Message, 0),
		userFile: userFile,
		msgFile:  msgFile,
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

	// Load Messages
	if _, err := os.Stat(s.msgFile); err == nil {
		data, err := os.ReadFile(s.msgFile)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(data, &s.Messages); err != nil {
			return err
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

func (s *Store) SaveMessages() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.Messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.msgFile, data, 0644)
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

func (s *Store) AddMessage(msg model.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Messages = append(s.Messages, msg)

	// Save messages (internal)
	data, err := json.MarshalIndent(s.Messages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.msgFile, data, 0644)
	return os.WriteFile(s.msgFile, data, 0644)
}

func (s *Store) GetMessages() []model.Message {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to be safe
	msgs := make([]model.Message, len(s.Messages))
	copy(msgs, s.Messages)
	return msgs
}
