package database

import (
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
)

// in-memory access play state store, synced to disk by StartSyncerToDB()

type UserDataStorage struct {
	dbHandle        *sqlx.DB
	mu              sync.Mutex
	userDataEntries map[UserDataKey]UserData
	lastDBSyncTime  time.Time
}

func NewUserDataStorage(d *sqlx.DB) *UserDataStorage {
	p := &UserDataStorage{
		dbHandle:        d,
		userDataEntries: make(map[UserDataKey]UserData),
	}
	err := p.LoadStateFromDB()
	if err != nil {
		log.Printf("Error loading play state from db: %s\n", err)
	}
	return p
}

// UserDataKey is the key for the user data map.
type UserDataKey struct {
	userID string
	itemID string
}

// UserData is the structure for storing user play state data.
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
	ErrUserDataNotFound = errors.New("play state not found")
)

// Get the play state details for an item per user.
func (u *UserDataStorage) Get(userID, itemID string) (details UserData, err error) {
	// log.Printf("UserDataStorageGet: userID: %s, itemID: %s\n", userID, itemID)

	u.mu.Lock()
	defer u.mu.Unlock()

	key := makeKey(userID, itemID)
	if details, ok := u.userDataEntries[key]; ok {
		return details, nil
	}
	err = ErrUserDataNotFound
	return
}

// Update stores the play state details for a user and item.
func (u *UserDataStorage) Update(userID, itemID string, details UserData) (err error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	details.Timestamp = time.Now().UTC()

	// log.Printf("UserDataStorageUpdate: userID: %s, itemID: %s, data: %+v\n", userID, itemID, details)

	key := makeKey(userID, itemID)
	u.userDataEntries[key] = details

	return
}

// GetFavorites returns all favorite items of a user.
func (u *UserDataStorage) GetFavorites(userID string) (favoriteItemIDs []string, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for key, state := range u.userDataEntries {
		if key.userID == userID && state.Favorite {
			favoriteItemIDs = append(favoriteItemIDs, key.itemID)
		}
	}
	return
}

// GetRecentlyWatched returns up to 10 most recently watched items that have not been fully watched.
func (u *UserDataStorage) GetRecentlyWatched(userID string, includeFullyWatched bool) (resumeItemIDs []string, err error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	type resumeItem struct {
		itemID    string
		timestamp time.Time
	}
	var resumeItems []resumeItem

	for key, state := range u.userDataEntries {
		if key.userID == userID {
			// add, if partial watched or fully watched.
			if (!state.Played && state.PlayedPercentage > 0 && state.PlayedPercentage < 100) || includeFullyWatched {
				i := resumeItem{
					itemID:    key.itemID,
					timestamp: state.Timestamp,
				}
				resumeItems = append(resumeItems, i)
			}
		}
	}

	// Sort by timestamp descending
	sort.Slice(resumeItems, func(i, j int) bool {
		return resumeItems[i].timestamp.After(resumeItems[j].timestamp)
	})

	// No need to list all unfinished items of the past, limit to 10 most recent items.
	for i := range min(len(resumeItems), 10) {
		resumeItemIDs = append(resumeItemIDs, resumeItems[i].itemID)
	}
	return
}

// LoadUserDataFromDB loads UserData table into memory.
func (u *UserDataStorage) LoadStateFromDB() error {
	if u.dbHandle == nil {
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

	err := u.dbHandle.Select(&UserDatas, "SELECT * FROM playstate")
	if err != nil {
		return err
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	for _, ps := range UserDatas {
		key := makeKey(ps.UserID, ps.ItemID)
		u.userDataEntries[key] = UserData{
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
func (u *UserDataStorage) writeStateToDB() error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.dbHandle == nil {
		return ErrNoDbHandle
	}

	tx, err := u.dbHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, value := range u.userDataEntries {
		if value.Timestamp.After(u.lastDBSyncTime) {
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

	u.lastDBSyncTime = time.Now().UTC()
	return tx.Commit()
}

// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
func (u *UserDataStorage) BackgroundJobs() {
	if u.dbHandle == nil {
		log.Fatal(ErrNoDbHandle)
	}

	u.lastDBSyncTime = time.Now().UTC()

	for {
		if err := u.writeStateToDB(); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func makeKey(userID, itemID string) UserDataKey {
	return UserDataKey{userID: userID, itemID: itemID}
}
