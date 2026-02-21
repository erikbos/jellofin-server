package sqlite

import (
	"context"
	"log"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

func (s *SqliteRepo) CreatePlaylist(ctx context.Context, newPlaylist model.Playlist) (playlistID string, err error) {
	log.Printf("CreatePlaylist: %+v", newPlaylist)

	// every create playlist will have a unique id (=Jellyfin behaviour)
	newPlaylist.ID = idhash.NewRandomID()

	tx, err := s.dbWriteHandle.Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err = tx.NamedExecContext(ctx, `INSERT INTO playlist (id, name, userid, timestamp)
		VALUES (:id, :name, :userid, :timestamp)`,
		map[string]any{
			"id":        newPlaylist.ID,
			"name":      newPlaylist.Name,
			"userid":    newPlaylist.UserID,
			"timestamp": time.Now().UTC(),
		}); err != nil {
		return "", err
	}

	order := 1
	for _, itemID := range newPlaylist.ItemIDs {
		_, err := tx.NamedExecContext(ctx, `INSERT INTO playlist_item (playlistid, itemid, itemorder, timestamp)
	            VALUES (:playlist_id, :item_id, :item_order, :timestamp)`,
			map[string]any{
				"playlist_id": newPlaylist.ID,
				"item_id":     itemID,
				"item_order":  order,
				"timestamp":   time.Now().UTC(),
			})
		if err != nil {
			return "", err
		}
		order++
	}
	return newPlaylist.ID, tx.Commit()
}

func (s *SqliteRepo) GetPlaylists(ctx context.Context, userID string) (playlistIDs []string, err error) {
	var playlistIDEntries []struct {
		ID string `db:"id"`
	}
	err = s.dbReadHandle.SelectContext(ctx, &playlistIDEntries, "SELECT id FROM playlist WHERE userid=?", userID)
	if err != nil {
		return
	}
	for _, row := range playlistIDEntries {
		playlistIDs = append(playlistIDs, row.ID)
	}
	return
}

func (s *SqliteRepo) GetPlaylist(ctx context.Context, userID, playlistID string) (*model.Playlist, error) {
	// log.Printf("db - GetPlaylist: %s\n", playlistID)

	var playlist struct {
		ID        string    `db:"id"`
		Name      string    `db:"name"`
		UserID    string    `db:"userid"`
		Timestamp time.Time `db:"timestamp"`
	}
	if err := s.dbReadHandle.GetContext(ctx, &playlist, "SELECT id, name, userid, timestamp FROM playlist WHERE userid=? AND id=? LIMIT 1",
		userID, playlistID); err != nil {
		return nil, err
	}

	result := &model.Playlist{
		ID:     playlist.ID,
		Name:   playlist.Name,
		UserID: playlist.UserID,
	}

	var playlistEntries []struct {
		PlaylistID string    `db:"playlistid"`
		ItemID     string    `db:"itemid"`
		ItemOrder  string    `db:"itemorder"`
		Timestamp  time.Time `db:"timestamp"`
	}
	if err := s.dbReadHandle.SelectContext(ctx, &playlistEntries, "SELECT playlistid, itemid, itemorder, timestamp FROM playlist_item WHERE playlistid=?",
		playlistID); err != nil {
		return nil, err
	}
	for _, ps := range playlistEntries {
		result.ItemIDs = append(result.ItemIDs, ps.ItemID)
	}
	return result, nil
}

func (s *SqliteRepo) AddItemsToPlaylist(ctx context.Context, UserID, playlistID string, itemIDs []string) error {
	log.Printf("AddItemsToPlaylist: %s, %s, %+v\n", UserID, playlistID, itemIDs)

	tx, err := s.dbWriteHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// get the highest order number of the playlist to determine the order of the new items
	var maxOrder int
	if err = tx.GetContext(ctx, &maxOrder,
		"SELECT COALESCE(MAX(itemorder), 0) FROM playlist_item WHERE playlistid = $1", playlistID); err != nil {
		log.Printf("AddItemsToPlaylist: err: %+v\n", err)
		return err
	}

	order := maxOrder + 1
	for _, itemID := range itemIDs {
		_, err := tx.NamedExecContext(ctx, `INSERT OR REPLACE INTO playlist_item (playlistid, itemid, itemorder, timestamp)
                VALUES (:playlistid, :itemid, :itemorder, :timestamp)`,
			map[string]any{
				"playlistid": playlistID,
				"itemid":     itemID,
				"itemorder":  order,
				"timestamp":  time.Now().UTC(),
			})
		log.Printf("AddItemsToPlaylist: err2: %+v\n", err)
		if err != nil {
			return err
		}
		order++
	}
	return tx.Commit()
}

func (s *SqliteRepo) DeleteItemsFromPlaylist(ctx context.Context, playlistID string, itemIDs []string) error {
	log.Printf("DeleteItemsFromPlaylist: %s, %+v\n", playlistID, itemIDs)
	return nil

}

func (s *SqliteRepo) MovePlaylistItem(ctx context.Context, playlistID string, itemID string, newIndex int) error {
	log.Printf("MovePlaylistItem: %s, %s, %d", playlistID, itemID, newIndex)
	return nil
}
