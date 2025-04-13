package database

import (
	"crypto/rand"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// in-memory access token store,
// access tokens are stored in memory and written to the database every 3 seconds.

type AccessTokenStorage struct {
	dbHandle         *sqlx.DB
	lastDBSyncTime   time.Time
	mu               sync.Mutex
	accessTokenCache map[string]*AccessToken
}

var (
	ErrAccessTokenNotFound = errors.New("access token not found")
)

// NewAccessTokenStorage initializes access token issuer
func NewAccessTokenStorage(d *sqlx.DB) *AccessTokenStorage {
	return &AccessTokenStorage{
		dbHandle:         d,
		accessTokenCache: make(map[string]*AccessToken),
	}
}

// AccessToken holds token and userid of a authenticated user
type AccessToken struct {
	// Token is the access token
	Token string
	// UserID of the user
	UserID string
	// LastUsed of last use
	LastUsed time.Time
}

// Generate generates new token
func (s *AccessTokenStorage) Generate(userID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := rand.Text()
	t := &AccessToken{
		Token:    token,
		UserID:   userID,
		LastUsed: time.Now().UTC(),
	}
	// Store accesstoken in database
	s.storeToken(*t)
	// Store accesstoken in memory
	s.accessTokenCache[token] = t

	return token
}

// Get accesstoken details by tokenid
func (s *AccessTokenStorage) Get(token string) (*AccessToken, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try our in-memory store first
	if at, ok := s.accessTokenCache[token]; ok {
		// Update token timestamp so we can keep track of in-use tokens
		at.LastUsed = time.Now().UTC()
		s.accessTokenCache[token] = at
		return at, nil
	}

	// try database
	var t AccessToken
	sqlerr := s.dbHandle.Get(&t, "SELECT * FROM accesstokens WHERE token=? LIMIT 1", token)
	if sqlerr == nil {
		t.LastUsed = time.Now().UTC()
		// Store accesstoken in memory
		s.accessTokenCache[token] = &t
		return &t, nil
	}

	return nil, ErrAccessTokenNotFound
}

// BackgroundJobs writes changed accesstokens to database.
func (s *AccessTokenStorage) BackgroundJobs() {
	if s.dbHandle == nil {
		log.Fatal(ErrNoDbHandle)
	}

	s.lastDBSyncTime = time.Now().UTC()
	for {
		if err := s.writeChangedAccessTokensToDB(); err != nil {
			log.Printf("Error writing access tokens to db: %s\n", err)
		}
		time.Sleep(60 * time.Second)
	}
}

// writeChangedAccessTokensToDB writes updated access tokens to db to persist last use date.
func (s *AccessTokenStorage) writeChangedAccessTokensToDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, value := range s.accessTokenCache {
		if value.LastUsed.After(s.lastDBSyncTime) {
			if err := s.storeToken(*value); err != nil {
				return err
			}
		}
	}
	s.lastDBSyncTime = time.Now().UTC()
	return nil
}

// storeToken stores an access token in the database
func (s *AccessTokenStorage) storeToken(t AccessToken) error {
	if s.dbHandle == nil {
		return ErrNoDbHandle
	}
	tx, err := s.dbHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.NamedExec(`INSERT OR REPLACE INTO accesstokens (userid, token, lastused)
		VALUES (:userid, :token, :lastused)`, t)
	if err != nil {
		return err

	}
	return tx.Commit()
}
