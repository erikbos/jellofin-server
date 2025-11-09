package model

import (
	"errors"
	"time"
)

var (
	ErrNoConfiguration = errors.New("database directory not set")
	ErrNoDbHandle      = errors.New("db connection not available")
	ErrNotFound        = errors.New("not found")
	ErrInvalidPassword = errors.New("invalid password")
)

// User represents a user in the system.
type User struct {
	// ID is the unique identifier for the user.
	ID string
	// Username is the username of the user.
	Username string
	// Password is the hashed password of the user.
	Password string
}

// AccessToken represents an access token for a user.
type AccessToken struct {
	UserID   string
	Token    string
	LastUsed time.Time
}

// Item represents a media item in the database.
type Item struct {
	ID         string
	Name       string
	Votes      int
	Genre      string
	Rating     float32
	Year       int
	NfoTime    int64
	FirstVideo int64
	LastVideo  int64
}

// UserData is the structure for storing user play state data.
type UserData struct {
	// Offset in seconds
	Position int64
	// Played playedPercentage
	PlayedPercentage int
	// True if the item has been fully played
	Played bool
	// True if the item is favorite of user
	Favorite bool
	// Timestamp of item playing
	Timestamp time.Time
}

// Playlist represents a user playlist with item IDs.
type Playlist struct {
	// ID is the unique identifier for the playlist.
	ID string
	// UserID is the identifier of the user who owns the playlist.
	UserID string
	// Name of the playlist.
	Name string
	// ItemIDs is a list of item IDs contained in the playlist.
	ItemIDs []string
}
