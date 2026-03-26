package jellyfin

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/mux"
)

// /Artists
//
// artistsHandler returns a list of artists
func (j *Jellyfin) artistsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	//
	// Not implemented but some clients like to call it when searching.. so we return an empty list instead of an error.
	//
	artists := []JFItem{}
	response := UserItemsResponse{
		Items:            artists,
		StartIndex:       0,
		TotalRecordCount: len(artists),
	}
	serveJSON(response, w)
}

// /Artists/{name}
//
// artistHandler returns details of a specific artist
func (j *Jellyfin) artistHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		apierror(w, "Missing artist name", http.StatusBadRequest)
		return
	}
	name, err := url.QueryUnescape(name)
	if err != nil {
		apierror(w, "Invalid artist name", http.StatusBadRequest)
		return
	}
	response, err := j.makeJFItemArtist(r.Context(), reqCtx.User.ID, makeJFArtistID(name))
	if err != nil {
		apierror(w, "could not create artist item", http.StatusInternalServerError)
		return
	}
	serveJSON(response, w)
}

// makeJFItemArtist creates a JFItem representing an artist
func (j *Jellyfin) makeJFItemArtist(ctx context.Context, userID string, artistID string) (JFItem, error) {
	name, err := decodeJFArtistID(artistID)
	if err != nil {
		return JFItem{}, err
	}
	response := JFItem{
		ID:                  artistID,
		ServerID:            j.serverID,
		Type:                itemTypePerson,
		Name:                name,
		SortName:            makeSortName(name),
		Etag:                artistID,
		LocationType:        "FileSystem",
		MediaType:           "Unknown",
		PlayAccess:          "Full",
		ProductionLocations: []string{},
		ImageBlurHashes:     &JFImageBlurHashes{},
		BackdropImageTags:   []string{},
		People:              []JFPeople{},
		Studios:             []JFStudios{},
		Genres:              []string{},
		GenreItems:          []JFGenreItem{},
		LockedFields:        []string{},
		Taglines:            []string{},
		Tags:                []string{},
		UserData: &JFUserData{
			Key:    "Artist-" + name,
			ItemID: artistID,
		},
		// Given an item trigger a request for this artist, we assume this artist was involved in at least one item.
		ChildCount: 1,
	}

	if playstate, err := j.repo.GetUserData(ctx, userID, artistID); err == nil {
		response.UserData = j.makeJFUserData(userID, artistID, playstate)
	} else {
		response.UserData = j.makeJFUserData(userID, artistID, nil)
	}
	return response, nil
}

// makeJFArtistID returns an external id for an artist.
func makeJFArtistID(name string) string {
	return encodeExternalName(itemprefix_artist, name)
}

// isJFArtistID checks if the provided ID is an artist ID.
func isJFArtistID(id string) bool {
	return strings.HasPrefix(id, itemprefix_artist)
}

// decodeJFArtistID decodes an artist ID to get the original name.
func decodeJFArtistID(artistID string) (string, error) {
	return decodeExternalName(itemprefix_artist, artistID)
}
