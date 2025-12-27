package jellyfin

import "net/http"

// /Movies/Recommendations
//
// moviesRecommendationsHandler returns a list of recommended movie items
func (j *Jellyfin) moviesRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented
	response := []JFItem{}
	serveJSON(response, w)
}
