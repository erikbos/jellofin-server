package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

const (
	// PositionTicks are in micro seconds
	TicsToSeconds = 10000000
)

func registerPlayStateHandlers(r *mux.Router) {
	gzip := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(http.HandlerFunc(handler))
	}

	r.Handle("/Sessions/Dump", gzip(sessionsDumpHandler))
	r.Handle("/Sessions/Playing", gzip(sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Ping", gzip(sessionsPlayingPingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", gzip(sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", gzip(sessionsPlayingStoppedHandler)).Methods("POST")

	// r.Handle("/Sessions/PlayedItems/{itemID}", gzip(sessionsPlayingitemIDsHandler))
	// r.Handle("/Sessions/PlayedItems/{itemID}/Progress", gzip(sessionsPlayingitemIDsProgressHandler)).Methods("POST")

	r.Handle("/UserPlayeditemIDs/{itemID}", gzip(userPlayeditemIDsPostHandler)).Methods("POST")
	r.Handle("/UserPlayeditemIDs/{itemID}", gzip(userPlayeditemIDsDeleteHandler)).Methods("DELETE")
	// Infuse calls these, not in OpenAPI spec ?!
	r.Handle("/Users/{user}/PlayedItems/{itemID}", gzip(usersPlayeditemIDsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{itemID}", gzip(usersPlayeditemIDsDeleteHandler)).Methods("DELETE")
}

// Reports playback has started within a session.
func sessionsDumpHandler(w http.ResponseWriter, r *http.Request) {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()
	serveJSON(PlayState.state, w)
}

// Reports playback has started within a session.
func sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	var request JFPlaybackProgressInfo
	// receives PlaybackProgressInfo
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	log.Printf("sessionsPlayingHandler %+v\n", request)
	log.Printf("sessionsPlayingHandler PlaySessionId %s, Progress: %d seconds\n", *request.PlaySessionId, *request.PositionTicks/TicsToSeconds)

	PlayState.Update(*request.PlaySessionId, *request.PositionTicks)

	w.WriteHeader(http.StatusNoContent)
}

// Pings a playback session.
func sessionsPlayingPingHandler(w http.ResponseWriter, r *http.Request) {
	// playSessionId := r.URL.Query().Get("playSessionId")
	// log.Printf("sessionsPlayingPingHandler, playSessionId: %s\n", playSessionId)
	w.WriteHeader(http.StatusNoContent)
}

// Reports playback progress within a session.
func sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	var request JFPlaybackProgressInfo
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("sessionsPlayingProgressHandler %+v\n", request)
	// log.Printf("sessionsPlayingProgressHandler PlaySessionId %s, Progress: %d seconds\n", *request.PlaySessionId, *request.PositionTicks/TicsToSeconds)

	PlayState.Update(*request.PlaySessionId, *request.PositionTicks)

	w.WriteHeader(http.StatusNoContent)
}

// Reports playback has stopped within a session.
func sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	var request JFPlaybackProgressInfo
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("sessionsPlayingStoppedHandler %+v\n", request)
	// log.Printf("sessionsPlayingStoppedHandler PlaySessionId %s\n", *request.PlaySessionId)
	// log.Printf("sessionsPlayingStoppedHandler Progress: %d seconds\n", *request.PositionTicks)

	PlayState.Update(*request.PlaySessionId, *request.PositionTicks)

	w.WriteHeader(http.StatusNoContent)
}

// Marks an itemID as played for user.
func userPlayeditemIDsPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queryparams := r.URL.Query()
	// log.Printf("userPlayeditemIDsHandler: itemID %s, userId %s, method %s\n", vars["itemID"], queryparams.Get("userId"), r.Method)
	PlayState.MarkitemIDPlayed(queryparams.Get("userId"), vars["itemID"])
	w.WriteHeader(http.StatusOK)
}

