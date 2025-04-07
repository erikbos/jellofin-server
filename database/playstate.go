package database

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// in-memory access play state store, synced to disk by StartSyncerToDB()

type PlayStateStorage struct {
	dbHandle       *sqlx.DB
	mu             sync.Mutex
	state          map[playStateKey]PlayState
	lastDBSyncTime time.Time
}

func NewPlayStateStorage(d *sqlx.DB) *PlayStateStorage {
	p := &PlayStateStorage{
		dbHandle: d,
		state:    make(map[playStateKey]PlayState),
	}
	p.LoadStateFromDB()
	return p
}

type playStateKey struct {
	userID string
	itemID string
}

type PlayState struct {
	// Offset in seconds
	Position int
	// Played playedPercentage
	PlayedPercentage int
	// True if the item has been fully played
	Played bool
	// Timestamp of item playing
	Timestamp time.Time
}

// FIXME: should come from jellyfin package
const itemprefix_separator = "_"

// Update stores the play state details for a user and item.
func (p *PlayStateStorage) Update(userID, itemID string, details PlayState) {
	if i := strings.Index(itemID, itemprefix_separator); i != -1 {
		itemID = itemID[i+1:]
	}

	details.Timestamp = time.Now().UTC()

	// log.Printf("sessionUpdate: userID: %s, itemID: %s, data: %+v\n", userID, itemID, details)

	p.mu.Lock()
	defer p.mu.Unlock()

	key := playStateKey{userID: userID, itemID: itemID}
	p.state[key] = details
}

// Get the play state details for an item per user.
func (p *PlayStateStorage) Get(userID, itemID string) (details PlayState, err error) {
	if i := strings.Index(itemID, itemprefix_separator); i != -1 {
		itemID = itemID[i+1:]
	}
	// log.Printf("sessionGet: userID: %s, itemID: %s\n", userID, itemID)

	p.mu.Lock()
	defer p.mu.Unlock()

	key := playStateKey{userID: userID, itemID: itemID}
	if details, ok := p.state[key]; ok {
		return details, nil
	}
	err = errors.New("play state not found")
	return
}

// LoadPlayStateFromDB loads playstate table into memory.
func (p *PlayStateStorage) LoadStateFromDB() error {
	if p.dbHandle == nil {
		return errors.New("db connection not available")
	}

	var playStates []struct {
		UserID           string    `db:"userid"`
		ItemID           string    `db:"itemid"`
		Position         int       `db:"position"`
		PlayedPercentage int       `db:"playedpercentage"`
		Played           bool      `db:"played"`
		Timestamp        time.Time `db:"timestamp"`
	}

	err := p.dbHandle.Select(&playStates, "SELECT * FROM playstate")
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ps := range playStates {
		key := playStateKey{userID: ps.UserID, itemID: ps.ItemID}
		p.state[key] = PlayState{
			Position:         ps.Position,
			PlayedPercentage: ps.PlayedPercentage,
			Played:           ps.Played,
			Timestamp:        ps.Timestamp,
		}
	}
	return nil
}

// writePlayStateToDB writes all changed entries in PlayStateRepo.state to db.
func (p *PlayStateStorage) writeStateToDB() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.dbHandle == nil {
		return ErrNoDbHandle
	}

	tx, err := p.dbHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, value := range p.state {
		if value.Timestamp.After(p.lastDBSyncTime) {
			log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.userID, key.itemID, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :timestamp)`,
				map[string]interface{}{
					"userid":           key.userID,
					"itemid":           key.itemID,
					"position":         value.Position,
					"playedPercentage": value.PlayedPercentage,
					"played":           value.Played,
					"timestamp":        value.Timestamp.UTC(),
				})
			if err != nil {
				return err
			}
		}
	}

	p.lastDBSyncTime = time.Now()
	return tx.Commit()
}

// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
func (p *PlayStateStorage) BackgroundJobs() {
	if p.dbHandle == nil {
		log.Fatal(ErrNoDbHandle)
	}

	p.LoadStateFromDB()
	p.lastDBSyncTime = time.Now()

	for {
		if err := p.writeStateToDB(); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(10 * time.Second)
	}
}
