package database

import (
	"context"
	"fmt"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/database/sqlite"
)

// Database repo aggregates the repo interfaces.
type Repository interface {
	UserRepo
	AccessTokenRepo
	ItemRepo
	UserDataRepo
	PlaylistRepo
	PersonRepo
	ImageRepo
	StartBackgroundJobs(ctx context.Context)
}

// UserRepo defines the interface for user database operations
type UserRepo interface {
	// GetUser retrieves a user.
	GetUser(ctx context.Context, username string) (user *model.User, err error)
	// GetByID retrieves a user from the database by ID.
	GetUserByID(ctx context.Context, userID string) (user *model.User, err error)
	// GetUsers retrieves all users from the database.
	GetAllUsers(ctx context.Context) (users []model.User, err error)
	// UpsertUser upserts a user into the database.
	UpsertUser(ctx context.Context, user *model.User) (err error)
	// DeleteUser deletes a user from the database.
	DeleteUser(ctx context.Context, userID string) error
}

// AccessTokenRepo defines access token operations
type AccessTokenRepo interface {
	// Get accesstoken details by tokenid.
	GetAccessToken(ctx context.Context, token string) (*model.AccessToken, error)
	// Get all access tokens for a user.
	GetAccessTokens(ctx context.Context, userID string) ([]model.AccessToken, error)
	// UpsertAccessToken upserts an access token.
	UpsertAccessToken(ctx context.Context, token model.AccessToken) error
	// DeleteAccessToken upserts an access token.
	DeleteAccessToken(ctx context.Context, token string) error
}

// ItemRepo defines item operations
type ItemRepo interface {
	DbLoadItem(item *model.Item)
}

// UserDataRepo defines play-state operations
type UserDataRepo interface {
	// Get the play state details for an item per user.
	GetUserData(ctx context.Context, userID, itemID string) (details *model.UserData, err error)
	// Get all favorite items of a user.
	GetFavorites(ctx context.Context, userID string) (favoriteItemIDs []string, err error)
	// GetRecentlyWatched returns last 10 watched items that have not been fully watched.
	// If seriesID is provided, it returns all watched items.
	GetRecentlyWatched(ctx context.Context, userID string, count int, includeFullyWatched bool) (resumeItemIDs []string, err error)
	// Update stores the play state details for a user and item.
	UpdateUserData(ctx context.Context, userID, itemID string, details *model.UserData) error
}

// PlaylistRepo defines playlist DB operations
type PlaylistRepo interface {
	CreatePlaylist(ctx context.Context, p model.Playlist) (playlistID string, err error)
	GetPlaylists(ctx context.Context, userID string) (playlistIDs []string, err error)
	GetPlaylist(ctx context.Context, userID, playlistID string) (*model.Playlist, error)
	AddItemsToPlaylist(ctx context.Context, userID, playlistID string, itemIDs []string) error
	DeleteItemsFromPlaylist(ctx context.Context, playlistID string, itemIDs []string) error
	MovePlaylistItem(ctx context.Context, playlistID string, itemID string, newIndex int) error
}

// PersonRepo defines person DB operations
type PersonRepo interface {
	// GetPerson retrieves a person by name.
	GetPersonByName(ctx context.Context, name, userID string) (person *model.Person, err error)
}

type ImageRepo interface {
	// HasImage checks if an image exists for the given itemID and type
	HasImage(ctx context.Context, itemID, imageType string) (model.ImageMetadata, error)
	// GetImage retrieves image data for the given itemID and type
	GetImage(ctx context.Context, itemID, imageType string) (model.ImageMetadata, []byte, error)
	// StoreImage stores image data for the given itemID and type
	StoreImage(ctx context.Context, itemID, imageType string, metadata model.ImageMetadata, data []byte) error
	// DeleteImage deletes an image for the given itemID and type
	DeleteImage(ctx context.Context, itemID, imageType string) error
}

// New creates a new database repository based on the type and options provided.
func New(t string, o any) (Repository, error) {
	switch t {
	case "sqlite":
		switch v := o.(type) {
		case sqlite.ConfigFile:
			return sqlite.New(&v)
		case *sqlite.ConfigFile:
			return sqlite.New(v)
		}
		return nil, fmt.Errorf("invalid config for sqlite database")
	default:
		return nil, fmt.Errorf("unknown database type: %s", t)
	}
}
