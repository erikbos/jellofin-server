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

// func (j *Jellyfin) registerHandlersPlayState(r *mux.Router, middleware func(http.HandlerFunc) http.Handler) {
// 	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
// 	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
// 	r.Handle("/Sessions/Playing", middleware(j.sessionsPlayingHandler)).Methods("POST")
// 	r.Handle("/Sessions/Playing/Progress", middleware(j.sessionsPlayingProgressHandler)).Methods("POST")
// 	r.Handle("/Sessions/Playing/Stopped", middleware(j.sessionsPlayingStoppedHandler)).Methods("POST")
// }

// usersPlayedItemsPostHandler marks item as played.
func (j *Jellyfin) usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	j.playStateUpdate(accessTokenDetails.UserID, itemID, 0, true)
	w.WriteHeader(http.StatusOK)
}

// usersPlayedItemsPostHandler marks item as not played.
func (j *Jellyfin) usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
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
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
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
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken context not found", http.StatusUnauthorized)
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
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken context not found", http.StatusUnauthorized)
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
	log.Printf("playStateUpdate userID: %s, itemID: %s, Progress: %d sec\n",
		userID, itemID, positionTicks/TicsToSeconds)

	var duration int
	if strings.HasPrefix(itemID, itemprefix_episode) {
		_, _, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID))

		// fix me: we should not have to load NFO here
		j.lazyLoadNFO(&episode.Nfo, episode.NfoPath)

		duration = episode.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
	} else {
		_, item := j.collections.GetItemByID(itemID)
		duration = item.Nfo.Runtime * 60
	}

	playstate := database.PlayState{
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

	j.db.PlayStateRepo.Update(userID, trimPrefix(itemID), playstate)
	return nil
}
