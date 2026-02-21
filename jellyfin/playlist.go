package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

// POST /Playlists
//
// createPlaylistHandler creates a new playlist
func (j *Jellyfin) createPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var req JFCreatePlaylistRequest

	queryparams := r.URL.Query()
	req.Name = queryparams.Get("name")
	req.UserID = queryparams.Get("userId")
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror(w, ErrInvalidJSONPayload, http.StatusBadRequest)
			return
		}
	}
	if req.Name == "" || req.UserID == "" {
		apierror(w, "Name and UserId are required", http.StatusBadRequest)
		return
	}

	newPlaylist := model.Playlist{
		Name:   req.Name,
		UserID: req.UserID,
	}
	if req.Ids != nil {
		newPlaylist.ItemIDs = req.Ids
	} else {
		for i := range strings.SplitSeq(queryparams.Get("Ids"), ",") {
			newPlaylist.ItemIDs = append(newPlaylist.ItemIDs, trimPrefix(i))
		}
	}
	// log.Printf("newPlaylist: %+v", newPlaylist)

	playlistID, err := j.repo.CreatePlaylist(r.Context(), newPlaylist)
	log.Printf("playlistID: %s, err: %v", playlistID, err)
	if err != nil {
		apierror(w, "Failed to create playlist", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	serveJSON(&JFCreatePlaylistResponse{
		Id: itemprefix_playlist + playlistID,
	}, w)
}

// POST /Playlists/{playlistId}
//
// updatePlaylistHandler updates a playlist
func (j *Jellyfin) updatePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// playlistID := vars["playlistid"]

	// var req struct {
	// 	Name string `json:"Name"`
	// }

	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	// 	http.Error(w, "Invalid request body", http.StatusBadRequest)
	// 	return
	// }

	// // Note: Since Jellyfin API typically creates new playlists rather than updating,
	// // this endpoint might be used for updating playlist metadata
	// _, err := j.db..GetPlaylist(playlistID)
	// if err != nil {
	// 	http.Error(w, "Playlist not found", http.StatusNotFound)
	// 	return
	// }

	// w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}
//
// getPlaylistHandler retrieves a playlist by ID
func (j *Jellyfin) getPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	playlist, err := j.repo.GetPlaylist(r.Context(), accessToken.User.ID, trimPrefix(playlistID))
	// log.Printf("querying playlist: %+v, %+v\n", playlist, err)
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}

	response := JFGetPlaylistResponse{
		OpenAccess: false,
		Shares:     []string{},
		ItemIds:    playlist.ItemIDs,
	}
	serveJSON(response, w)
}

// GET /Playlists/{playlistId}/Items
//
// getPlaylistItemsHandler retrieves items in a playlist
func (j *Jellyfin) getPlaylistItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	playlist, err := j.repo.GetPlaylist(r.Context(), accessToken.User.ID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}

	items := []JFItem{}
	for _, itemID := range playlist.ItemIDs {
		c, i := j.collections.GetItemByID(itemID)
		if c != nil || i != nil {
			jfitem, err := j.makeJFItem(r.Context(), accessToken.User.ID, i, idhash.IdHash(c.Name))
			if err != nil {
				apierror(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, jfitem)
		}
	}
	response := UserItemsResponse{
		Items:            items,
		TotalRecordCount: len(items),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// POST /Playlists/{playlistId}/Items
//
// addPlaylistItemsHandler Adds items to a playlist
func (j *Jellyfin) addPlaylistItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	playlistID := vars["playlistid"]
	queryparams := r.URL.Query()

	var itemIDs []string
	for ID := range strings.SplitSeq(queryparams.Get("Ids"), ",") {
		itemIDs = append(itemIDs, trimPrefix(ID))
	}

	if err := j.repo.AddItemsToPlaylist(r.Context(), accessToken.User.ID, trimPrefix(playlistID), itemIDs); err != nil {
		apierror(w, "Failed to add items", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}/Items/{itemId}/Move/{newIndex}
//
// movePlaylistItemHandler moves an item in a playlist
func (j *Jellyfin) movePlaylistItemHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// playlistID := vars["playlistid"]
	// itemID := vars["itemId"]
	// newIndex, err := strconv.Atoi(vars["newIndex"])
	// if err != nil {
	// 	http.Error(w, "Invalid newIndex", http.StatusBadRequest)
	// 	return
	// }

	// if err := j.db..MovePlaylistItem(playlistID, itemID, newIndex); err != nil {
	// 	http.Error(w, "Failed to move item", http.StatusInternalServerError)
	// 	return
	// }

	// w.WriteHeader(http.StatusNoContent)
}

// DELETE /Playlists/{playlistId}/Items
//
// deletePlaylistItemsHandler deletes items from a playlist
func (j *Jellyfin) deletePlaylistItemsHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// playlistID := vars["playlistid"]

	// itemIDs := r.URL.Query()["Ids"]
	// if len(itemIDs) == 0 {
	// 	http.Error(w, "Ids parameter required", http.StatusBadRequest)
	// 	return
	// }

	// if err := j.db..DeleteItemsFromPlaylist(playlistID, itemIDs); err != nil {
	// 	http.Error(w, "Failed to delete items", http.StatusInternalServerError)
	// 	return
	// }

	// w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}/Users
//
// getPlaylistAllUsersHandler retrieves users with access to a playlist. Always returns the current user.
func (j *Jellyfin) getPlaylistAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	response := []JFPlaylistAccess{
		{
			Users:   []string{accessToken.User.ID},
			Canedit: true,
		},
	}
	serveJSON(response, w)
}

// GET /Playlists/{playlistId}/Users/{user}
//
// getPlaylistUsersHandler retrieves users with access to a playlist. Always returns the current user.
func (j *Jellyfin) getPlaylistUsersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	response := JFPlaylistAccess{
		Users:   []string{accessToken.User.ID},
		Canedit: true,
	}
	serveJSON(response, w)
}

