package sqlite

import (
	"context"
	"log"
	"strings"

	"github.com/erikbos/jellofin-server/database/model"
)

// GetUser retrieves a user.
func (s *SqliteRepo) GetUser(ctx context.Context, username string) (user *model.User, err error) {
	const query = `SELECT id, username,	password, created, lastlogin, lastused FROM users WHERE username=? LIMIT 1`
	return s.loadUser(s.dbReadHandle.QueryRowContext(ctx, query, username))
}

// GetByID retrieves a user from the database by their ID.
func (s *SqliteRepo) GetUserByID(ctx context.Context, userID string) (*model.User, error) {
	query := `SELECT id, username, password, created, lastlogin, lastused FROM users WHERE id=? LIMIT 1`
	return s.loadUser(s.dbReadHandle.QueryRowContext(ctx, query, userID))
}

// GetAllUsers retrieves all users from the database.
func (s *SqliteRepo) GetAllUsers(ctx context.Context) ([]model.User, error) {
	const query = `SELECT id, username,	password, created, lastlogin, lastused FROM users`
	rows, err := s.dbReadHandle.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []model.User
	for rows.Next() {
		if user, err := s.loadUser(rows); err == nil {
			users = append(users, *user)
		} else {
			log.Printf("Error loading user from db: %s\n", err)
		}
	}
	return users, rows.Err()
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func (s *SqliteRepo) loadUser(scanner sqlScanner) (*model.User, error) {
	var user model.User
	if err := scanner.Scan(
		&user.ID,
		&user.Username,
		&user.Password,
		&user.Created,
		&user.LastLogin,
		&user.LastUsed); err != nil {
		return nil, model.ErrNotFound
	}
	var err error
	user.Properties, err = s.loadUserProperties(context.Background(), user.ID)
	return &user, err
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
	err = tx.Commit()
	if err != nil {
		return err
	}
	return s.saveUserProperties(ctx, user.ID, user.Properties)
}

func (s *SqliteRepo) DeleteUser(ctx context.Context, userID string) error {
	tx, _ := s.dbWriteHandle.BeginTxx(ctx, nil)
	defer tx.Rollback()
	const query = `DELETE FROM users WHERE id = ?`
	_, err := tx.ExecContext(ctx, query, userID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Database keys for user properties
const (
	propAdmin            = "admin"
	propDisabled         = "disabled"
	propEnableAllFolders = "enableallfolders"
	propEnabledFolders   = "enabledfolders"
	propEnableDownloads  = "enabledownloads"
	propIsHidden         = "ishidden"
	propOrderedViews     = "orderedviews"
	propMyMediaExcludes  = "mymediaexcludes"
	propAllowTags        = "allowtags"
	propBlockTags        = "blocktags"
)

func (s *SqliteRepo) loadUserProperties(ctx context.Context, userID string) (model.UserProperties, error) {
	const query = ` SELECT key, value FROM user_properties WHERE userid = ?`
	rows, err := s.dbReadHandle.QueryContext(ctx, query, userID)
	if err != nil {
		return model.UserProperties{}, err
	}
	defer rows.Close()
	// We set default values for a user here in case we do not have entries in db.
	// jellyfin/user.go:createUser() has the same default values, so if we change defaults there, we should also change them here.
	props := model.UserProperties{
		IsHidden:         true,
		EnableAllFolders: true,
		EnableDownloads:  true,
	}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return model.UserProperties{}, err
		}
		switch key {
		case propAdmin:
			props.Admin = value == "1"
		case propEnableDownloads:
			props.EnableDownloads = value == "1"
		case propDisabled:
			props.Disabled = value == "1"
		case propEnableAllFolders:
			props.EnableAllFolders = value == "1"
		case propEnabledFolders:
			props.EnabledFolders = splitComma(value)
		case propIsHidden:
			props.IsHidden = value == "1"
		case propOrderedViews:
			props.OrderedViews = splitComma(value)
		case propMyMediaExcludes:
			props.MyMediaExcludes = splitComma(value)
		case propAllowTags:
			props.AllowTags = splitComma(value)
		case propBlockTags:
			props.BlockTags = splitComma(value)
		default:
			log.Printf("Unknown user property key: %s\n", key)
		}
	}
	return props, rows.Err()
}

func splitComma(value string) []string {
	if value == "" {
		return []string{}
	}
	return strings.Split(value, ",")
}

func (s *SqliteRepo) saveUserProperties(ctx context.Context, userID string, props model.UserProperties) error {
	const query = `INSERT INTO user_properties (userid, key, value) VALUES (?, ?, ?) ON CONFLICT(userid, key) DO UPDATE SET value = excluded.value`
	tx, err := s.dbWriteHandle.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	// User properties to save, we convert boolean values to "1" or "0" strings
	// and slice values to comma-separated strings
	properties := []struct{ key, value string }{
		{propAdmin, boolToString(props.Admin)},
		{propIsHidden, boolToString(props.IsHidden)},
		{propDisabled, boolToString(props.Disabled)},
		{propEnableDownloads, boolToString(props.EnableDownloads)},
		{propEnableAllFolders, boolToString(props.EnableAllFolders)},
		{propEnabledFolders, strings.Join(props.EnabledFolders, ",")},
		{propOrderedViews, strings.Join(props.OrderedViews, ",")},
		{propMyMediaExcludes, strings.Join(props.MyMediaExcludes, ",")},
		{propAllowTags, strings.Join(props.AllowTags, ",")},
		{propBlockTags, strings.Join(props.BlockTags, ",")},
	}
	for _, item := range properties {
		// log.Printf("Saving user property for userID: %s, key: %s, value: %s\n", userID, item.key, item.value)
		if _, err := stmt.ExecContext(ctx, userID, item.key, item.value); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func boolToString(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
