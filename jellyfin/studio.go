package jellyfin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
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
	items, err := j.getJFItems(r.Context(), accessToken.User.ID, parentID)
	if err != nil {
		apierror(w, "Failed to get items", http.StatusInternalServerError)
		return
	}

	// Build unique studios from the items.
	studios := []JFItem{}
	studioSet := make(map[string]struct{})
	for _, item := range items {
		for _, studio := range item.Studios {
			if studio.Name != "" {
				if _, exists := studioSet[studio.ID]; !exists {
					studioSet[studio.ID] = struct{}{}
					if studioItem, err := j.makeJFItemStudio(r.Context(), accessToken.User.ID, studio.ID); err == nil {
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
	var err error
	studio, err = url.PathUnescape(studio)
	if err != nil {
		apierror(w, "Invalid studio name", http.StatusBadRequest)
		return
	}
	response, err := j.makeJFItemStudio(r.Context(), accessToken.User.ID, makeJFStudioID(studio))
	if err != nil {
		apierror(w, "Studio not found", http.StatusNotFound)
		return
	}
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFItemStudio(ctx context.Context, _ string, studioID string) (JFItem, error) {
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
		ImageTags:         j.makeJFImageTags(ctx, studioID, imageTypePrimary),
		BackdropImageTags: []string{},
		UserData:          &JFUserData{},
		LockedFields:      []string{},
	}
	return response, nil
}

// makeJFStudios converts a list of studio names to a list of JFStudios with IDs and names.
func makeJFStudios(studios []string) []JFStudios {
	studioItems := make([]JFStudios, 0, len(studios))
	for _, studio := range studios {
		if studio != "" {
			studioItems = append(studioItems, JFStudios{ID: makeJFStudioID(studio), Name: studio})
		}
	}
	return studioItems
}

// makeJFStudioID returns an external id for a studio.
func makeJFStudioID(studioName string) string {
	return encodeExternalName(itemprefix_studio, studioName)
}

// isJFStudioID checks if the provided ID is a studio ID.
func isJFStudioID(id string) bool {
	return strings.HasPrefix(id, itemprefix_studio)
}

// decodeJFStudioID decodes a studio ID to get the original name.
func decodeJFStudioID(studioID string) (string, error) {
	return decodeExternalName(itemprefix_studio, studioID)
}
