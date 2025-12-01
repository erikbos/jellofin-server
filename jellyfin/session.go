package jellyfin

import (
	"net/http"

	"github.com/erikbos/jellofin-server/database/model"
)

// /Sessions
//
// sessionsHandler returns a list of active user sessions known to the server.
func (j *Jellyfin) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	// Get all access tokens for this user
	accessTokens, err := j.repo.GetAccessTokens(r.Context(), accessToken.UserID)
	if err != nil {
		apierror(w, "error retrieving sessions", http.StatusInternalServerError)
		return
	}

	// Keep most recent access token per deviceid only, we assume older
	// tokens for the same deviceid are stale.
	uniqueAccessTokens := make(map[string]model.AccessToken)
	for _, t := range accessTokens {
		existing, found := uniqueAccessTokens[t.DeviceId]
		if !found || t.LastUsed.After(existing.LastUsed) {
			uniqueAccessTokens[t.DeviceId] = t
		}
	}

	// Build session list based upon access tokens
	var sessions []JFSessionInfo
	for _, t := range uniqueAccessTokens {
		sessions = append(sessions, *j.makeJFSessionInfo(t, dbuser.Username))
	}
	serveJSON(sessions, w)
}

func (j *Jellyfin) makeJFSessionInfo(accessToken model.AccessToken, username string) *JFSessionInfo {
	s := &JFSessionInfo{
		ID:                    sessionID,
		UserID:                accessToken.UserID,
		UserName:              username,
		LastActivityDate:      accessToken.LastUsed,
		RemoteEndPoint:        accessToken.RemoteAddress,
		DeviceName:            accessToken.DeviceName,
		DeviceID:              accessToken.DeviceId,
		Client:                accessToken.ApplicationName,
		ApplicationVersion:    accessToken.ApplicationVersion,
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
		NowPlayingQueue:          []string{},
		NowPlayingQueueFullItems: []string{},
		SupportedCommands:        []string{},
		PlayableMediaTypes:       []string{},
	}
	return s
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
