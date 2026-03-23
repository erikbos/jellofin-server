package jellyfin

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

// /Sessions
//
// sessionsHandler returns a list of active user sessions known to the server.
func (j *Jellyfin) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	queryParams := r.URL.Query()
	filterDeviceID := queryParams.Get("deviceId")
	activeWithinSeconds, err := strconv.Atoi(queryParams.Get("activeWithinSeconds"))
	if err != nil {
		activeWithinSeconds = 0
	}

	sessions := j.sessionTable.GetAll()
	response := make([]JFSessionInfo, 0, len(sessions))
	for _, s := range sessions {
		// Filter by deviceId if provided
		if filterDeviceID != "" && s.DeviceID != filterDeviceID {
			continue
		}

		// Skip sessions that haven't been active within the specified time frame
		if activeWithinSeconds != 0 &&
			time.Since(s.UpdatedAt) > time.Duration(activeWithinSeconds)*time.Second {
			continue
		}
		response = append(response, *j.makeJFSessionInfo(r.Context(), &s))
	}
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFSessionInfo(ctx context.Context, s *sessionEntry) *JFSessionInfo {
	jfsession := &JFSessionInfo{
		ID:                    s.ID,
		UserID:                s.UserID,
		LastActivityDate:      s.UpdatedAt,
		RemoteEndPoint:        s.RemoteAddress,
		DeviceName:            s.DeviceName,
		DeviceID:              s.DeviceID,
		Client:                s.ApplicationName,
		ApplicationVersion:    s.ApplicationVersion,
		IsActive:              true,
		SupportsMediaControl:  false,
		SupportsRemoteControl: false,
		HasCustomDeviceName:   false,
		ServerID:              j.serverID,
		AdditionalUsers:       []string{},
		PlayState: JFSessionResponsePlayState{
			RepeatMode:    "RepeatNone",
			PlaybackOrder: "Default",
		},
		Capabilities: JFSessionResponseCapabilities{
			PlayableMediaTypes:           []string{},
			SupportedCommands:            []string{},
			SupportsPersistentIdentifier: true,
		},
		NowPlayingQueue:          []JFSessionNowPlayingQueueItem{},
		NowPlayingQueueFullItems: []JFItem{},
		SupportedCommands:        []string{},
		PlayableMediaTypes:       []string{},
	}

	dbuser, err := j.repo.GetUserByID(ctx, s.UserID)
	if err == nil {
		jfsession.UserName = dbuser.Username
		jfsession.LastActivityDate = dbuser.LastUsed
	}

	// Set imagetag if user has an image
	if _, err := j.repo.HasImage(ctx, s.UserID, imageTypeProfile); err == nil {
		jfsession.UserPrimaryImageTag = s.UserID
	}

	// If the session is currently playing an item add its details
	if s.ItemID != "" {
		// Add full item details
		if item, err := j.makeJFItemByID(ctx, s.UserID, s.ItemID); err == nil {
			jfsession.PlayState = JFSessionResponsePlayState{
				AudioStreamIndex:    1,
				CanSeek:             true,
				IsMuted:             false,
				IsPaused:            s.IsPaused,
				MediaSourceID:       s.ItemID,
				PlayMethod:          "DirectPlay",
				PlaybackOrder:       "Default",
				RepeatMode:          "RepeatNone",
				SubtitleStreamIndex: -1,
				VolumeLevel:         100,
			}

			jfsession.NowPlayingItem = &item
			jfsession.NowPlayingQueueFullItems = []JFItem{item}

			playlistID := "playlistItem0"
			jfsession.PlaylistItemID = playlistID
			jfsession.NowPlayingQueue = []JFSessionNowPlayingQueueItem{
				{
					ID:             s.ItemID,
					PlaylistItemID: playlistID,
				},
			}

			if item.UserData != nil && !item.UserData.LastPlayedDate.IsZero() {
				jfsession.PlayState.PositionTicks = item.UserData.PlaybackPositionTicks
				jfsession.LastPlaybackCheckIn = item.UserData.LastPlayedDate
			}

		}
	}
	return jfsession
}

