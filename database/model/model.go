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
	// Properties of the user
	Properties UserProperties
	// Created is the time the user was created.
	Created time.Time
	// LastLogin is the last time the user logged in.
	LastLogin time.Time
	// LastUsed is the last time the user was active.
	LastUsed time.Time
}

// UserProperties represents the properties of a user.
type UserProperties struct {
	// Admin indicates if the user is an administrator.
	Admin bool
	// Disabled indicates if the user account is disabled.
	Disabled bool
	// IsHidden indicates if the user is hidden from other users.
	IsHidden bool
	// EnableDownloads indicates if the user is allowed to download content.
	EnableDownloads bool
	// EnableAllFolders indicates if the user has access to all folders (collections) by default.
	EnableAllFolders bool
	// EnabledFolders is a list of collection item IDs that should be enabled for the user.
	EnabledFolders []string
	// OrderedViews is a list of collection displayPreferenceIDs in the order they should be displayed for the user.
	OrderedViews []string
	// MyMediaExcludes is a list of collection displayPreferenceIDs that should be excluded from the user's personalized view.
	MyMediaExcludes []string
	// AllowTags is a list of tags that are allowed for the user.
	AllowTags []string
	// BlockTags is a list of tags that are blocked for the user.
	BlockTags []string
}

// AccessToken represents an access token for a user.
type AccessToken struct {
	// User is the user associated with the access token.
	User User
	// Token is the access token string.
	Token string
	// DeviceId is the unique identifier for the device.
	DeviceId string
	// DeviceName is the name of the device.
	DeviceName string
	// ApplicationName is the name of the application.
	ApplicationName string
	// ApplicationVersion is the version of the application.
	ApplicationVersion string
	// RemoteAddress is the remote address of the client.
	RemoteAddress string
	// Created is the time the token was created.
	Created time.Time
	// LastUsed is the last time the token was used.
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
	// Play count of the item
	PlayCount int
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

type Person struct {
	// ID is the unique identifier for the person.
	ID string
	// Name is the name of the person.
	Name string
	// DateOfBirth is the birth date of the person.
	DateOfBirth time.Time
	// PlaceOfBirth is the birthplace of the person.
	PlaceOfBirth string
	// PosterURL is the URL to the person's poster image.
	PosterURL string
	// Bio is a short biography of the person.
	Bio string
	// Created is the time the person was created.
	Created time.Time
	// LastUpdated is the last time the person was updated.
	LastUpdated time.Time
}

type ImageMetadata struct {
	// MimeType is the MIME type of the image (e.g., "image/jpeg").
	MimeType string
	// FileSize is the size of the image file in bytes.
	FileSize int
	// Etag is a unique identifier for the image content, used for caching.
	Etag string
	// Updated is the time the image was last received and stored.
	Updated time.Time
}
