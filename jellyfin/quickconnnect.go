package jellyfin

import (
	"log"
	"net/http"
)

// GET /QuickConnect/Enabled
//
// quickConnectEnabledHandler returns boolean whether quickconnect is enabled.
func (j *Jellyfin) quickConnectEnabledHandler(w http.ResponseWriter, r *http.Request) {
	// For now always return false as quickconnect is not implemented
	serveJSON(false, w)
}

// POST /QuickConnect/Authorize
//
// usersAuthenticateByNameHandler stores quickconnect code of an authenticated user.
func (j *Jellyfin) quickConnectAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	userID := queryparams.Get("userId")
	code := queryparams.Get("code")

	if userID != accessToken.UserID {
		apierror(w, "userID does not match access token", http.StatusForbidden)
		return
	}
	log.Printf("quickConnectAuthorizeHandler: user: %s, code: %s", userID, code)
	apierror(w, "quickconnect code not implemented", http.StatusUnauthorized)
}

// GET /QuickConnect/Connect
//
// quickConnectConnectHandler returns info about a quick connect code.
func (j *Jellyfin) quickConnectConnectHandler(w http.ResponseWriter, r *http.Request) {
	apierror(w, "quickconnect code not found", http.StatusNotFound)
}

// POST /QuickConnect/Initiate
//
// usersAuthenticateByNameHandler authenticates a user by quick connect code.
func (j *Jellyfin) quickConnectInitiateHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("quickConnectInitiateHandler: %+v", r)
	apierror(w, "quickconnect code not implemented", http.StatusUnauthorized)
}
