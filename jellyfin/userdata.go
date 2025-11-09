package jellyfin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	// APIresponse PositionTicks are in micro seconds
	TicsToSeconds             = 10000000
	ErrFailedToUpdateUserData = "Failed to update userdata"
	ErrInvalidJSONPayload     = "Invalid JSON payload"
)

// POST /UserPlayedItems/{item}
// POST /Users/{user}/PlayedItems/{item}
//
// usersPlayedItemsPostHandler marks an item as played.
func (j *Jellyfin) usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	if err := j.userDataUpdate(r.Context(), accessToken.UserID, itemID, 0, true); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETE /UserPlayedItems/{item}
// DELETE /Users/{user}/PlayedItems/{item}
//
// // usersPlayedItemsPostHandler marks an item as not played.
func (j *Jellyfin) usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	if err := j.userDataUpdate(r.Context(), accessToken.UserID, itemID, 0, false); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// /Sessions/Playing
func (j *Jellyfin) sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessToken.UserID, request.ItemId, request.PositionTicks/TicsToSeconds)
	if err := j.userDataUpdate(r.Context(), accessToken.UserID, request.ItemId, request.PositionTicks, false); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Playing/Progress
func (j *Jellyfin) sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingProgressHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessToken.UserID, request.ItemId, request.PositionTicks/TicsToSeconds)
	if err := j.userDataUpdate(r.Context(), accessToken.UserID, request.ItemId, request.PositionTicks, false); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Playing/Stopped
func (j *Jellyfin) sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingStoppedHandler UserID: %s, ItemId: %s, Progress: %d seconds, canSeek: %t\n\n",
	// 	accessToken.UserID, request.ItemId, request.PositionTicks/TicsToSeconds, request.CanSeek)
	if err := j.userDataUpdate(r.Context(), accessToken.UserID, request.ItemId, request.PositionTicks, false); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) userDataUpdate(ctx context.Context, userID, itemID string, positionTicks int64, markAsWatched bool) (err error) {
	var duration int64
	if _, item := j.collections.GetItemByID(trimPrefix(itemID)); item != nil {
		duration = int64(item.Duration().Seconds())
	}
	// log.Printf("userDataUpdate userID: %s, itemID: %s, Progress: %d sec, Duration: %d sec\n",
	// 	userID, itemID, positionTicks/TicsToSeconds, duration)

	// If we don't have a duration, we assume 1 hour
	if duration == 0 {
		duration = 60 * 60
	}

	playstate, err := j.repo.GetUserData(ctx, userID, trimPrefix(itemID))
	if err != nil {
		playstate = &model.UserData{
			Timestamp: time.Now().UTC(),
		}
	}

	position := positionTicks / TicsToSeconds
	playedPercentage := int(100 * position / duration)

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

	return j.repo.UpdateUserData(ctx, userID, trimPrefix(itemID), playstate)
}

// POST /UserFavoriteItems/{item}
//
// // userFavoriteItemsPostHandler marks an item as favorite.
func (j *Jellyfin) userFavoriteItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("userFavoriteItemsPostHandler: %s\n", r.URL.Path)

	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	playstate, err := j.repo.GetUserData(r.Context(), accessToken.UserID, trimPrefix(itemID))
	if err != nil {
		playstate = &model.UserData{}
	}

	playstate.Favorite = true

	if err := j.repo.UpdateUserData(r.Context(), accessToken.UserID, itemID, playstate); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	userData := j.makeJFUserData(accessToken.UserID, itemID, playstate)
	serveJSON(userData, w)
}

// DELETE /UserFavoriteItems/{item}
//
// // userFavoriteItemsDeleteHandler unmarks an item as favorite.
func (j *Jellyfin) userFavoriteItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	playstate, err := j.repo.GetUserData(r.Context(), accessToken.UserID, trimPrefix(itemID))
	if err != nil {
		playstate = &model.UserData{}
	}

	playstate.Favorite = false

	if err := j.repo.UpdateUserData(r.Context(), accessToken.UserID, itemID, playstate); err != nil {
		http.Error(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	userData := j.makeJFUserData(accessToken.UserID, itemID, playstate)
	serveJSON(userData, w)
}
