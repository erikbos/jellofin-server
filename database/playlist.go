package database

import (
	"log"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/miquels/notflix-server/idhash"
)

type PlaylistStorage struct {
	dbHandle *sqlx.DB
}

func NewPlaylistStorage(d *sqlx.DB) *PlaylistStorage {
	return &PlaylistStorage{
		dbHandle: d,
	}
}

type Playlist struct {
	ID      string
	UserID  string
	Name    string
	ItemIDs []string
}

func (p *PlaylistStorage) CreatePlaylist(newPlaylist Playlist) (playlistID string, err error) {
	log.Printf("CreatePlaylist: %+v", newPlaylist)

	// newPlaylist.ID = idhash.IdHash(newPlaylist.Name)
	// every create playlist will have a unique id (=Jellyfin behaviour)
	newPlaylist.ID = idhash.IdHash(newPlaylist.Name + time.Now().String())

	tx, err := p.dbHandle.Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err = tx.NamedExec(`INSERT INTO playlist (id, name, userid, timestamp)
		VALUES (:id, :name, :userid, :timestamp)`,
		map[string]interface{}{
			"id":        newPlaylist.ID,
			"name":      newPlaylist.Name,
			"userid":    newPlaylist.UserID,
			"timestamp": time.Now().UTC(),
		}); err != nil {
		return "", err
	}

	order := 1
	for _, itemID := range newPlaylist.ItemIDs {
		_, err := tx.NamedExec(`INSERT INTO playlist_item (playlistid, itemid, itemorder, timestamp)
	            VALUES (:playlist_id, :item_id, :item_order, :timestamp)`,
			map[string]interface{}{
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

func (p *PlaylistStorage) GetPlaylists(userID string) (playlistIDs []string, err error) {
	var playlistIDEntries []struct {
		ID string `db:"id"`
	}
	err = p.dbHandle.Select(&playlistIDEntries, "SELECT id FROM playlist WHERE userid=?", userID)
	if err != nil {
		return
	}
	for _, row := range playlistIDEntries {
		playlistIDs = append(playlistIDs, row.ID)
	}
	return
}

func (p *PlaylistStorage) GetPlaylist(playlistID string) (*Playlist, error) {
	// log.Printf("db - GetPlaylist: %s\n", playlistID)

	var playlist struct {
		ID        string    `db:"id"`
		Name      string    `db:"name"`
		UserID    string    `db:"userid"`
		Timestamp time.Time `db:"timestamp"`
	}
	if err := p.dbHandle.Get(&playlist, "SELECT * FROM playlist WHERE id=? LIMIT 1", playlistID); err != nil {
		return nil, err
	}

	result := &Playlist{
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
	if err := p.dbHandle.Select(&playlistEntries, "SELECT * FROM playlist_item WHERE playlistid=?",
		playlistID); err != nil {
		return nil, err
	}
	for _, ps := range playlistEntries {
		result.ItemIDs = append(result.ItemIDs, ps.ItemID)
	}
	return result, nil
}

func (p *PlaylistStorage) AddItemsToPlaylist(UserID, playlistID string, itemIDs []string) error {
	log.Printf("AddItemsToPlaylist: %s, %s, %+v\n", UserID, playlistID, itemIDs)

	tx, err := p.dbHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// get the highest order number of the playlist to determine the order of the new items
	var maxOrder int
	if err = tx.Get(&maxOrder,
		"SELECT COALESCE(MAX(itemorder), 0) FROM playlist_item WHERE playlistid = $1", playlistID); err != nil {
		log.Printf("AddItemsToPlaylist: err: %+v\n", err)
		return err
	}

	order := maxOrder + 1
	for _, itemID := range itemIDs {
		_, err := tx.NamedExec(`INSERT OR REPLACE INTO playlist_item (playlistid, itemid, itemorder, timestamp)
                VALUES (:playlistid, :itemid, :itemorder, :timestamp)`,
			map[string]interface{}{
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

func (p *PlaylistStorage) DeleteItemsFromPlaylist(playlistID string, itemIDs []string) error {
	log.Printf("DeleteItemsFromPlaylist: %s, %+v\n", playlistID, itemIDs)
	return nil

}

func (p *PlaylistStorage) MovePlaylistItem(playlistID string, itemID string, newIndex int) error {
	log.Printf("MovePlaylistItem: %s, %s, %d", playlistID, itemID, newIndex)
	return nil
}
