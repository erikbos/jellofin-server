package sqlite

import (
	"context"

	"github.com/erikbos/jellofin-server/database/model"
)

// GetQuickConnectCodeBySecret retrieves a quick connect code for a user by secret string.
func (s *SqliteRepo) GetQuickConnectCodeBySecret(ctx context.Context, secret string) (*model.QuickConnectCode, error) {
	query := `SELECT userid, deviceid, secret, authorized, code, created FROM quickconnect WHERE secret=? LIMIT 1`
	return s.loadQuickConnectCode(s.dbReadHandle.QueryRowContext(ctx, query, secret))
}

// GetQuickConnectCodeByCode retrieves a quick connect code for a user by code string.
func (s *SqliteRepo) GetQuickConnectCodeByCode(ctx context.Context, code string) (*model.QuickConnectCode, error) {
	query := `SELECT userid, deviceid, secret, authorized, code, created FROM quickconnect WHERE code=? LIMIT 1`
	return s.loadQuickConnectCode(s.dbReadHandle.QueryRowContext(ctx, query, code))
}

func (s *SqliteRepo) loadQuickConnectCode(scanner sqlScanner) (*model.QuickConnectCode, error) {
	var t model.QuickConnectCode
	if err := scanner.Scan(&t.UserID,
		&t.DeviceID,
		&t.Secret,
		&t.Authorized,
		&t.Code,
		&t.Created); err != nil {
		return nil, model.ErrNotFound
	}
	return &t, nil
}

// UpsertQuickConnectCode inserts or updates a quick connect code for a user.
func (s *SqliteRepo) UpsertQuickConnectCode(ctx context.Context, code model.QuickConnectCode) error {
	query := `REPLACE INTO quickconnect (userid, deviceid, secret, authorized, code, created)
	VALUES (?, ?, ?, ?, ?, ?)`

	_, err := s.dbWriteHandle.ExecContext(ctx, query,
		code.UserID,
		code.DeviceID,
		code.Secret,
		code.Authorized,
		code.Code,
		code.Created)
	return err
}
