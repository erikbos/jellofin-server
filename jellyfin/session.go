package jellyfin

import (
	"net/http"
	"strings"
)

// /Sessions
//
// sessionsHandler returns a list of active user sessions known to the server.
func (j *Jellyfin) sessionsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	dbuser, err := j.db.UserRepo.GetByID(accessToken.UserID)
	if err != nil {
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	remoteAdress := strings.Split(r.RemoteAddr, ":")

	// Every connected client gets to see the same session details ;-)
	response := JFSessionResponse{
		ID:                    "ac8900873f2a7b460877e426f37b4d06",
		UserID:                accessToken.UserID,
		UserName:              dbuser.Username,
		LastActivityDate:      accessToken.LastUsed,
		RemoteEndPoint:        remoteAdress[0],
		DeviceName:            "Client",
		DeviceID:              "Client",
		Client:                "curl",
		ApplicationVersion:    "1.0",
		IsActive:              true,
		SupportsMediaControl:  false,
		SupportsRemoteControl: false,
		HasCustomDeviceName:   false,
		ServerID:              serverID,
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
	serveJSON(response, w)
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