// Marks an itemID as unplayed for user.
func userPlayeditemIDsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queryparams := r.URL.Query()
	// log.Printf("userPlayeditemIDsHandler: itemID %s, userId %s, method %s\n", vars["itemID"], queryparams.Get("userId"), r.Method)
	PlayState.MarkitemIDUnplayed(queryparams.Get("userId"), vars["itemID"])
	w.WriteHeader(http.StatusOK)
}

// Marks an itemID as played for user.
func usersPlayeditemIDsPostHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// log.Printf("usersPlayeditemIDsPostHandler: user %s, itemID %s\n", vars["user"], vars["itemID"])
	PlayState.MarkitemIDPlayed(vars["user"], vars["itemID"])
	w.WriteHeader(http.StatusOK)
}

// Marks an itemID as not played for user.
func usersPlayeditemIDsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	// log.Printf("usersPlayeditemIDsDeleteHandler: user %s, itemID %s\n", vars["user"], vars["itemID"])
	PlayState.MarkitemIDUnplayed(vars["user"], vars["itemID"])
	w.WriteHeader(http.StatusOK)
}

////

func init() {
	PlayState.state = make(map[string]PlaySessionDetails)
}

var PlayState PlaySessionRepo

type PlaySessionRepo struct {
	mu    sync.Mutex
	state map[string]PlaySessionDetails
}

type PlaySessionDetails struct {
	User           string    `json:"user"`
	ItemID         string    `json:"itemID"`
	PositionTicks  int64     `json:"positionTicks"`
	LastPlayedDate time.Time `json:"lastPlayedDate"`
	Played         bool      `json:"played"`
}

// New generates new PlaysessionID
func (p *PlaySessionRepo) New(user, itemID string) string {
	playSessionId := genSessionID(user, itemID)
	p.state[playSessionId] = PlaySessionDetails{
		User:           user,
		ItemID:         itemID,
		LastPlayedDate: time.Now().UTC(),
	}
	return playSessionId
}

// Update
func (p *PlaySessionRepo) Update(playSessionId string, progress int64) {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	if session, ok := p.state[playSessionId]; ok {
		session.LastPlayedDate = time.Now().UTC()
		session.PositionTicks = progress * 1000
		p.state[playSessionId] = session

		log.Printf("sessionUpdate: PlaySessionId: %s, %d seconds\n", playSessionId, progress/TicsToSeconds)
	}
}

func (p *PlaySessionRepo) MarkitemIDPlayed(user, itemID string) {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()
	log.Printf("MarkitemIDPlayed: user %s, itemID %s\n", user, itemID)
	playSessionId := genSessionID(user, itemID)
	if session, ok := p.state[playSessionId]; ok {
		session.Played = true
		p.state[playSessionId] = session
	}
}

func (p *PlaySessionRepo) MarkitemIDUnplayed(user, itemID string) {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()
	log.Printf("MarkitemIDUnplayed: user %s, itemID %s\n", user, itemID)
	playSessionId := genSessionID(user, itemID)
	if session, ok := p.state[playSessionId]; ok {
		session.PositionTicks = 0
		session.Played = false
		p.state[playSessionId] = session
	}
}

func (p *PlaySessionRepo) ItemUserData(item *JFItem, user string) *JFUserData {
	PlayState.mu.Lock()
	defer PlayState.mu.Unlock()

	// id := genSessionID(user, item.ID)
	// log.Printf("itemUserData: %s, %s (%s)", user, item.ID, id)
	// log.Printf("itemUserData: %d\n\n", item.MediaSources[0].RunTimeTicks)

	if session, ok := p.state[genSessionID(user, item.ID)]; ok {
		data := JFUserData{
			PlaybackPositionTicks: session.PositionTicks,
			Played:                session.Played,
			LastPlayedDate:        session.LastPlayedDate,
			PlayedPercentage:      0.50,
			// PlayedPercentage:      float64(session.PositionTicks) / float64(item.MediaSources[0].RunTimeTicks),
			Key: item.ID,
		}
		if !session.LastPlayedDate.IsZero() {
			data.PlayCount = 1
		}
		return &data
	}
	return nil
}

func genSessionID(user, itemID string) string {
	return user + "/" + itemID
}
