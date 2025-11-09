package jellyfin

import (
	"net/http"
	"strings"

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
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	remoteAdress := strings.Split(r.RemoteAddr, ":")[0]

	// Every connected client gets to see the same session details ;-)
	response := j.makeJFSessionInfo(accessToken, dbuser.Username, remoteAdress)

	serveJSON(response, w)
}

func (j *Jellyfin) makeJFSessionInfo(accessToken *model.AccessToken, username, remoteAdress string) *JFSessionInfo {
	s := &JFSessionInfo{
		ID:                    sessionID,
		UserID:                accessToken.UserID,
		UserName:              username,
		LastActivityDate:      accessToken.LastUsed,
		RemoteEndPoint:        remoteAdress,
		DeviceName:            "Client",
		DeviceID:              "Client",
		Client:                "curl",
		ApplicationVersion:    "1.0",
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
