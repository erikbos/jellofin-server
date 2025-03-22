package database

import (
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "modernc.org/sqlite"
)

type (
	Options struct {
		Filename string
	}

	DatabaseRepo struct {
		UserRepo
		AccessTokenRepo
		ItemRepo
		PlayStateRepo
		PlaylistRepo
	}

	// UserRepo defines the interface for user database operations
	UserRepo interface {
		// GetById retrieves a user from the database by their ID.
		GetById(UserID string) (user *User, err error)
		// Validate checks if the user exists and the password is correct.
		Validate(username, password string) (user *User, err error)
		// Insert inserts a new user into the database.
		Insert(username, password string) (user *User, err error)
	}

	AccessTokenRepo interface {
		// Get accesstoken details by tokenid
		Get(token string) (*AccessToken, error)
		// Generate generates new token
		Generate(UserID string) string
	}

	ItemRepo interface {
		DbLoadItem(item *Item)
	}

	PlayStateRepo interface {
		// Get the play state details for an item per user.
		Get(UserID, itemID string) (details PlayState, err error)
		// Update stores the play state details for a user and item.
		Update(UserID, itemID string, details PlayState)
		// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
		BackgroundJobs()
	}

	// PlaylistRepo defines the interface for database operations
	PlaylistRepo interface {
		CreatePlaylist(Playlist) (playlistID string, err error)
		GetPlaylists(UserID string) (playlistIDs []string, err error)
		GetPlaylist(playlistID string) (*Playlist, error)
		AddItemsToPlaylist(UserID, playlistID string, itemIDs []string) error
		DeleteItemsFromPlaylist(playlistID string, itemIDs []string) error
		MovePlaylistItem(playlistID string, itemID string, newIndex int) error
	}
)

func New(o *Options) (*DatabaseRepo, error) {
	if o.Filename == "" {
		return nil, fmt.Errorf("database directory not set")
	}
	dbHandle, err := sqlx.Open("sqlite", o.Filename)
	if err != nil {
		return nil, err
	}
	if err := dbInitSchema(dbHandle); err != nil {
		return nil, err
	}
	d := &DatabaseRepo{
		UserRepo:        NewUserStorage(dbHandle),
		AccessTokenRepo: NewAccessTokenStorage(),
		ItemRepo:        NewItemStorage(dbHandle),
		PlayStateRepo:   NewPlayStateStorage(dbHandle),
		PlaylistRepo:    NewPlaylistStorage(dbHandle),
	}
	return d, nil
}

func dbInitSchema(d *sqlx.DB) error {
	tx, err := d.Beginx()
	if err != nil {
		return err
	}

	schema := []string{
		`CREATE TABLE IF NOT EXISTS items (
id TEXT NOT NULL PRIMARY KEY,
name TEXT NOT NULL,
votes INTEGER,
year INTEGER,
genre TEXT NOT NULL,
rating REAL,
nfotime INTEGER NOT NULL,
firstvideo INTEGER NOT NULL,
lastvideo INTEGER NOT NULL);`,

		`CREATE INDEX IF NOT EXISTS items_name_idx ON items (name);`,

		`CREATE TABLE IF NOT EXISTS users (
id TEXT NOT NULL PRIMARY KEY,
username TEXT NOT NULL,
password TEXT NOT NULL);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS users_name_idx ON users (username);`,

		`CREATE TABLE IF NOT EXISTS playstate (
userid TEXT NOT NULL,
itemid TEXT NOT NULL,
position INTEGER,
playedpercentage INTEGER,
played BOOLEAN,
timestamp DATETIME);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS userid_itemid_idx ON playstate (userid, itemid);`,

		`CREATE TABLE IF NOT EXISTS playlist (
id TEXT NOT NULL PRIMARY KEY,
name TEXT NOT NULL,
userid TEXT NOT NULL,
timestamp DATETIME);`,

		`CREATE TABLE IF NOT EXISTS playlist_item (
playlistid TEXT NOT NULL,
itemid TEXT NOT NULL,
itemorder INTEGER NOT NULL,
timestamp DATETIME,
PRIMARY KEY (playlistid, itemid),
FOREIGN KEY (playlistid) REFERENCES playlists(id)
);`,
	}

	for _, query := range schema {
		if _, err = tx.Exec(query); err != nil {
			log.Printf("dbInitSchema error: %s\n", err)
			return tx.Rollback()
		}
	}

	return tx.Commit()
}
