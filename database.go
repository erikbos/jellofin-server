package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type DbItem struct {
	Id         string
	Name       string
	Votes      int
	Genre      string
	Rating     float32
	Year       int
	NfoTime    int64
	FirstVideo int64
	LastVideo  int64
}

type DbUser struct {
	Id       string
	Username string
	Password string
}

var dbHandle *sqlx.DB

func dbInit(dbFile string) (err error) {
	dbHandle, err = sqlx.Connect("sqlite3", dbFile)
	if err != nil {
		return
	}
	err = dbInitSchema()
	// if err == nil {
	// 	_, err = dbHandle.Query("SELECT count(*) FROM items")
	// 	if err != nil {
	// 		// database is empty, CREATE tables.
	// 		err = dbInitSchema()
	// 	}
	// }
	return
}

func dbInitSchema() error {
	tx, err := dbHandle.Beginx()
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
	}

	for _, query := range schema {
		if _, err = tx.Exec(query); err != nil {
			log.Printf("dbInitSchema error: %s\n", err)
			tx.Rollback()
			return err
		}
	}

	tx.Commit()
	return err
}

// Check NFO file.
func itemCheckNfo(item *Item) (updated bool) {
	if item.NfoPath == "" {
		return
	}
	if item.NfoTime > 0 {
		fi, err := os.Stat(item.NfoPath)
		if err == nil && TimeToUnixMS(fi.ModTime()) == item.NfoTime {
			return
		}
	}

	fh, err := os.Open(item.NfoPath)
	if err != nil {
		fmt.Printf("XXX DEBUG open %s: %s\n", item.NfoPath, err)
		return
	}
	nfo := decodeNfo(fh)
	ftime := int64(0)
	fi, err := fh.Stat()
	if err == nil {
		ftime = TimeToUnixMS(fi.ModTime())
	}
	fh.Close()
	if nfo == nil {
		fmt.Printf("XXX DEBUG XML failed %s\n", item.Name)
		return
	}
	otime := item.NfoTime

	item.NfoTime = ftime
	item.Genre = nfo.Genre
	item.Rating = nfo.Rating
	item.Votes = nfo.Votes
	if nfo.Year != 0 {
		item.Year = nfo.Year
	}
	updated = ftime > otime
	return
}

func dbInsertItem(tx *sqlx.Tx, item *Item) (err error) {
	item.Genrestring = strings.Join(item.Genre, ",")
	_, err = tx.NamedExec(
		`INSERT INTO items(id, name, votes, genre, rating, year, nfotime, `+
			`		firstvideo, lastvideo)`+
			`VALUES (:id, :name, :votes, :genrestring, :rating, :year, :nfotime, `+
			`		:firstvideo, :lastvideo)`, item)
	return
}

func dbUpdateItem(tx *sqlx.Tx, item *Item) (err error) {
	item.Genrestring = strings.Join(item.Genre, ",")
	_, err = tx.NamedExec(
		`UPDATE items SET votes = :votes, genre = :genrestring, rating = :rating, `+
			`		year = :year, nfotime = :nfotime, `+
			`		firstvideo = :firstvideo, lastvideo = :lastvideo `+
			`		WHERE name = :name`, item)
	return
}

func dbLoadItem(coll *Collection, item *Item) {
	var data DbItem

	// Find this item by name in the database.
	tx, _ := dbHandle.Beginx()
	err := dbHandle.Get(&data, "SELECT * FROM items WHERE name=? LIMIT 1", item.Name)

	// Not in database yet, insert
	if err == sql.ErrNoRows {
		itemCheckNfo(item)
		// fmt.Printf("dbLoadItem: add to database: %s\n", item.Name)
		item.Id = idHash(item.Name)
		err = dbInsertItem(tx, item)
		if err != nil {
			// INSERT: error: UNIQUE constraint failed: items.id
			// if strings.Contains(err.Error(), "UNIQUE constraint") {
			fmt.Printf("dbLoadItem: INSERT: name=%s, id=%s: error: %s\n", item.Name, item.Id, err)
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

	item.Id = data.Id
	item.Genre = strings.Split(data.Genre, ",")
	item.Rating = data.Rating
	item.Votes = data.Votes
	item.NfoTime = data.NfoTime

	if data.Year == 0 && item.Year > 0 {
		needUpdate = true
	} else {
		item.Year = data.Year
	}

	if item.FirstVideo == 0 {
		item.FirstVideo = data.FirstVideo
	}
	if item.LastVideo == 0 {
		item.LastVideo = data.LastVideo
	}

	if item.FirstVideo != data.FirstVideo ||
		item.LastVideo != data.LastVideo {
		needUpdate = true
	}

	// Got it. See if we need to update the database.
	if itemCheckNfo(item) {
		needUpdate = true
	}

	if needUpdate {
		err = dbUpdateItem(tx, item)
		if err != nil {
			fmt.Printf("dbLoadItem %s: update: %s\n", item.Name, err)
			tx.Rollback()
			return
		}
	}

	tx.Commit()
}

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
)

// dbUserValidate checks if the user exists and the password is correct.
func dbUserValidate(username, password *string) (user *DbUser, err error) {
	var data DbUser
	sqlerr := dbHandle.Get(&data, "SELECT * FROM users WHERE username=? LIMIT 1", username)
	if sqlerr != nil {
		return nil, ErrUserNotFound

	}
	err = bcrypt.CompareHashAndPassword([]byte(data.Password), []byte(*password))
	if err != nil {
		return nil, ErrInvalidPassword
	}
	return &data, nil
}

// dbUserInsert inserts a new user into the database.
func dbUserInsert(username, password *string) (user *DbUser, err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.MinCost)
	if err != nil {
		return nil, err
	}

	user = &DbUser{
		Id:       idHash(*username),
		Username: *username,
		Password: string(hashedPassword),
	}

	tx, _ := dbHandle.Beginx()
	_, err = tx.NamedExec(`INSERT INTO users (id, username, password) `+
		`VALUES (:id, :username, :password)`, user)
	if err != nil {
		tx.Rollback()
	}
	tx.Commit()
	return
}

