package jellyfin

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
)

// studiosHandler returns a list of studios for one or all collections.
func (j *Jellyfin) studiosHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var details collection.CollectionDetails
	if collection := r.URL.Query().Get("parentId"); collection != "" {
		// Not every collection has studios (e.g. dynamic collections such as playlists or favorites)
		if c := j.collections.GetCollection(strings.TrimPrefix(collection, itemprefix_collection)); c != nil {
			details = c.Details()
		}
	} else {
		details = j.collections.Details()
	}

	studios := []JFItem{}
	for _, s := range details.Studios {
		studios = append(studios, j.makeJFItemStudio(r.Context(), s))
	}

	response := UserItemsResponse{
		Items:            studios,
		TotalRecordCount: len(studios),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// studioHandler returns details of a specific studio
func (j *Jellyfin) studioHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	studio := vars["studio"]
	if studio == "" {
		apierror(w, "Missing studio", http.StatusBadRequest)
		return
	}

	response := j.makeJFItemStudio(r.Context(), studio)
	serveJSON(response, w)
}
