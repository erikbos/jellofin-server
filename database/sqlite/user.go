package sqlite

import (
	"context"

	"github.com/erikbos/jellofin-server/database/model"
)

// GetUser retrieves a user.
func (s *SqliteRepo) GetUser(ctx context.Context, username string) (user *model.User, err error) {
	var data model.User
	sqlerr := s.dbHandle.Get(&data, "SELECT id, username, password FROM users WHERE username=? LIMIT 1", username)
	if sqlerr != nil {
		return nil, model.ErrNotFound
	}
	return &data, nil
}

// GetByID retrieves a user from the database by their ID.
func (s *SqliteRepo) GetUserByID(ctx context.Context, userID string) (user *model.User, err error) {
	var data model.User
	if err := s.dbHandle.Get(&data, "SELECT id, username, password FROM users WHERE id=? LIMIT 1", userID); err != nil {
		return nil, model.ErrNotFound
	}
	// No need to return hashed pw
	data.Password = ""
	return &data, nil
}

// UpsertUser upserts a user into the database.
func (s *SqliteRepo) UpsertUser(ctx context.Context, user *model.User) error {
	tx, _ := s.dbHandle.Beginx()
	defer tx.Rollback()

	_, err := tx.NamedExec(`INSERT INTO users (id, username, password) `+
		`VALUES (:id, :username, :password)`, user)
	if err != nil {
		return err
	}
	tx.Commit()
	return nil
}
