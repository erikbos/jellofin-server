package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

// POST /Playlists?ids=rVFG3EzPthk2wowNkqUl&name=MMM&userId=XAOVn7iqiBujnIQY8sd0
//
// createPlaylistHandler creates a new playlist
func (j *Jellyfin) createPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	queryparams := r.URL.Query()
	// Populate request from query parameters, lower priority than POST body
	req := JFCreatePlaylistRequest{
		Name:   queryparams.Get("name"),
		UserID: queryparams.Get("userId"),
	}
	if queryparams.Get("ids") != "" {
		for i := range strings.SplitSeq(queryparams.Get("ids"), ",") {
			req.Ids = append(req.Ids, trimPrefix(i))
		}
	}
	// POST-submitted values have priority over query parameters
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	log.Printf("createPlaylistHandler: %+v", req)
	if req.Name == "" || req.UserID == "" || len(req.Ids) == 0 {
		apierror(w, "Name, UserId, and Ids are required", http.StatusBadRequest)
		return
	}

	newPlaylist := model.Playlist{
		ID:        idhash.NewRandomID(),
		Name:      req.Name,
		UserID:    req.UserID,
		ItemIDs:   req.Ids,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	log.Printf("newPlaylist: %+v", newPlaylist)

	if err := j.repo.UpsertPlaylist(r.Context(), newPlaylist); err != nil {
		log.Printf("playlistID: %s, err: %v", newPlaylist.ID, err)
		apierror(w, "Failed to create playlist", http.StatusInternalServerError)
		return
	}
	response := JFCreatePlaylistResponse{
		Id: itemprefix_playlist + newPlaylist.ID,
	}
	w.WriteHeader(http.StatusCreated)
	serveJSON(&response, w)
}

// POST /Playlists/{playlistId}
//
// updatePlaylistHandler updates a playlist
func (j *Jellyfin) updatePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	var req JFCreatePlaylistRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}
	playlist, err := j.repo.GetPlaylist(r.Context(), req.UserID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Failed to retrieve playlist", http.StatusInternalServerError)
		return
	}
	if req.Name != "" {
		playlist.Name = req.Name
	}
	if req.Ids != nil {
		playlist.ItemIDs = req.Ids
	}
	playlist.UpdatedAt = time.Now().UTC()
	if err := j.repo.UpsertPlaylist(r.Context(), *playlist); err != nil {
		log.Printf("playlistID: %s, err: %v", playlist.ID, err)
		apierror(w, "Failed to update playlist", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}
//
// getPlaylistHandler retrieves a playlist by ID
func (j *Jellyfin) getPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)

	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	playlist, err := j.repo.GetPlaylist(r.Context(), reqCtx.User.ID, trimPrefix(playlistID))
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
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	playlist, err := j.repo.GetPlaylist(r.Context(), reqCtx.User.ID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}
	items, err := j.makeJFItemByIDs(r.Context(), reqCtx.User.ID, playlist.ItemIDs)
	if err != nil {
		apierror(w, "Failed to retrieve playlist items", http.StatusInternalServerError)
		return
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
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	vars := mux.Vars(r)
	playlistID := vars["playlistid"]

	var req JFCreatePlaylistRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			apierror(w, "Invalid request body", http.StatusBadRequest)
			return
		}
	}
	playlist, err := j.repo.GetPlaylist(r.Context(), reqCtx.User.ID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}
	// Remove all items we already have in the playlist from the request, so we don't add duplicates
	uniqueItemIDs := []string{}
	for _, ID := range req.Ids {
		if !slices.Contains(playlist.ItemIDs, ID) {
			uniqueItemIDs = append(uniqueItemIDs, ID)
		}
	}
	if err := j.repo.AddItemsToPlaylist(r.Context(), playlist.ID, uniqueItemIDs); err != nil {
		apierror(w, "Failed to add items", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /Playlists/{playlistId}/Items
//
// deletePlaylistItemsHandler deletes items from a playlist
func (j *Jellyfin) deletePlaylistItemsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	vars := mux.Vars(r)
	playlistID := vars["playlistid"]
	queryparams := r.URL.Query()

	playlist, err := j.repo.GetPlaylist(r.Context(), reqCtx.User.ID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}
	// Remove item IDs from the playlist if they are present
	var itemsToDelete []string
	for ID := range strings.SplitSeq(queryparams.Get("entryIds"), ",") {
		if slices.Contains(playlist.ItemIDs, ID) {
			itemsToDelete = append(itemsToDelete, ID)
		}
	}
	if len(itemsToDelete) > 0 {
		j.repo.DeleteItemsFromPlaylist(r.Context(), playlist.ID, itemsToDelete)
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}/Items/{itemId}/Move/{newIndex}
//
// movePlaylistItemHandler moves an item in a playlist
func (j *Jellyfin) movePlaylistItemHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	vars := mux.Vars(r)
	playlistID := vars["playlistid"]
	playlist, err := j.repo.GetPlaylist(r.Context(), reqCtx.User.ID, trimPrefix(playlistID))
	if err != nil {
		apierror(w, "Playlist not found", http.StatusNotFound)
		return
	}
	itemID := vars["itemId"]
	newIndex, err := strconv.Atoi(vars["newIndex"])
	if err != nil {
		http.Error(w, "Invalid newIndex", http.StatusBadRequest)
		return
	}
	log.Printf("movePlaylistItemHandler: playlistID: %s, itemID: %s, newIndex: %d not implemented", playlist.ID, itemID, newIndex)
	w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}/Users
//
// getPlaylistAllUsersHandler retrieves users with access to a playlist. Always returns the current user.
func (j *Jellyfin) getPlaylistAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	response := []JFPlaylistAccess{
		{
			Users:   []string{reqCtx.User.ID},
			Canedit: true,
		},
	}
	serveJSON(response, w)
}

