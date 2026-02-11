package jellyfin

import "net/http"

// /SyncPlay/List
//
// syncPlayListHandler lists sync play sessions.
func (j *Jellyfin) syncPlayListHandler(w http.ResponseWriter, r *http.Request) {
	// Currently not implemented, return empty list
	response := []string{}
	serveJSON(response, w)
}

// /SyncPlay/New
//
// syncPlayNewHandler creates a new sync play session.
func (j *Jellyfin) syncPlayNewHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
}
