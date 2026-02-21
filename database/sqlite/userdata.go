package sqlite

import (
	"context"
	"log"
	"sort"
	"time"

	"github.com/jmoiron/sqlx"

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

	const query = `SELECT
	position,
	playedpercentage,
	played,
	favorite,
	timestamp
FROM playstate WHERE userid = ? AND itemid = ?`
	row := s.dbReadHandle.QueryRowContext(ctx, query, userID, itemID)
	var i model.UserData
	err := row.Scan(
		&i.Position,
		&i.PlayedPercentage,
		&i.Played,
		&i.Favorite,
		&i.Timestamp,
	)
	if err != nil {
		log.Printf("Error retrieving play state from db for userID: %s, itemID: %s: %s\n", userID, itemID, err)
	}
	return &i, err
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

// GetRecentlyWatched returns last 10 watched items that have not been fully watched.
// If seriesID is provided, it returns all watched items.
func (s *SqliteRepo) GetRecentlyWatched(ctx context.Context, userID string, count int, includeFullyWatched bool) ([]string, error) {
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
	for i := range min(len(resumeItems), count) {
		resumeItemIDs = append(resumeItemIDs, resumeItems[i].itemID)
	}
	return resumeItemIDs, nil
}

// loadUserDataFromDB loads UserData table into memory.
func (s *SqliteRepo) loadUserDataFromDB() error {
	if s.dbReadHandle == nil {
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

	if err := s.dbReadHandle.Select(&UserDatas, "SELECT userid, itemid, position, playedpercentage, played, favorite, timestamp FROM playstate"); err != nil {
		// log.Printf("Error loading play state from db: %s\n", err)
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

// userDataBackgroundJob loads state and writes changed play state to database every 3 seconds.
func (s *SqliteRepo) userDataBackgroundJob(ctx context.Context, interval time.Duration) {
	if s.dbReadHandle == nil || s.dbWriteHandle == nil {
		log.Fatal(model.ErrNoDbHandle)
	}

	for {
		if err := s.writeChangedUserDataToDB(ctx); err != nil {
			log.Printf("Error writing play state to db: %s\n", err)
		}
		time.Sleep(interval)
	}
}

// writeUserDataToDB writes all update userdata entries to db.
func (s *SqliteRepo) writeChangedUserDataToDB(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.dbWriteHandle.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for k, userdata := range s.userDataEntries {
		if userdata.Timestamp.After(s.userDataEntriesCacheSyncTime) {
			if err := s.storeUserData(ctx, tx, k.userID, k.itemID, userdata); err != nil {
				return err
			}
		}
	}
	// Update sync time so we only write changed entries next time
	s.userDataEntriesCacheSyncTime = time.Now().UTC()
	return tx.Commit()
}

func (s *SqliteRepo) storeUserData(ctx context.Context, tx *sqlx.Tx, userID, itemID string, data model.UserData) error {
	const query = `REPLACE INTO playstate (
		userid,
		itemid,
		position,
		playedpercentage,
		played,
		favorite,
		timestamp) VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := tx.ExecContext(ctx, query,
		userID,
		itemID,
		data.Position,
		data.PlayedPercentage,
		data.Played,
		data.Favorite,
		data.Timestamp.UTC(),
	)
	return err
	// if err != nil {
	// 	log.Printf("Error storing play state to db for userID: %s, itemID: %s: %s\n", userID, itemID, err)
	// }
	// return err
}

func makeUserDataCacheKey(userID, itemID string) userDataKey {
	return userDataKey{userID: userID, itemID: itemID}
}
