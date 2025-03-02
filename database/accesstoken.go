package database

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// in-memory access token store for now

type AccessTokenRepo struct {
	mu sync.Mutex
	db map[string]*AccessToken
}

type AccessToken struct {
	Accesstoken string
	UserId      string
}

// Init initializes access token issuer
func (s *AccessTokenRepo) Init() {
	s.db = make(map[string]*AccessToken)
}

// New generates new token
func (s *AccessTokenRepo) New(userId string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(bytes)
	// Requires Go 1.24
	// token := rand.Text()

	t := &AccessToken{
		UserId:      userId,
		Accesstoken: token,
	}
	s.db[token] = t

	return token
}

// Lookup accesstoken details by tokenid
func (s *AccessTokenRepo) Lookup(token string) *AccessToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	if at, ok := s.db[token]; ok {
		return at
	}
	return nil
}
