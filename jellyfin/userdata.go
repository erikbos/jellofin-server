package jellyfin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/database/model"
)

const (
	// APIresponse PositionTicks are in micro seconds
	TicsToSeconds             = 10000000
	ErrFailedToUpdateUserData = "Failed to update userdata"
	ErrInvalidJSONPayload     = "Invalid JSON payload"
	ErrInvalidUserID          = "Forbidden, provided userID does not match authenticated user"
	ErrInvalidPositionTicks   = "Invalid positionTicks value"
)

// /UserItems/1d57ee2251656c5fb9a05becdf0e62a3/Userdata
//
// usersItemUserDataHandler returns the user data for a specific item
func (j *Jellyfin) usersItemUserDataHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["itemid"]

	playstate, err := j.repo.GetUserData(r.Context(), reqCtx.User.ID, trimPrefix(itemID))
	if err != nil {
		// If we don't have user data for this item, we return an empty userdata object
		playstate = &model.UserData{}
	}
	userData := j.makeJFUserData(reqCtx.User.ID, itemID, playstate)
	serveJSON(userData, w)
}

// POST /UserPlayedItems/{item}
// POST /Users/{user}/PlayedItems/{item}
//
// usersPlayedItemsPostHandler marks an item as played.
func (j *Jellyfin) usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	vars := mux.Vars(r)
	itemID := vars["itemid"]
	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, itemID, 0, true); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DELETE /UserPlayedItems/{item}
// DELETE /Users/{user}/PlayedItems/{item}
//
// // usersPlayedItemsPostHandler marks an item as not played.
func (j *Jellyfin) usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["itemid"]

	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, itemID, 0, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// POST /PlayingItems/{itemid}
//
// playingItemsHandler is called when an item starts playing.
// all state is provided as query parameters
func (j *Jellyfin) playingItemsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	queryParams := r.URL.Query()
	if reqCtx.User.ID != queryParams.Get("userId") {
		apierror(w, ErrInvalidUserID, http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	itemID := vars["itemid"]

	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, itemID, false)

	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, itemID, 0, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /PlayingItems/{itemid}/Progress
//
// playingItemsProgressHandler is called periodically while an item is playing to report progress.
func (j *Jellyfin) playingItemsProgressHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	queryParams := r.URL.Query()
	if reqCtx.User.ID != queryParams.Get("userId") {
		apierror(w, ErrInvalidUserID, http.StatusBadRequest)
		return
	}
	positionTicks, err := strconv.ParseInt(queryParams.Get("positionTicks"), 10, 64)
	if err != nil {
		apierror(w, ErrInvalidPositionTicks, http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	itemID := vars["itemid"]

	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, itemID, false)

	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, itemID, positionTicks, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
}

// DELETE /PlayingItems/{itemid}
//
// playingItemsDeleteHandler is called when an item stops playing.
func (j *Jellyfin) playingItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	queryParams := r.URL.Query()
	if reqCtx.User.ID != queryParams.Get("userId") {
		apierror(w, ErrInvalidUserID, http.StatusBadRequest)
		return
	}
	// we only care about capturing latest position when the item is stopped
	positionTicks, err := strconv.ParseInt(queryParams.Get("positionTicks"), 10, 64)
	if err != nil {
		apierror(w, ErrInvalidPositionTicks, http.StatusBadRequest)
		return
	}
	vars := mux.Vars(r)
	itemID := vars["itemid"]

	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, itemID, false)

	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, itemID, positionTicks, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Playing
//
// sessionsPlayingHandler is called when an item starts playing.
func (j *Jellyfin) sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		apierror(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, request.ItemId, request.IsPaused)

	// log.Printf("\nsessionsPlayingHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	reqCtx.User.ID, request.ItemId, request.PositionTicks/TicsToSeconds)
	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, request.ItemId, request.PositionTicks, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Playing/Progress
//
// sessionsPlayingProgressHandler is called periodically while an item is playing to report progress.
func (j *Jellyfin) sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		apierror(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, request.ItemId, request.IsPaused)

	// log.Printf("\nsessionsPlayingProgressHandler UserID: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	reqCtx.User.ID, request.ItemId, request.PositionTicks/TicsToSeconds)
	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, request.ItemId, request.PositionTicks, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// /Sessions/Playing/Stopped
//
// sessionsPlayingStoppedHandler is called when an item stops playing.
func (j *Jellyfin) sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	var request JFPlayState
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		apierror(w, ErrInvalidJSONPayload, http.StatusBadRequest)
		return
	}
	// Zero out itemid in session as playing has stopped
	j.sessionTable.UpdateStatus(reqCtx.User.ID, reqCtx.Token.DeviceID, "", false)

	// log.Printf("\nsessionsPlayingStoppedHandler UserID: %s, ItemId: %s, Progress: %d seconds, canSeek: %t\n\n",
	// 	reqCtx.User.ID, request.ItemId, request.PositionTicks/TicsToSeconds, request.CanSeek)
	if err := j.userDataUpdate(r.Context(), reqCtx.User.ID, request.ItemId, request.PositionTicks, false); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
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

	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["itemid"]

	playstate, err := j.repo.GetUserData(r.Context(), reqCtx.User.ID, trimPrefix(itemID))
	if err != nil {
		playstate = &model.UserData{}
	}

	playstate.Favorite = true

	if err := j.repo.UpdateUserData(r.Context(), reqCtx.User.ID, itemID, playstate); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	userData := j.makeJFUserData(reqCtx.User.ID, itemID, playstate)
	serveJSON(userData, w)
}

// DELETE /UserFavoriteItems/{item}
//
// // userFavoriteItemsDeleteHandler unmarks an item as favorite.
func (j *Jellyfin) userFavoriteItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["itemid"]

	playstate, err := j.repo.GetUserData(r.Context(), reqCtx.User.ID, trimPrefix(itemID))
	if err != nil {
		playstate = &model.UserData{}
	}

	playstate.Favorite = false

	if err := j.repo.UpdateUserData(r.Context(), reqCtx.User.ID, itemID, playstate); err != nil {
		apierror(w, ErrFailedToUpdateUserData, http.StatusInternalServerError)
		return
	}
	userData := j.makeJFUserData(reqCtx.User.ID, itemID, playstate)
	serveJSON(userData, w)
}

// makeJFUserData creates a JFUserData object, and populates from Userdata if provided
func (j *Jellyfin) makeJFUserData(userID, itemID string, p *model.UserData) (response *JFUserData) {
	response = &JFUserData{
		Key:    userID + "/" + itemID,
		ItemID: "00000000000000000000000000000000",
	}
	if p != nil {
		response.IsFavorite = p.Favorite
		response.LastPlayedDate = p.Timestamp
		response.PlaybackPositionTicks = p.Position * TicsToSeconds
		response.PlayedPercentage = p.PlayedPercentage
		response.Played = p.Played
	}
	return
}
