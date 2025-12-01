package sqlite

import (
	"context"
	"database/sql"

	"github.com/erikbos/jellofin-server/database/model"
)

// GetUser retrieves a user.
func (s *SqliteRepo) GetUser(ctx context.Context, username string) (user *model.User, err error) {
	const query = `SELECT id,
		username,
		password,
		created,
		lastlogin,
		lastused FROM users WHERE username=? LIMIT 1`
	return sqlScanUser(s.dbReadHandle.QueryRowContext(ctx, query, username))
}

// GetByID retrieves a user from the database by their ID.
func (s *SqliteRepo) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	query := `SELECT id,
		username,
		password,
		created,
		lastlogin,
		lastused FROM users WHERE id=? LIMIT 1`
	return sqlScanUser(s.dbReadHandle.QueryRowContext(ctx, query, userID))
}

func sqlScanUser(row *sql.Row) (*model.User, error) {
	var user model.User
	if err := row.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Created,
		&user.LastLogin,
		&user.LastUsed); err != nil {
		return nil, model.ErrNotFound
	}
	return &user, nil
}

// UpsertUser upserts a user into the database.
func (s *SqliteRepo) UpsertUser(ctx context.Context, user *model.User) error {
	tx, _ := s.dbWriteHandle.BeginTxx(ctx, nil)
	defer tx.Rollback()

	const query = `REPLACE INTO users (id, username, password, created, lastlogin, lastused) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query,
		user.ID,
		user.Username,
		user.Password,
		user.Created,
		user.LastLogin,
		user.LastUsed)
	if err != nil {
		// log.Printf("Error upserting user to db for userID: %s: %s\n", user.ID, err)
		return err
	}
	return tx.Commit()
}
