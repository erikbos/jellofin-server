package jellyfin

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	ErrUserIDNotFound = "userid not found"
)

// GET /Users
//
// usersAllHandler returns all users, we return only the current user
func (j *Jellyfin) usersAllHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err != nil {
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	response := []JFUser{
		j.makeJFUser(dbuser),
	}
	serveJSON(response, w)
}

// GET /Users/Me
//
// usersMeHandler returns the current user
func (j *Jellyfin) usersMeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err != nil {
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	response := j.makeJFUser(dbuser)
	serveJSON(response, w)
}

// GET /Users/{user}
//
// usersHandler returns a user, we always return the current user
func (j *Jellyfin) usersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	if vars["user"] != accessToken.UserID {
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}

	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err != nil {
		http.Error(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	response := j.makeJFUser(dbuser)
	serveJSON(response, w)
}

// GET /Users/Public
//
// usersHandler returns list of public users, none
func (j *Jellyfin) usersPublicHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFUser{}
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFUser(user *model.User) JFUser {
	return JFUser{
		Id:                        user.ID,
		Name:                      user.Username,
		ServerId:                  j.serverID,
		HasPassword:               true,
		HasConfiguredPassword:     true,
		HasConfiguredEasyPassword: false,
		EnableAutoLogin:           false,
		LastLoginDate:             time.Now().UTC(),
		LastActivityDate:          time.Now().UTC(),
		Configuration: JFUserConfiguration{
			GroupedFolders:             []string{},
			LatestItemsExcludes:        []string{},
			MyMediaExcludes:            []string{},
			OrderedViews:               []string{},
			SubtitleMode:               "Default",
			PlayDefaultAudioTrack:      true,
			RememberAudioSelections:    true,
			RememberSubtitleSelections: true,
		},
		Policy: JFUserPolicy{
			// Needs to be true to allow Streamyfin to Cast
			IsAdministrator: true,
			// Checked by Streamyfin to permit download
			EnableContentDownloading:         true,
			AccessSchedules:                  []string{},
			AllowedTags:                      []string{},
			BlockedChannels:                  []string{},
			BlockedMediaFolders:              []string{},
			BlockedTags:                      []string{},
			BlockUnratedItems:                []string{},
			EnabledChannels:                  []string{},
			EnabledDevices:                   []string{},
			EnabledFolders:                   []string{},
			EnableContentDeletionFromFolders: []string{},
			EnableMediaPlayback:              true,
			EnableRemoteAccess:               true,
			EnableAllDevices:                 true,
			EnableAllFolders:                 true,
			AuthenticationProviderID:         "DefaultAuthenticationProvider",
			PasswordResetProviderID:          "DefaultPasswordResetProvider",
			SyncPlayAccess:                   "CreateAndJoinGroups",
		},
	}
}
