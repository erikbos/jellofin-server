package sqlite

import (
	"log"

	"github.com/jmoiron/sqlx"
)

func dbInitSchema(d *sqlx.DB) error {
	schema := []string{
		// This is needed to improve concurrent reads and writes.
		`PRAGMA journal_mode = WAL;`,
		// Without this foreign key constraints won't be enforced and cascade deletes won't happen.
		`PRAGMA foreign_keys = ON;`,

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
password TEXT NOT NULL,
created DATETIME,
lastlogin DATETIME,
lastused DATETIME);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS users_name_idx ON users (username);`,

		`CREATE TABLE IF NOT EXISTS user_properties (
userid INTEGER NOT NULL,
key TEXT NOT NULL,
value TEXT,
PRIMARY KEY (userid, key),
FOREIGN KEY (userid) REFERENCES users(id) ON DELETE CASCADE
);`,

		`CREATE TABLE IF NOT EXISTS accesstokens (
userid TEXT NOT NULL,
token TEXT NOT NULL,
deviceid TEXT,
devicename TEXT,
applicationname TEXT,
applicationversion TEXT,
remoteaddress TEXT,
created DATETIME,
lastused DATETIME);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS accesstokens_idx ON accesstokens (userid, token);`,

		`CREATE TABLE IF NOT EXISTS quickconnect (
userid TEXT NOT NULL,
deviceid TEXT NOT NULL,
secret TEXT NOT NULL,
authorized BOOLEAN NOT NULL,
code TEXT NOT NULL,
created DATETIME NOT NULL,
PRIMARY KEY(deviceid, secret));`,

		`CREATE TABLE IF NOT EXISTS playstate (
userid TEXT NOT NULL,
itemid TEXT NOT NULL,
position INTEGER,
playedpercentage INTEGER,
played BOOLEAN,
playcount INTEGER,
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
FOREIGN KEY (playlistid) REFERENCES playlists(id));`,

		`CREATE TABLE IF NOT EXISTS images (
itemid TEXT NOT NULL,
type TEXT NOT NULL,
mimetype TEXT NOT NULL,
etag TEXT NOT NULL,
updated DATETIME NOT NULL,
filesize INTEGER NOT NULL,
data BLOB NOT NULL);`,

		`CREATE UNIQUE INDEX IF NOT EXISTS images_idx ON images (itemid, type)`,
	}

	for _, query := range schema {
		if _, err := d.Exec(query); err != nil {
			log.Printf("dbInitSchema error: %s\n", err)
			return err
		}
	}
	return nil
}
