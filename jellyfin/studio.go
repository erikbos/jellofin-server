package jellyfin

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// /Studios
//
// studiosHandler returns a list of studios for one or all collections.
func (j *Jellyfin) studiosHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	// Get all items for which we need to get studios.
	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")
	items, err := j.getJFItems(r.Context(), accessToken.UserID, parentID)
	if err != nil {
		apierror(w, "Failed to get items", http.StatusInternalServerError)
		return
	}

	// Build unique studios from the items.
	studios := []JFItem{}
	studioSet := make(map[string]struct{})
	for _, item := range items {
		for _, studio := range item.Studios {
			if studio.ID != "" {
				if _, exists := studioSet[studio.ID]; !exists {
					studioSet[studio.ID] = struct{}{}
					if studioItem, err := j.makeJFItemStudio(r.Context(), accessToken.UserID, studio.ID); err == nil {
						studios = append(studios, studioItem)
					}
				}
			}
		}
	}

	studios = j.applyItemSorting(studios, r.URL.Query())

	response := UserItemsResponse{
		Items:            studios,
		TotalRecordCount: len(studios),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Studios/{name}
//
// studioHandler returns details of a specific studio
func (j *Jellyfin) studioHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	studio := vars["name"]
	if studio == "" {
		apierror(w, "Missing studio", http.StatusBadRequest)
		return
	}

	response, err := j.makeJFItemStudio(r.Context(), accessToken.UserID, makeJFStudioID(studio))
	if err != nil {
		apierror(w, "Studio not found", http.StatusNotFound)
		return
	}
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFItemStudio(_ context.Context, _ string, studioID string) (JFItem, error) {
	studio, err := decodeJFStudioID(studioID)
	if err != nil {
		return JFItem{}, err
	}
	response := JFItem{
		ID:                studioID,
		ServerID:          j.serverID,
		Type:              itemTypeStudio,
		Name:              studio,
		SortName:          studio,
		Etag:              studioID,
		DateCreated:       time.Now().UTC(),
		PremiereDate:      time.Now().UTC(),
		LocationType:      "FileSystem",
		MediaType:         "Unknown",
		ImageBlurHashes:   &JFImageBlurHashes{},
		ImageTags:         &JFImageTags{},
		BackdropImageTags: []string{},
		UserData:          &JFUserData{},
		LockedFields:      []string{},
	}
	return response, nil
}
