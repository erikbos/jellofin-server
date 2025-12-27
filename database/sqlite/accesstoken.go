package sqlite

import (
	"context"
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/erikbos/jellofin-server/database/model"
)

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
	query := `SELECT
		userid,
		token,
		devicename,
		deviceid,
		applicationname,
		applicationversion,
		remoteaddress,
		created,
		lastused FROM accesstokens WHERE token=? LIMIT 1`

	var t model.AccessToken
	row := s.dbReadHandle.QueryRowContext(ctx, query, token)
	err := row.Scan(&t.UserID,
		&t.Token,
		&t.DeviceName,
		&t.DeviceId,
		&t.ApplicationName,
		&t.ApplicationVersion,
		&t.RemoteAddress,
		&t.Created,
		&t.LastUsed)

	if err == nil {
		t.LastUsed = time.Now().UTC()
		// Store accesstoken in memory
		s.accessTokenCache[token] = &t
		return &t, nil
	}
	return nil, model.ErrNotFound
}

// GetAccessTokens returns all access tokens for a user.
func (s *SqliteRepo) GetAccessTokens(ctx context.Context, userID string) ([]model.AccessToken, error) {
	query := `SELECT
		userid,
		token,
		devicename,
		deviceid,
		applicationname,
		applicationversion,
		remoteaddress,
		created,
		lastused FROM accesstokens`
	rows, err := s.dbReadHandle.QueryxContext(ctx, query)
	if err != nil {
		log.Printf("Error retrieving access tokens from db for userID: %s: %s\n", userID, err)
		return nil, err
	}
	defer rows.Close()

	var tokens []model.AccessToken
	for rows.Next() {
		var t model.AccessToken
		if err := rows.StructScan(&t); err != nil {
			log.Printf("Error scanning access token row from db for userID: %s: %s\n", userID, err)
			return nil, err
		}
		if t.UserID == userID {
			tokens = append(tokens, t)
		}
	}
	return tokens, nil
}

// UpsertAccessToken upserts a token.
func (s *SqliteRepo) UpsertAccessToken(ctx context.Context, t model.AccessToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store accesstoken in database
	tx, err := s.dbReadHandle.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.storeAccessToken(ctx, tx, t); err != nil {
		return err
	}

	// Commit transaction before storing in memory to ensure it is persisted
	if tx.Commit() != nil {
		return err
	}
	// Store accesstoken in cache
	s.accessTokenCache[t.Token] = &t

	return nil
}

// DeleteAccessToken deletes an access token from the database and cache.
func (s *SqliteRepo) DeleteAccessToken(ctx context.Context, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	const query = `DELETE FROM accesstokens WHERE token = ?;`
	_, err := s.dbWriteHandle.ExecContext(ctx, query, token)
	if err != nil {
		log.Printf("Error deleting access token from db for token: %s: %s\n", token, err)
		return err
	}

	// Remove from cache
	delete(s.accessTokenCache, token)
	return nil
}

// accessTokenBackgroundJob writes changed accesstokens to database.
func (s *SqliteRepo) accessTokenBackgroundJob(ctx context.Context, interval time.Duration) {
	if s.dbReadHandle == nil || s.dbWriteHandle == nil {
		log.Fatal(model.ErrNoDbHandle)
	}

	for {
		if err := s.writeChangedAccessTokensToDB(ctx); err != nil {
			log.Printf("Error writing access tokens to db: %s\n", err)
		}
		time.Sleep(interval)
	}
}

// writeChangedAccessTokensToDB writes updated access tokens to db to persist last use date.
func (s *SqliteRepo) writeChangedAccessTokensToDB(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.dbWriteHandle.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, value := range s.accessTokenCache {
		if value.LastUsed.After(s.accessTokenCacheSyncTime) {
			if err := s.storeAccessToken(ctx, tx, *value); err != nil {
				return err
			}
		}
	}
	// Update sync time so we only write changed entries next time
	s.accessTokenCacheSyncTime = time.Now().UTC()
	return tx.Commit()
}

// storeAccessToken stores an access token in the database
func (s *SqliteRepo) storeAccessToken(ctx context.Context, tx *sqlx.Tx, t model.AccessToken) error {
	const query = `REPLACE INTO accesstokens (
		userid,
		token,
		deviceid,
		devicename,
		applicationname,
		applicationversion,
		remoteaddress,
		created,
		lastused) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	_, err := tx.ExecContext(ctx, query,
		t.UserID,
		t.Token,
		t.DeviceId,
		t.DeviceName,
		t.ApplicationName,
		t.ApplicationVersion,
		t.RemoteAddress,
		t.Created,
		t.LastUsed)

	// if err != nil {
	// 	log.Printf("Error storing access token to db for userID: %s, token: %s: %s\n", t.UserID, t.Token, err)
	// }
	return err
}
