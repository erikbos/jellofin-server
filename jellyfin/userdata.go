package jellyfin

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/database"
)

// POST /UserPlayedItems/{item}
// POST /Users/{user}/PlayedItems/{item}
//
// usersPlayedItemsPostHandler marks an item as played.
func (j *Jellyfin) usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	j.playStateUpdate(accessTokenDetails.UserID, itemID, 0, true)
	w.WriteHeader(http.StatusOK)
}

// DELETE /UserPlayedItems/{item}
// DELETE /Users/{user}/PlayedItems/{item}
//
// // usersPlayedItemsPostHandler marks an item as not played.
func (j *Jellyfin) usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	j.playStateUpdate(accessTokenDetails.UserID, itemID, 0, false)
	w.WriteHeader(http.StatusOK)
}

// PositionTicks are in micro seconds
const TicsToSeconds = 10000000

func (j *Jellyfin) sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessTokenDetails.UserID, request.ItemId, request.PositionTicks/TicsToSeconds)
	j.playStateUpdate(accessTokenDetails.UserID, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingProgressHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessTokenDetails.UserID, request.ItemId, request.PositionTicks/TicsToSeconds)
	j.playStateUpdate(accessTokenDetails.UserID, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingStoppedHandler UserID: %s, ItemId: %s, Progress: %d seconds, canSeek: %t\n\n",
	// 	accessTokenDetails.UserID, request.ItemId, request.PositionTicks/TicsToSeconds, request.CanSeek)
	j.playStateUpdate(accessTokenDetails.UserID, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) playStateUpdate(userID, itemID string, positionTicks int, markAsWatched bool) (err error) {
	// log.Printf("playStateUpdate userID: %s, itemID: %s, Progress: %d sec\n",
	// 	userID, itemID, positionTicks/TicsToSeconds)

	// fixme: duration determination should be moved to the collections
	var duration int
	if strings.HasPrefix(itemID, itemprefix_episode) {
		_, _, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID))

		// fix me: we should not have to load NFO here
		episode.LoadNfo()
		if episode.Nfo != nil &&
			episode.Nfo.FileInfo != nil &&
			episode.Nfo.FileInfo.StreamDetails != nil &&
			episode.Nfo.FileInfo.StreamDetails.Video != nil {
			duration = episode.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
		} else {
			log.Printf("playStateUpdate: no duration for episode %s\n", itemID)
		}
	} else {
		_, item := j.collections.GetItemByID(itemID)
		if item != nil {
			item.LoadNfo()
		}
		if item.Nfo != nil {
			if item.Nfo.Runtime != 0 {
				duration = item.Nfo.Runtime * 60
			} else if item.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds != 0 {
				duration = item.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
			}
		}
	}

	playstate := database.UserData{
		Timestamp: time.Now().UTC(),
	}

	position := positionTicks / TicsToSeconds
	playedPercentage := 100 * position / duration

	// Mark as watched in case > 98% of the item is played
	if markAsWatched || playedPercentage >= 98 {
		playstate.Position = 0
		playstate.PlayedPercentage = 0
		playstate.Played = true
	} else {
		playstate.Position = position
		playstate.PlayedPercentage = playedPercentage
		playstate.Played = false
	}

	j.db.UserDataRepo.Update(userID, trimPrefix(itemID), playstate)
	return nil
}

// POST /UserFavoriteItems/{item}
//
// // userFavoriteItemsPostHandler marks an item as favorite.
func (j *Jellyfin) userFavoriteItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	playstate, err := j.db.UserDataRepo.Get(accessTokenDetails.UserID, itemID)
	if err != nil {
		playstate = database.UserData{}
	}
	userData := j.makeJFUserData(accessTokenDetails.UserID, itemID, playstate)
	serveJSON(userData, w)
}

// DELETE /UserFavoriteItems/{item}
//
// // userFavoriteItemsDeleteHandler unmarks an item as favorite.
func (j *Jellyfin) userFavoriteItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	playstate, err := j.db.UserDataRepo.Get(accessTokenDetails.UserID, itemID)
	if err != nil {
		playstate = database.UserData{}
	}
	playstate.Favorite = false
	j.db.UserDataRepo.Update(accessTokenDetails.UserID, itemID, playstate)

	userData := j.makeJFUserData(accessTokenDetails.UserID, itemID, playstate)
	serveJSON(userData, w)
}
