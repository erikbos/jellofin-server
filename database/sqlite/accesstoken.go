package sqlite

import (
	"context"
	"crypto/rand"
	"log"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
)

// CreateAccessToken creates new token.
func (s *SqliteRepo) CreateAccessToken(ctx context.Context, userID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := rand.Text()
	t := &model.AccessToken{
		Token:    token,
		UserID:   userID,
		LastUsed: time.Now().UTC(),
	}
	// Store accesstoken in database
	if err := s.storeToken(*t); err != nil {
		return "", err
	}

	// Store accesstoken in memory
	s.accessTokenCache[token] = t

	return token, nil
}

// GetAccessToken returns accesstoken details based upon tokenid.
func (s *SqliteRepo) GetAccessToken(ctx context.Context, token string) (*model.AccessToken, error) {
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
	var t model.AccessToken
	sqlerr := s.dbHandle.Get(&t, "SELECT userid, token, lastused FROM accesstokens WHERE token=? LIMIT 1", token)
	if sqlerr == nil {
		t.LastUsed = time.Now().UTC()
		// Store accesstoken in memory
		s.accessTokenCache[token] = &t
		return &t, nil
	}

	return nil, model.ErrNotFound
}

// BackgroundJobs writes changed accesstokens to database.
func (s *SqliteRepo) AccessTokenBackgroundJobs() {
	if s.dbHandle == nil {
		log.Fatal(model.ErrNoDbHandle)
	}

	s.accessTokenCacheSyncTime = time.Now().UTC()
	for {
		if err := s.writeChangedAccessTokensToDB(); err != nil {
			log.Printf("Error writing access tokens to db: %s\n", err)
		}
		time.Sleep(60 * time.Second)
	}
}

// writeChangedAccessTokensToDB writes updated access tokens to db to persist last use date.
func (s *SqliteRepo) writeChangedAccessTokensToDB() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, value := range s.accessTokenCache {
		if value.LastUsed.After(s.accessTokenCacheSyncTime) {
			if err := s.storeToken(*value); err != nil {
				return err
			}
		}
	}
	s.accessTokenCacheSyncTime = time.Now().UTC()
	return nil
}

// storeToken stores an access token in the database
func (s *SqliteRepo) storeToken(t model.AccessToken) error {
	if s.dbHandle == nil {
		return model.ErrNoDbHandle
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
