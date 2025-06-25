package jellyfin

import (
	"net/http"
	"slices"
	"strings"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/idhash"
)

// /Genres
//
// genresHandler returns a list of genres for one or all collections.
func (j *Jellyfin) genresHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var details collection.CollectionDetails
	if collection := r.URL.Query().Get("parentId"); collection != "" {
		// Not every collection has genres (e.g. dynamic collections such as playlists or favorites)
		if c := j.collections.GetCollection(strings.TrimPrefix(collection, itemprefix_collection)); c != nil {
			details = c.Details()
		}
	} else {
		details = j.collections.Details()
	}

	genres := []JFItem{}
	for _, g := range details.Genres {
		genres = append(genres, j.makeJFItemGenre(r.Context(), g))
	}

	response := UserItemsResponse{
		Items:            genres,
		TotalRecordCount: len(genres),
		StartIndex:       0,
	}

	serveJSON(response, w)
}

// /Genres/Thriller
//
// genreHandler returns details of a specific genre
func (j *Jellyfin) genreHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	genreParam := vars["genre"]
	if genreParam == "" {
		http.Error(w, "Missing genre", http.StatusBadRequest)
		return
	}

	for _, genre := range j.collections.Details().Genres {
		if genre == genreParam {
			response := j.makeJFItemGenre(r.Context(), genre)
			serveJSON(response, w)
			return
		}
	}
	http.Error(w, "Genre not found", http.StatusNotFound)
}

// /Items/Filters?userId=XAOVnIQY8sd0&parentId=collection_1
//
// usersItemsFiltersHandler returns a list of genre filter values
func (j *Jellyfin) usersItemsFiltersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var details collection.CollectionDetails
	if collection := r.URL.Query().Get("parentId"); collection != "" {
		// Not every collection has genres (e.g. dynamic collections such as playlists or favorites)
		if c := j.collections.GetCollection(strings.TrimPrefix(collection, itemprefix_collection)); c != nil {
			details = c.Details()
		}
	} else {
		details = j.collections.Details()
	}

	slices.Sort(details.Years)

	response := JFItemFilterResponse{
		Genres:          details.Genres,
		Tags:            details.Tags,
		OfficialRatings: details.OfficialRatings,
		Years:           details.Years,
	}
	serveJSON(response, w)
}

// /Items/Filters2?userId=XAOVnIQY8sd0&parentId=collection_1
//
// usersItemsFilters2Handler returns a list of genre name and their id.
func (j *Jellyfin) usersItemsFilters2Handler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var details collection.CollectionDetails
	if searchCollection := r.URL.Query().Get("parentId"); searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		details = j.collections.GetCollection(collectionid).Details()
	} else {
		details = j.collections.Details()
	}

	response := JFItemFilter2Response{
		Genres: makeJFGenreItems(details.Genres),
		Tags:   details.Tags,
	}
	serveJSON(response, w)

}

func makeJFGenreItems(array []string) (genreItems []JFGenreItem) {
	for _, v := range array {
		genreItems = append(genreItems, JFGenreItem{
			Name: v,
			ID:   idhash.IdHash(v),
		})
	}
	return genreItems
}