// /Sessions/Capabilities
//
// sessionsCapabilitiesHandler accepts the capabilities of the client. Ignored.
func (j *Jellyfin) sessionsCapabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Capabilities/Full
//
// sessionsCapabilitiesFullHandler accepts the capabilities of the client. Ignored.
func (j *Jellyfin) sessionsCapabilitiesFullHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// //////////////////////////////////////////////////////////////////////////////////////////
//
// in memory for now
//

// sessionEntry represents the value stored in the session table
type sessionEntry struct {
	// ID is a unique identifier for the session
	ID string
	// UserID is the ID of the user associated with the session
	UserID string
	// DeviceID is the unique identifier for the device.
	DeviceID string
	// DeviceName is the name of the device.
	DeviceName string
	// ApplicationName is the name of the application.
	ApplicationName string
	// ApplicationVersion is the version of the application.
	ApplicationVersion string
	// RemoteAddress is the remote address of the client.
	RemoteAddress string
	// ItemID is the ID of the media item being played
	ItemID string
	// IsPaused indicates whether the session is currently paused
	IsPaused bool
	// CreatedAt is the timestamp when the session entry was created
	CreatedAt time.Time
	// UpdatedAt is the timestamp when the session entry was last updated
	UpdatedAt time.Time
}

// SessionTable maintains an in-memory table of user sessions
type SessionTable struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
}

// NewSessionTable creates a new instance of SessionTable
func NewSessionTable() *SessionTable {
	return &SessionTable{
		sessions: make(map[string]sessionEntry),
	}
}

// GetAll returns all session entries in the table
// Returns a copy of all entries to avoid external modification
func (st *SessionTable) GetAll() []sessionEntry {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Create a copy of the map to return
	result := make([]sessionEntry, 0, len(st.sessions))
	for _, entry := range st.sessions {
		result = append(result, entry)
	}
	return result
}

// Create adds or updates a session entry based upon the provided access token
func (st *SessionTable) Create(t *model.AccessToken) (*sessionEntry, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	sessionID := makeSessionTableKey(t.UserID, t.DeviceID)
	session, exists := st.sessions[sessionID]
	if !exists {
		// Create a new session entry
		session = sessionEntry{
			ID:        sessionID,
			UserID:    t.UserID,
			DeviceID:  t.DeviceID,
			CreatedAt: time.Now().UTC(),
		}
	}

	// Always update for new and existing sessions to capture latest details and activity
	session.DeviceName = t.DeviceName
	session.ApplicationName = t.ApplicationName
	session.ApplicationVersion = t.ApplicationVersion
	session.RemoteAddress = t.RemoteAddress
	session.UpdatedAt = time.Now().UTC()

	st.sessions[session.ID] = session

	// log.Printf("Created session %s, details %+v", session.ID, session)
	return &session, nil
}

// UpdateStatus stores or updates a session entry for the given userID and deviceID
func (st *SessionTable) UpdateStatus(userID, username, itemID string, isPaused bool) bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	sessionID := makeSessionTableKey(userID, username)
	session, exists := st.sessions[sessionID]
	if !exists {
		return false
	}

	session.ItemID = itemID
	session.IsPaused = isPaused
	session.UpdatedAt = time.Now().UTC()
	st.sessions[sessionID] = session

	// log.Printf("Updated session %s, details %+v", sessionID, session)

	return true
}

// Remove deletes a session entry based on userID and deviceID
// Returns true if the entry was found and removed, false otherwise
func (st *SessionTable) Remove(userID, username string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()

	sessionID := makeSessionTableKey(userID, username)
	if _, exists := st.sessions[sessionID]; exists {
		delete(st.sessions, sessionID)

		log.Printf("Removed session %s", sessionID)
		return true
	}
	return false
}

func makeSessionTableKey(userID, deviceID string) string {
	return idhash.Hash(userID + "/" + deviceID)
}
