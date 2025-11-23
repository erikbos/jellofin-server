package sqlite

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

func (i *SqliteRepo) dbInsertItem(tx *sqlx.Tx, item *model.Item) error {
	// item.Genrestring = strings.Join(item.Genre, ",")
	_, err := tx.NamedExec(
		`INSERT INTO items(id, name, votes, genre, rating, year, nfotime, `+
			`		firstvideo, lastvideo)`+
			`VALUES (:id, :name, :votes, :genre, :rating, :year, :nfotime, `+
			`		:firstvideo, :lastvideo)`, item)
	return err
}

func (i *SqliteRepo) dbUpdateItem(tx *sqlx.Tx, item *model.Item) error {
	// item.Genrestring = strings.Join(item.Genre, ",")
	_, err := tx.NamedExec(
		`UPDATE items SET votes = :votes, genre = :genre, rating = :rating, `+
			`		year = :year, nfotime = :nfotime, `+
			`		firstvideo = :firstvideo, lastvideo = :lastvideo `+
			`		WHERE name = :name`, item)
	return err
}

func (i *SqliteRepo) DbLoadItem(item *model.Item) {
	var data model.Item

	// Find this item by name in the database.
	tx, _ := i.dbHandle.Beginx()
	err := i.dbHandle.Get(&data, "SELECT id, name, votes, genre, rating, year, nfotime, firstvideo, lastvideo FROM items WHERE name=? LIMIT 1", item.Name)

	// Not in database yet, insert
	if err == sql.ErrNoRows {
		// itemCheckNfo(item)
		// fmt.Printf("dbLoadItem: add to database: %s\n", item.Name)
		item.ID = idhash.IdHash(item.Name)
		err = i.dbInsertItem(tx, item)
		if err != nil {
			// INSERT: error: UNIQUE constraint failed: items.id
			// if strings.Contains(err.Error(), "UNIQUE constraint") {
			fmt.Printf("dbLoadItem: INSERT: name=%s, id=%s: error: %s\n", item.Name, item.ID, err)
			os.Exit(1)
			tx.Rollback()
			return
		}
		tx.Commit()
		return
	}

	// Error? Too bad.
	if err != nil {
		fmt.Printf("dbLoadItem (%s): %s\n", item.Name, err)
		tx.Rollback()
		return
	}

	needUpdate := false

	// item.Id = data.Id
	// item.Genre = strings.Split(data.Genre, ",")
	// item.Rating = data.Rating
	// item.Votes = data.Votes
	// item.NfoTime = data.NfoTime

	// if data.Year == 0 && item.Year > 0 {
	// 	needUpdate = true
	// } else {
	// 	item.Year = data.Year
	// }

	// if item.FirstVideo == 0 {
	// 	item.FirstVideo = data.FirstVideo
	// }
	// if item.LastVideo == 0 {
	// 	item.LastVideo = data.LastVideo
	// }

	// if item.FirstVideo != data.FirstVideo ||
	// 	item.LastVideo != data.LastVideo {
	// 	needUpdate = true
	// }

	// Got it. See if we need to update the database.
	// if itemCheckNfo(item) {
	// 	needUpdate = true
	// }

	if needUpdate {
		err = i.dbUpdateItem(tx, item)
		if err != nil {
			fmt.Printf("dbLoadItem %s: update: %s\n", item.Name, err)
			tx.Rollback()
			return
		}
	}

	tx.Commit()
}

// Check NFO file.
// func (d *Database) itemCheckNfo(item *collection.Item) (updated bool) {
// 	if item.NfoPath == "" {
// 		return
// 	}
// 	if item.NfoTime > 0 {
// 		fi, err := os.Stat(item.NfoPath)
// 		if err == nil && TimeToUnixMS(fi.ModTime()) == item.NfoTime {
// 			return
// 		}
// 	}

// 	fh, err := os.Open(item.NfoPath)
// 	if err != nil {
// 		fmt.Printf("XXX DEBUG open %s: %s\n", item.NfoPath, err)
// 		return
// 	}
// 	nfo := nfo.DecodeNfo(fh)
// 	ftime := int64(0)
// 	fi, err := fh.Stat()
// 	if err == nil {
// 		ftime = TimeToUnixMS(fi.ModTime())
// 	}
// 	fh.Close()
// 	if nfo == nil {
// 		fmt.Printf("XXX DEBUG XML failed %s\n", item.Name)
// 		return
// 	}
// 	otime := item.NfoTime

// 	item.NfoTime = ftime
// 	item.Genre = nfo.Genre
// 	item.Rating = nfo.Rating
// 	item.Votes = nfo.Votes
// 	if nfo.Year != 0 {
// 		item.Year = nfo.Year
// 	}
// 	updated = ftime > otime
// 	return
// }
