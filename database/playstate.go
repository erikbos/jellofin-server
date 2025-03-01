package database

import (
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

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
	Position int
	// Played playedPercentage
	PlayedPercentage int
	// True if the item has been fully played
	Played bool
	// Timestamp of item playing
	Timestamp time.Time
}

func (p *PlayStateRepo) Init() {
	p.state = make(map[PlayStateKey]PlayStateEntry)
	p.LoadPlayStateFromDB()

	p.lastDBSyncTime = time.Now().UTC()
	p.StartSyncerToDB()
}

// FIXME: should come from jellyfin package
const itemprefix_separator = "_"

// Update stores the play state details for a user and item.
func (p *PlayStateRepo) Update(userId, itemId string, details PlayStateEntry) {
	if i := strings.Index(itemId, itemprefix_separator); i != -1 {
		itemId = itemId[i+1:]
	}

	details.Timestamp = time.Now().UTC()

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
			Position:         ps.Position,
			PlayedPercentage: ps.PlayedPercentage,
			Played:           ps.Played,
			Timestamp:        ps.Timestamp,
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
		if value.Timestamp.After(p.lastDBSyncTime) {
			log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.UserId, key.ItemId, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :timestamp)`,
				map[string]interface{}{
					"userid":           key.UserId,
					"itemid":           key.ItemId,
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
