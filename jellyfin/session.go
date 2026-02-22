package jellyfin

import (
	"net/http"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	// sessionID is a unique ID for authenticated session, it's the same
	// as we do not really track sessions per user
	sessionID = "e3a869b7a901f8894de8ee65688db6c0"
)

// /Sessions
//
// sessionsHandler returns a list of active user sessions known to the server.
func (j *Jellyfin) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	// Get all access tokens for this user
	accessTokens, err := j.repo.GetAccessTokens(r.Context(), reqCtx.User.ID)
	if err != nil {
		apierror(w, "error retrieving sessions", http.StatusInternalServerError)
		return
	}
	// Build session list based upon access tokens
	var sessions []JFSessionInfo
	for _, t := range accessTokens {
		sessions = append(sessions, *j.makeJFSessionInfo(&t, reqCtx.User.Username))
	}
	serveJSON(sessions, w)
}

func (j *Jellyfin) makeJFSessionInfo(accessToken *model.AccessToken, username string) *JFSessionInfo {
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
