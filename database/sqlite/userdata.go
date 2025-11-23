package sqlite

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
)

// userDataKey is the key for the user data map.
type userDataKey struct {
	userID string
	itemID string
}

// Get the play state details for an item per user.
func (s *SqliteRepo) GetUserData(ctx context.Context, userID, itemID string) (*model.UserData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// log.Printf("SqliteRepoGet: userID: %s, itemID: %s\n", userID, itemID)

	key := makeUserDataCacheKey(userID, itemID)
	if details, ok := s.userDataEntries[key]; ok {
		return &details, nil
	}
	return nil, model.ErrNotFound
}

// Get the play state details for an item per user.
func (s *SqliteRepo) GetUserData2(ctx context.Context, userID, itemID string) (*model.UserData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var UserData struct {
		UserID           string    `db:"userid"`
		ItemID           string    `db:"itemid"`
		Position         int64     `db:"position"`
		PlayedPercentage int       `db:"playedpercentage"`
		Played           bool      `db:"played"`
		Favorite         bool      `db:"favorite"`
		Timestamp        time.Time `db:"timestamp"`
	}

	err := s.dbHandle.Get(&UserData, "SELECT userid, itemid, position, playedPercentage, played, favorite, timestamp FROM playstate WHERE userid = ? AND itemid = ?", userID, itemID)
	if err != nil {
		// log.Printf("SqliteRepoGet: userID: %s, itemID: %s, %+v\n", userID, itemID, err)
		return nil, err
	}

	// log.Printf("SqliteRepoGet: userID: %s, itemID: %s, %+v\n", userID, itemID, UserData)

	return &model.UserData{
		Position:         UserData.Position,
		PlayedPercentage: UserData.PlayedPercentage,
		Played:           UserData.Played,
		Favorite:         UserData.Favorite,
		Timestamp:        UserData.Timestamp,
	}, nil
}

// Update stores the play state details for a user and item.
func (s *SqliteRepo) UpdateUserData(ctx context.Context, userID, itemID string, details *model.UserData) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	details.Timestamp = time.Now().UTC()

	// log.Printf("SqliteRepoUpdate: userID: %s, itemID: %s, data: %+v\n", userID, itemID, details)

	key := makeUserDataCacheKey(userID, itemID)
	s.userDataEntries[key] = *details

	return nil
}

// GetFavorites returns all favorite items of a user.
func (s *SqliteRepo) GetFavorites(ctx context.Context, userID string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var favoriteItemIDs []string
	for key, state := range s.userDataEntries {
		if key.userID == userID && state.Favorite {
			favoriteItemIDs = append(favoriteItemIDs, key.itemID)
		}
	}
	return favoriteItemIDs, nil
}

// GetRecentlyWatched returns up to 10 most recently watched items that have not been fully watched.
func (s *SqliteRepo) GetRecentlyWatched(ctx context.Context, userID string, includeFullyWatched bool) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	type resumeItem struct {
		itemID    string
		timestamp time.Time
	}
	var resumeItems []resumeItem

	for key, state := range s.userDataEntries {
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
	var resumeItemIDs []string
	for i := range min(len(resumeItems), 10) {
		resumeItemIDs = append(resumeItemIDs, resumeItems[i].itemID)
	}
	return resumeItemIDs, nil
}

// LoadUserDataFromDB loads UserData table into memory.
func (s *SqliteRepo) LoadStateFromDB() error {
	if s.dbHandle == nil {
		return model.ErrNoDbHandle
	}

	var UserDatas []struct {
		UserID           string    `db:"userid"`
		ItemID           string    `db:"itemid"`
		Position         int64     `db:"position"`
		PlayedPercentage int       `db:"playedpercentage"`
		Played           bool      `db:"played"`
		Favorite         bool      `db:"favorite"`
		Timestamp        time.Time `db:"timestamp"`
	}

	err := s.dbHandle.Select(&UserDatas, "SELECT userid, itemid, position, playedPercentage, played, favorite, timestamp FROM playstate")
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, ps := range UserDatas {
		key := makeUserDataCacheKey(ps.UserID, ps.ItemID)
		s.userDataEntries[key] = model.UserData{
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
func (s *SqliteRepo) writeStateToDB() error {
	if s.dbHandle == nil {
		return model.ErrNoDbHandle
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.dbHandle.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for key, value := range s.userDataEntries {
		if value.Timestamp.After(s.userDataEntriesCacheSyncTime) {
			// log.Printf("Persisting play state for user %s, item %s, details: %+v\n", key.userID, key.itemID, value)
			_, err := tx.NamedExec(`INSERT OR REPLACE INTO playstate (userid, itemid, position, playedPercentage, played, favorite, timestamp)
                VALUES (:userid, :itemid, :position, :playedPercentage, :played, :favorite, :timestamp)`,
				map[string]any{
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

	s.userDataEntriesCacheSyncTime = time.Now().UTC()
	return tx.Commit()
}

// BackgroundJobs loads state and writes changed play state to database every 3 seconds.
func (s *SqliteRepo) UserDataBackgroundJobs() {
	if s.dbHandle == nil {
		log.Fatal(model.ErrNoDbHandle)
	}

	s.userDataEntriesCacheSyncTime = time.Now().UTC()

	for {
		if err := s.writeStateToDB(); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(10 * time.Second)
	}
}

func makeUserDataCacheKey(userID, itemID string) userDataKey {
	return userDataKey{userID: userID, itemID: itemID}
}
