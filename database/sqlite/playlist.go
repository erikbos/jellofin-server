package sqlite

import (
	"context"
	"log"

	"github.com/erikbos/jellofin-server/database/model"
)

// GetPlaylist returns a playlist for the given user and playlist ID.
func (s *SqliteRepo) GetPlaylist(ctx context.Context, userID, playlistID string) (*model.Playlist, error) {
	query := `SELECT id, name, userid, created, lastupdated FROM playlists WHERE userid=? AND id=? LIMIT 1`
	row := s.dbReadHandle.QueryRowContext(ctx, query, userID, playlistID)
	var playlist model.Playlist
	if err := row.Scan(&playlist.ID, &playlist.Name, &playlist.UserID, &playlist.Created, &playlist.LastUpdated); err != nil {
		return nil, model.ErrNotFound
	}

	query = `SELECT itemid FROM playlist_items WHERE playlistid=? ORDER BY itemorder`
	rows, err := s.dbReadHandle.QueryContext(ctx, query, playlistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	playlist.ItemIDs = []string{}
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		playlist.ItemIDs = append(playlist.ItemIDs, itemID)
	}
	return &playlist, nil
}

// GetPlaylists returns a list of playlist IDs for the given user.
func (s *SqliteRepo) GetPlaylists(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT id FROM playlists WHERE userid=?`
	rows, err := s.dbReadHandle.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var playlistIDs []string
	for rows.Next() {
		var itemID string
		if err := rows.Scan(&itemID); err != nil {
			return nil, err
		}
		playlistIDs = append(playlistIDs, itemID)
	}
	return playlistIDs, nil
}

// UpsertPlaylist creates or updates a playlist for the given user and returns the playlist ID.
func (s *SqliteRepo) UpsertPlaylist(ctx context.Context, newPlaylist model.Playlist) (err error) {
	tx, err := s.dbWriteHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	const query = `REPLACE INTO playlists (id, name, userid, created, lastupdated) VALUES (?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(ctx, query,
		newPlaylist.ID, newPlaylist.Name, newPlaylist.UserID, newPlaylist.Created, newPlaylist.LastUpdated); err != nil {
		return err
	}

	order := 1
	for _, itemID := range newPlaylist.ItemIDs {
		const query = `REPLACE INTO playlist_items (playlistid, itemid, itemorder, created, lastupdated) VALUES (?, ?, ?, ?, ?)`
		if _, err := tx.ExecContext(ctx, query,
			newPlaylist.ID, itemID, order, newPlaylist.Created, newPlaylist.LastUpdated); err != nil {
			return err
		}
		order++
	}
	return tx.Commit()
}

func (s *SqliteRepo) AddItemsToPlaylist(ctx context.Context, playlistID string, itemIDs []string) error {
	log.Printf("AddItemsToPlaylist: %s, %+v\n", playlistID, itemIDs)
	return nil

}

func (s *SqliteRepo) DeleteItemsFromPlaylist(ctx context.Context, playlistID string, itemIDs []string) error {
	log.Printf("DeleteItemsFromPlaylist: %s, %+v\n", playlistID, itemIDs)

	tx, err := s.dbWriteHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, itemID := range itemIDs {
		const query = `DELETE FROM playlist_items WHERE playlistid=? AND itemid=?`
		if _, err := tx.ExecContext(ctx, query, playlistID, itemID); err != nil {
			return err
		}
	}
	return tx.Commit()
}