////

var PlayState PlayStateRepo

type PlayStateRepo struct {
	mu             sync.Mutex
	state          map[PlayStateKey]PlayStateEntry
	lastDBSyncTime time.Time
}

type PlayStateKey struct {
	UserId string
	ItemId string
}

type PlayStateEntry struct {
	// Offset in seconds
	position int
	// Played playedPercentage
	playedPercentage int
	// True if the item has been fully played
	played bool
	// Timestamp of item playing
	timestamp time.Time
}

func (p *PlayStateRepo) Init() {
	p.state = make(map[PlayStateKey]PlayStateEntry)
	p.LoadPlayStateFromDB()

	p.lastDBSyncTime = time.Now().UTC()
	p.StartSyncerToDB()
}

// Update stores the play state details for a user and item.
func (p *PlayStateRepo) Update(userId, itemId string, details PlayStateEntry) {
	if i := strings.Index(itemId, itemprefix_separator); i != -1 {
		itemId = itemId[i+1:]
	}

	details.timestamp = time.Now().UTC()

	log.Printf("sessionUpdate: userId: %s, itemId: %s, data: %+v\n", userId, itemId, details)

	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	key := PlayStateKey{UserId: userId, ItemId: itemId}
	p.state[key] = details
}

// Get the play state details for an item per user.
func (p *PlayStateRepo) Get(userId, itemId string) (details PlayStateEntry, err error) {
	if i := strings.Index(itemId, itemprefix_separator); i != -1 {
		itemId = itemId[i+1:]
	}
	// log.Printf("sessionGet: userId: %s, itemId: %s\n", userId, itemId)

	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	key := PlayStateKey{UserId: userId, ItemId: itemId}
	if details, ok := p.state[key]; ok {
		return details, nil
	}
	err = errors.New("play state not found")
	return
}

// LoadPlayStateFromDB loads playstate table into memory.
func (p *PlayStateRepo) LoadPlayStateFromDB() error {
	if dbHandle == nil {
		return nil
	}

	var playStates []struct {
		UserId           string    `db:"userid"`
		ItemId           string    `db:"itemid"`
		Position         int       `db:"position"`
		PlayedPercentage int       `db:"playedpercentage"`
		Played           bool      `db:"played"`
		Timestamp        time.Time `db:"timestamp"`
	}

	err := dbHandle.Select(&playStates, "SELECT * FROM playstate")
	if err != nil {
		return err
	}

	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	for _, ps := range playStates {
		key := PlayStateKey{UserId: ps.UserId, ItemId: ps.ItemId}
		p.state[key] = PlayStateEntry{
			position:         ps.Position,
			playedPercentage: ps.PlayedPercentage,
			played:           ps.Played,
			timestamp:        ps.Timestamp,
		}
	}
	return nil
}

// WritePlayStateToDB writes all changed entries in PlayStateRepo.state to the SQLite table playstate.
func (p *PlayStateRepo) WritePlayStateToDB() error {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	if dbHandle == nil {
		return nil
	}

	tx, err := dbHandle.Beginx()
	if err != nil {
		return err
	}

	for key, value := range p.state {
		if value.timestamp.After(p.lastDBSyncTime) {
			log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.UserId, key.ItemId, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :timestamp)`,
				map[string]interface{}{
					"userid":           key.UserId,
					"itemid":           key.ItemId,
					"position":         value.position,
					"playedPercentage": value.playedPercentage,
					"played":           value.played,
					"timestamp":        value.timestamp.UTC(),
				})
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	p.lastDBSyncTime = time.Now()
	return tx.Commit()
}

// StartSyncerToDB writes the play state to the database every 3 seconds.
func (p *PlayStateRepo) StartSyncerToDB() {
	go func() {
		for {
			err := p.WritePlayStateToDB()
			if err != nil {
				log.Printf("Error writing play state to DB: %s\n", err)
			}
			time.Sleep(3 * time.Second)
		}
	}()
}