// makeJFItemCollectionPlaylist creates a top level collection item with items for each playlists of the user
func (j *Jellyfin) makeJFItemCollectionPlaylist(ctx context.Context, userID string) (JFItem, error) {
	var itemCount int

	// Get total item count across all playlists
	if playlistIDs, err := j.repo.GetPlaylists(ctx, userID); err == nil {
		for _, ID := range playlistIDs {
			playlist, err := j.repo.GetPlaylist(ctx, userID, ID)
			if err == nil && playlist != nil {
				itemCount += len(playlist.ItemIDs)
			}
		}
	}

	id := makeJFCollectionPlaylistID(playlistCollectionID)
	response := JFItem{
		Name:                     "Playlists",
		ServerID:                 j.serverID,
		ID:                       id,
		ParentID:                 makeJFRootID(collectionRootID),
		Etag:                     idhash.Hash(playlistCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           collectionTypePlaylists,
		SortName:                 collectionTypePlaylists,
		Type:                     itemTypeUserView,
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               itemCount,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(playlistCollectionID),
		ExternalUrls:             []JFExternalUrls{},
		PlayAccess:               "Full",
		PrimaryImageAspectRatio:  1.7777777777777777,
		RemoteTrailers:           []JFRemoteTrailers{},
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		ImageTags:                j.makeJFImageTags(ctx, id, imageTypePrimary),
		// PremiereDate should be set based upon most recent item in collection
	}
	return response, nil
}

// makeJFItemPlaylist creates a playlist item from the provided playlistID
func (j *Jellyfin) makeJFItemPlaylist(ctx context.Context, userID, playlistID string) (JFItem, error) {
	playlist, err := j.repo.GetPlaylist(ctx, userID, playlistID)
	if err != nil || playlist == nil {
		return JFItem{}, errors.New("could not find playlist")
	}

	response := JFItem{
		Type:                     itemTypePlaylist,
		ID:                       makeJFPlaylistID(playlist.ID),
		ParentID:                 makeJFCollectionPlaylistID(playlistCollectionID),
		ServerID:                 j.serverID,
		Name:                     playlist.Name,
		SortName:                 playlist.Name,
		IsFolder:                 true,
		Path:                     "/playlist",
		Etag:                     idhash.Hash(playlist.ID),
		DateCreated:              time.Now().UTC(),
		CanDelete:                true,
		CanDownload:              true,
		PlayAccess:               "Full",
		RecursiveItemCount:       len(playlist.ItemIDs),
		ChildCount:               len(playlist.ItemIDs),
		LocationType:             "FileSystem",
		MediaType:                "Video",
		DisplayPreferencesID:     makeJFDisplayPreferencesID(playlistCollectionID),
		EnableMediaSourceDisplay: true,
	}
	return response, nil
}

// makeJFItemPlaylistOverview creates a list of playlists of the user.
func (j *Jellyfin) makeJFItemPlaylistOverview(ctx context.Context, userID string) ([]JFItem, error) {
	playlistIDs, err := j.repo.GetPlaylists(ctx, userID)
	if err != nil {
		return []JFItem{}, err
	}

	items := []JFItem{}
	for _, ID := range playlistIDs {
		if playlistItem, err := j.makeJFItemPlaylist(ctx, userID, ID); err == nil {
			items = append(items, playlistItem)
		}
	}
	return items, nil
}

// makeJFItemPlaylistItemList creates an item list of one playlist of the user.
func (j *Jellyfin) makeJFItemPlaylistItemList(ctx context.Context, userID, playlistID string) ([]JFItem, error) {

	playlist, err := j.repo.GetPlaylist(ctx, userID, playlistID)
	log.Printf("makeJFItemPlaylistItemList: %+v, %+v", playlistID, err)
	if err != nil {
		return []JFItem{}, err
	}

	items := []JFItem{}
	for _, itemID := range playlist.ItemIDs {
		c, i := j.collections.GetItemByID(itemID)
		if i != nil {
			item, err := j.makeJFItem(ctx, userID, i, c.ID)
			if err != nil {
				return []JFItem{}, err
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// makeJFPlaylistID returns an external id for a playlist.
func makeJFPlaylistID(playlistID string) string {
	return itemprefix_playlist + playlistID
}

// isJFPlaylistID checks if the provided ID is a playlist ID.
func isJFPlaylistID(id string) bool {
	return strings.HasPrefix(id, itemprefix_playlist)
}

// makeJFCollectionPlaylistID returns an external id for a playlist collection.
func makeJFCollectionPlaylistID(playlistCollectionID string) string {
	return itemprefix_collection_playlist + playlistCollectionID
}

// isJFCollectionPlaylistID checks if the provided ID is the playlist collection ID.
func isJFCollectionPlaylistID(id string) bool {
	// There is only one playlist collection id, so we can do a direct comparison
	return id == makeJFCollectionPlaylistID(playlistCollectionID)
}