// GET /Playlists/{playlistId}/Users/{user}
//
// getPlaylistUsersHandler retrieves users with access to a playlist. Always returns the current user.
func (j *Jellyfin) getPlaylistUsersHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	response := JFPlaylistAccess{
		Users:   []string{reqCtx.User.ID},
		Canedit: true,
	}
	serveJSON(response, w)
}

// makeJFItemCollectionPlaylist creates a top level collection item with items for each playlists of the user
func (j *Jellyfin) makeJFItemCollectionPlaylist(ctx context.Context, userID string) (JFItem, error) {
	var itemCount int

	// log.Printf("makeJFItemCollectionPlaylist: userID: %s", userID)

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
		ParentID:                 j.makeJFRootID(),
		Etag:                     idhash.Hash(playlistCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           collectionTypePlaylists,
		SortName:                 collectionTypePlaylists,
		Type:                     itemTypeUserView,
		IsFolder:                 true,
		EnableMediaSourceDisplay: new(true),
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
		CanDelete:                new(false),
		CanDownload:              new(true),
		SpecialFeatureCount:      0,
		ImageTags:                j.makeJFImageTags(ctx, id, imageTypePrimary),
		UserData:                 j.makeJFUserData(userID, id, nil),
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

	id := makeJFPlaylistID(playlist.ID)
	response := JFItem{
		Type:                     itemTypePlaylist,
		ID:                       id,
		ParentID:                 makeJFCollectionPlaylistID(playlistCollectionID),
		ServerID:                 j.serverID,
		Name:                     playlist.Name,
		SortName:                 playlist.Name,
		IsFolder:                 true,
		Path:                     "/playlist/" + strings.ToLower(strings.Join(strings.Fields(playlist.Name), "")),
		Etag:                     idhash.Hash(playlist.ID),
		DateCreated:              time.Now().UTC(),
		CanDelete:                new(true),
		CanDownload:              new(true),
		PlayAccess:               "Full",
		RecursiveItemCount:       len(playlist.ItemIDs),
		ChildCount:               len(playlist.ItemIDs),
		LocationType:             "FileSystem",
		MediaType:                "Video",
		DisplayPreferencesID:     makeJFDisplayPreferencesID(playlistCollectionID),
		EnableMediaSourceDisplay: new(true),
		ImageTags:                j.makeJFImageTags(ctx, id, imageTypePrimary),
		UserData:                 j.makeJFUserData(userID, id, nil),
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
			item, err := j.makeJFItem(ctx, userID, i, c.ID, false)
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
