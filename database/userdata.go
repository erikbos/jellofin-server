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

type UserDataStorage struct {
	dbHandle       *sqlx.DB
	mu             sync.Mutex
	state          map[UserDataKey]UserData
	lastDBSyncTime time.Time
}

func NewUserDataStorage(d *sqlx.DB) *UserDataStorage {
	p := &UserDataStorage{
		dbHandle: d,
		state:    make(map[UserDataKey]UserData),
	}
	err := p.LoadStateFromDB()
	if err != nil {
		log.Printf("Error loading play state from db: %s\n", err)
	}
	return p
}

// UserDataKey is the key for the play state map.
type UserDataKey struct {
	userID string
	itemID string
}

type UserData struct {
	// Offset in seconds
	Position int
	// Played playedPercentage
	PlayedPercentage int
	// True if the item has been fully played
	Played bool
	// True if the item is favorite of user
	Favorite bool
	// Timestamp of item playing
	Timestamp time.Time
}

var (
	// FIXME: should come from jellyfin package
	itemprefix_separator = "_"

	ErrUserDataNotFound = errors.New("play state not found")
)

// Update stores the play state details for a user and item.
func (p *UserDataStorage) Update(userID, itemID string, details UserData) error {
	if i := strings.Index(itemID, itemprefix_separator); i != -1 {
		// fixme: stripping prefix should be done by caller
		itemID = itemID[i+1:]
	}

	details.Timestamp = time.Now().UTC()

	// log.Printf("UserDataStorageUpdate: userID: %s, itemID: %s, data: %+v\n", userID, itemID, details)

	p.mu.Lock()
	defer p.mu.Unlock()

	key := UserDataKey{userID: userID, itemID: itemID}
	p.state[key] = details

	return nil
}

// Get the play state details for an item per user.
func (p *UserDataStorage) Get(userID, itemID string) (details UserData, err error) {
	if i := strings.Index(itemID, itemprefix_separator); i != -1 {
		// fixme: stripping prefix should be done by caller
		itemID = itemID[i+1:]
	}
	// log.Printf("sessionGet: userID: %s, itemID: %s\n", userID, itemID)

	p.mu.Lock()
	defer p.mu.Unlock()

	key := UserDataKey{userID: userID, itemID: itemID}
	if details, ok := p.state[key]; ok {
		return details, nil
	}
	err = ErrUserDataNotFound
	return
}

// GetFavorites returns all favorite items of a user.
func (p *UserDataStorage) GetFavorites(userID string) (favoriteItems []string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, state := range p.state {
		if key.userID == userID && state.Favorite {
			favoriteItems = append(favoriteItems, key.itemID)
		}
	}
	return
}

// LoadUserDataFromDB loads UserData table into memory.
func (p *UserDataStorage) LoadStateFromDB() error {
	if p.dbHandle == nil {
		return errors.New("db connection not available")
	}

	var UserDatas []struct {
		UserID           string    `db:"userid"`
		ItemID           string    `db:"itemid"`
		Position         int       `db:"position"`
		PlayedPercentage int       `db:"playedpercentage"`
		Played           bool      `db:"played"`
		Favorite         bool      `db:"favorite"`
		Timestamp        time.Time `db:"timestamp"`
	}

	err := p.dbHandle.Select(&UserDatas, "SELECT * FROM playstate")
	if err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, ps := range UserDatas {
		key := UserDataKey{userID: ps.UserID, itemID: ps.ItemID}
		p.state[key] = UserData{
			Position:         ps.Position,
			PlayedPercentage: ps.PlayedPercentage,
			Played:           ps.Played,
			Favorite:         ps.Favorite,
			Timestamp:        ps.Timestamp,
		}
	}
	return nil
}

// writeUserDataToDB writes all changed entries in UserDataRepo.state to db.
func (p *UserDataStorage) writeStateToDB() error {
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
			// log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.userID, key.itemID, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, favorite, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :favorite, :timestamp)`,
				map[string]interface{}{
					"userid":           key.userID,
					"itemid":           key.itemID,
					"position":         value.Position,
					"playedPercentage": value.PlayedPercentage,
					"played":           value.Played,
					"favorite":         value.Favorite,
					"timestamp":        value.Timestamp.UTC(),
				})
			if err != nil {
				return err
			}
		}
	}

	p.lastDBSyncTime = time.Now().UTC()
	return tx.Commit()
}

// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
func (p *UserDataStorage) BackgroundJobs() {
	if p.dbHandle == nil {
		log.Fatal(ErrNoDbHandle)
	}

	p.lastDBSyncTime = time.Now().UTC()

	for {
		if err := p.writeStateToDB(); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(10 * time.Second)
	}
}
