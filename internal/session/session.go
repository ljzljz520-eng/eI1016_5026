package session

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

type Store struct {
	mu       sync.Mutex
	secret   string
	nextID   uint64
	sessions map[string]string
}

func NewStore(secret string) *Store {
	return &Store{secret: secret, sessions: make(map[string]string)}
}

func (s *Store) Create(username string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%d", s.secret, username, s.nextID)))
	token := hex.EncodeToString(digest[:])
	s.sessions[token] = username
	return token
}

func (s *Store) Username(token string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username, ok := s.sessions[token]
	return username, ok
}

func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}
