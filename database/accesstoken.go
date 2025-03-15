package database

import (
	"crypto/rand"
	"errors"
	"sync"
)

// in-memory access token store for now

type AccessTokenStorage struct {
	mu sync.Mutex
	db map[string]*AccessToken
}

// NewAccessTokenStorage initializes access token issuer
func NewAccessTokenStorage() *AccessTokenStorage {
	return &AccessTokenStorage{
		db: make(map[string]*AccessToken),
	}
}

// AccessToken holds token and userid of a authenticated user
type AccessToken struct {
	Token  string
	UserID string
}

// Generate generates new token
func (s *AccessTokenStorage) Generate(userID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := rand.Text()
	t := &AccessToken{
		Token:  token,
		UserID: userID,
	}
	s.db[token] = t

	return token
}

// Get accesstoken details by tokenid
func (s *AccessTokenStorage) Get(token string) (*AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if at, ok := s.db[token]; ok {
		return at, nil
	}
	return nil, errors.New("token not found")
}
