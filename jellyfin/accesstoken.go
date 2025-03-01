package jellyfin

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// in-memory access token store for now

func init() {
	AccessTokens.db = make(map[string]*AccessToken)
}

var AccessTokens AccessTokenRepo

type AccessTokenRepo struct {
	mu sync.Mutex
	db map[string]*AccessToken
}

type AccessToken struct {
	accesstoken string
	session     *JFSessionInfo
}

// New generates new token
func (s *AccessTokenRepo) New(session *JFSessionInfo) string {
	AccessTokens.mu.Lock()
	defer AccessTokens.mu.Unlock()

	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(bytes)
	// Requires Go 1.24
	// token := rand.Text()

	at := &AccessToken{
		accesstoken: token,
		session:     session,
	}
	s.db[token] = at

	return token
}

// Lookup accesstoken details
func (s *AccessTokenRepo) Lookup(id string) *AccessToken {
	AccessTokens.mu.Lock()
	defer AccessTokens.mu.Unlock()

	if at, ok := s.db[id]; ok {
		return at
	}
	return nil
}
