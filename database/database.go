package database

import (
	"errors"
	"fmt"
	"log"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type (
	Options struct {
		Filename string
	}

	DatabaseRepo struct {
		UserRepo
		AccessTokenRepo
		ItemRepo
		UserDataRepo
		PlaylistRepo
	}

	// UserRepo defines the interface for user database operations
	UserRepo interface {
		// GetByID retrieves a user from the database by their ID.
		GetByID(userID string) (user *User, err error)
		// Validate checks if the user exists and the password is correct.
		Validate(username, password string) (user *User, err error)
		// Insert inserts a new user into the database.
		Insert(username, password string) (user *User, err error)
	}

	AccessTokenRepo interface {
		// Get accesstoken details by tokenid
		Get(token string) (*AccessToken, error)
		// Generate generates new access token
		Generate(userID string) (string, error)
		// BackgroundJobs syncs changed accesstokens periodically to database
		BackgroundJobs()
	}

	ItemRepo interface {
		DbLoadItem(item *Item)
	}

	UserDataRepo interface {
		// Get the play state details for an item per user.
		Get(userID, itemID string) (details UserData, err error)
		// Get all favorite items of a user.
		GetFavorites(userID string) (favoriteItems []string, err error)
		// Update stores the play state details for a user and item.
		Update(userID, itemID string, details UserData) error
		// BackgroundJobs syncs changed play state to periodically to database.
		BackgroundJobs()
	}

	// PlaylistRepo defines the interface for database operations
	PlaylistRepo interface {
		CreatePlaylist(Playlist) (playlistID string, err error)
		GetPlaylists(userID string) (playlistIDs []string, err error)
		GetPlaylist(userID, playlistID string) (*Playlist, error)
		AddItemsToPlaylist(userID, playlistID string, itemIDs []string) error
		DeleteItemsFromPlaylist(playlistID string, itemIDs []string) error
		MovePlaylistItem(playlistID string, itemID string, newIndex int) error
	}
)

var (
	ErrNoDbHandle = errors.New("db connection not available")
)

func New(o *Options) (*DatabaseRepo, error) {
	if o.Filename == "" {
		return nil, fmt.Errorf("database directory not set")
	}
	dbHandle, err := sqlx.Connect("sqlite3", o.Filename)
	if err != nil {
		return nil, err
	}
	if err := dbInitSchema(dbHandle); err != nil {
		return nil, err
	}
	d := &DatabaseRepo{
		UserRepo:        NewUserStorage(dbHandle),
		AccessTokenRepo: NewAccessTokenStorage(dbHandle),
		ItemRepo:        NewItemStorage(dbHandle),
		UserDataRepo:    NewUserDataStorage(dbHandle),
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

		`CREATE TABLE IF NOT EXISTS accesstokens (
userid TEXT NOT NULL,
token TEXT NOT NULL,
lastused DATETIME);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS accesstokens_idx ON accesstokens (userid, token);`,

		`CREATE TABLE IF NOT EXISTS playstate (
userid TEXT NOT NULL,
itemid TEXT NOT NULL,
position INTEGER,
playedpercentage INTEGER,
played BOOLEAN,
favorite BOOLEAN,
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
