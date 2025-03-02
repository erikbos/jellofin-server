package database

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

// in-memory access token store, synced to disk by StartSyncerToDB()

type PlayStateRepo struct {
	mu             sync.Mutex
	state          map[PlayStateKey]PlayStateEntry
	lastDBSyncTime time.Time
}

type PlayStateKey struct {
	userId string
	itemId string
}

type PlayStateEntry struct {
	// Offset in seconds
	Position int
	// Played playedPercentage
	PlayedPercentage int
	// True if the item has been fully played
	Played bool
	// Timestamp of item playing
	Timestamp time.Time
}

func (d *DatabaseRepo) PlayStateInit() {
	d.PlayState.state = make(map[PlayStateKey]PlayStateEntry)
}

// FIXME: should come from jellyfin package
const itemprefix_separator = "_"

// Update stores the play state details for a user and item.
func (d *DatabaseRepo) PlaystateUpdate(userId, itemId string, details PlayStateEntry) {
	if i := strings.Index(itemId, itemprefix_separator); i != -1 {
		itemId = itemId[i+1:]
	}

	details.Timestamp = time.Now().UTC()

	log.Printf("sessionUpdate: userId: %s, itemId: %s, data: %+v\n", userId, itemId, details)

	d.PlayState.mu.Lock()
	defer d.PlayState.mu.Unlock()

	key := PlayStateKey{userId: userId, itemId: itemId}
	d.PlayState.state[key] = details
}

// Get the play state details for an item per user.
func (d *DatabaseRepo) PlayStateGet(userId, itemId string) (details PlayStateEntry, err error) {
	if i := strings.Index(itemId, itemprefix_separator); i != -1 {
		itemId = itemId[i+1:]
	}
	// log.Printf("sessionGet: userId: %s, itemId: %s\n", userId, itemId)

	d.PlayState.mu.Lock()
	defer d.PlayState.mu.Unlock()

	key := PlayStateKey{userId: userId, itemId: itemId}
	if details, ok := d.PlayState.state[key]; ok {
		return details, nil
	}
	err = errors.New("play state not found")
	return
}

// LoadPlayStateFromDB loads playstate table into memory.
func (d *DatabaseRepo) PlaystateLoadStateFromDB() error {
	if d.dbHandle == nil {
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

	err := d.dbHandle.Select(&playStates, "SELECT * FROM playstate")
	if err != nil {
		return err
	}

	d.PlayState.mu.Lock()
	defer d.PlayState.mu.Unlock()

	for _, ps := range playStates {
		key := PlayStateKey{userId: ps.UserId, itemId: ps.ItemId}
		d.PlayState.state[key] = PlayStateEntry{
			Position:         ps.Position,
			PlayedPercentage: ps.PlayedPercentage,
			Played:           ps.Played,
			Timestamp:        ps.Timestamp,
		}
	}
	return nil
}

// WritePlayStateToDB writes all changed entries in PlayStateRepo.state to the SQLite table playstate.
func (d *DatabaseRepo) PlayStateWriteStateToDB() error {
	d.PlayState.mu.Lock()
	defer d.PlayState.mu.Unlock()

	if d.dbHandle == nil {
		return nil
	}

	tx, err := d.dbHandle.Beginx()
	if err != nil {
		return err
	}

	for key, value := range d.PlayState.state {
		if value.Timestamp.After(d.PlayState.lastDBSyncTime) {
			log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.userId, key.itemId, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :timestamp)`,
				map[string]interface{}{
					"userid":           key.userId,
					"itemid":           key.itemId,
					"position":         value.Position,
					"playedPercentage": value.PlayedPercentage,
					"played":           value.Played,
					"timestamp":        value.Timestamp.UTC(),
				})
			if err != nil {
				tx.Rollback()
				return err
			}
		}
	}

	d.PlayState.lastDBSyncTime = time.Now()
	return tx.Commit()
}

// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
func (d *DatabaseRepo) BackgroundJobs() {
	if d.dbHandle == nil {
		log.Fatal("StartBackgroundJobs called without db connection")
	}

	d.PlaystateLoadStateFromDB()
	d.PlayState.lastDBSyncTime = time.Now().UTC()

	for {
		if err := d.PlayStateWriteStateToDB(); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(3 * time.Second)
	}
}
