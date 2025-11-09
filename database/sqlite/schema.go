package sqlite

import (
	"log"

	"github.com/jmoiron/sqlx"
)

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
