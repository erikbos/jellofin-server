package jellyfin

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin/database"
	"github.com/erikbos/jellofin/idhash"
)

type JFCreatePlaylistRequest struct {
	Name   string   `json:"Name"`
	UserID string   `json:"UserId"`
	Ids    []string `json:"Ids,omitempty"`
}

type JFCreatePlaylistResponse struct {
	Id string `json:"Id"`
}

type JFGetPlaylistResponse struct {
	OpenAccess bool     `json:"OpenAccess"`
	Shares     []string `json:"Shares"`
	ItemIds    []string `json:"ItemIds,omitempty"`
}

type JFPlaylistAccess struct {
	Users   []string `json:"Users"`
	Canedit bool     `json:"CanEdit"`
}

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
	req.Name = queryparams.Get("Name")
	req.UserID = queryparams.Get("userId")
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Name == "" || req.UserID == "" {
		http.Error(w, "Name and UserId are required", http.StatusBadRequest)
		return
	}

	newPlaylist := database.Playlist{
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

	playlistID, err := j.db.PlaylistRepo.CreatePlaylist(newPlaylist)
	log.Printf("playlistID: %s, err: %v", playlistID, err)
	if err != nil {
		http.Error(w, "Failed to create playlist", http.StatusInternalServerError)
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
	// playlistID := vars["playlist"]

	// var req struct {
	// 	Name string `json:"Name"`
	// }

	// if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
	// 	http.Error(w, "Invalid request body", http.StatusBadRequest)
	// 	return
	// }

	// // Note: Since Jellyfin API typically creates new playlists rather than updating,
	// // this endpoint might be used for updating playlist metadata
	// _, err := j.db.PlaylistRepo.GetPlaylist(playlistID)
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
	playlistID := vars["playlist"]

	playlist, err := j.db.PlaylistRepo.GetPlaylist(accessToken.UserID, trimPrefix(playlistID))
	// log.Printf("querying playlist: %+v, %+v\n", playlist, err)
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
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
	playlistID := vars["playlist"]

	playlist, err := j.db.PlaylistRepo.GetPlaylist(accessToken.UserID, trimPrefix(playlistID))
	if err != nil {
		http.Error(w, "Playlist not found", http.StatusNotFound)
		return
	}

	items := []JFItem{}
	for _, itemID := range playlist.ItemIDs {
		c, i := j.collections.GetItemByID(itemID)
		if c != nil || i != nil {
			items = append(items, j.makeJFItem(accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
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
	playlistID := vars["playlist"]
	queryparams := r.URL.Query()

	var itemIDs []string
	for ID := range strings.SplitSeq(queryparams.Get("Ids"), ",") {
		itemIDs = append(itemIDs, trimPrefix(ID))
	}

	if err := j.db.PlaylistRepo.AddItemsToPlaylist(accessToken.UserID, trimPrefix(playlistID), itemIDs); err != nil {
		http.Error(w, "Failed to add items", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GET /Playlists/{playlistId}/Items/{itemId}/Move/{newIndex}
//
// movePlaylistItemHandler moves an item in a playlist
func (j *Jellyfin) movePlaylistItemHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// playlistID := vars["playlist"]
	// itemID := vars["itemId"]
	// newIndex, err := strconv.Atoi(vars["newIndex"])
	// if err != nil {
	// 	http.Error(w, "Invalid newIndex", http.StatusBadRequest)
	// 	return
	// }

	// if err := j.db.PlaylistRepo.MovePlaylistItem(playlistID, itemID, newIndex); err != nil {
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
	// playlistID := vars["playlist"]

	// itemIDs := r.URL.Query()["Ids"]
	// if len(itemIDs) == 0 {
	// 	http.Error(w, "Ids parameter required", http.StatusBadRequest)
	// 	return
	// }

	// if err := j.db.PlaylistRepo.DeleteItemsFromPlaylist(playlistID, itemIDs); err != nil {
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
			Users:   []string{accessToken.UserID},
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
		Users:   []string{accessToken.UserID},
		Canedit: true,
	}
	serveJSON(response, w)
}
