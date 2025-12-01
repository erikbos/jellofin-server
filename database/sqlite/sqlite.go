package sqlite

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"

	"github.com/erikbos/jellofin-server/database/model"
)

type SqliteRepo struct {
	// Read db handle
	dbReadHandle *sqlx.DB
	// Handle specfically for writes
	dbWriteHandle *sqlx.DB
	// in-memory access token store, entries written to the database every 3 seconds.
	accessTokenCache map[string]*model.AccessToken
	// last time the access token cache was synced to the database
	accessTokenCacheSyncTime time.Time
	// in-memory user data store, entries are written to the database every 3 seconds.
	userDataEntries map[userDataKey]model.UserData
	// last time the user data entries were synced to the database
	userDataEntriesCacheSyncTime time.Time
	// mutex to protect access to in-memory stores
	mu sync.Mutex
}

// ConfigFile holds configuration options
type ConfigFile struct {
	Filename string `yaml:"filename"`
}

// New initializes a sqlite database and creates schema if necssary.
func New(o *ConfigFile) (*SqliteRepo, error) {
	if o == nil || o.Filename == "" {
		return nil, fmt.Errorf("database filename not set")
	}

	dbHandle, err := sqlx.Connect("sqlite3", o.Filename)
	if err != nil {
		return nil, err
	}
	dbHandle.SetMaxOpenConns(max(4, runtime.NumCPU()))

	writeDB, err := sqlx.Connect("sqlite3", o.Filename)
	if err != nil {
		return nil, err
	}
	// sqlite needs to have a single writer
	writeDB.SetMaxOpenConns(1)

	if err := dbInitSchema(writeDB); err != nil {
		return nil, err
	}

	d := &SqliteRepo{
		dbReadHandle:     dbHandle,
		dbWriteHandle:    writeDB,
		userDataEntries:  make(map[userDataKey]model.UserData),
		accessTokenCache: make(map[string]*model.AccessToken),
	}

	d.loadUserDataFromDB()

	return d, nil
}

// StartBackgroundJobs starts background jobs for the database repository.
// these jobs handle periodic syncing of in-memory caches to the database.
func (s *SqliteRepo) StartBackgroundJobs(ctx context.Context) {
	syncInterval := 10 * time.Second

	go s.accessTokenBackgroundJob(ctx, syncInterval)
	go s.userDataBackgroundJob(ctx, syncInterval)
}
