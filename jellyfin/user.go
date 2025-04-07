package jellyfin

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/database"
)

// GET /Users
//
// usersAllHandler returns all users, we return only the current user
func (j *Jellyfin) usersAllHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	dbuser, err := j.db.UserRepo.GetById(accessTokenDetails.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusNotFound)
		return
	}
	response := []JFUser{
		genJFUser(dbuser),
	}
	serveJSON(response, w)
}

// GET /Users/Me
//
// usersMeHandler returns the current user
func (j *Jellyfin) usersMeHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}
	dbuser, err := j.db.UserRepo.GetById(accessTokenDetails.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusNotFound)
		return
	}
	response := genJFUser(dbuser)
	serveJSON(response, w)
}

// GET /Users/{user}
//
// usersHandler returns a user, we always return the current user
func (j *Jellyfin) usersHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(w, r)
	if accessTokenDetails == nil {
		return
	}

	vars := mux.Vars(r)
	if vars["user"] != accessTokenDetails.UserID {
		http.Error(w, "invalid user id", http.StatusNotFound)
		return
	}

	dbuser, err := j.db.UserRepo.GetById(accessTokenDetails.UserID)
	if err != nil {
		http.Error(w, "invalid user id", http.StatusNotFound)
		return
	}
	response := genJFUser(dbuser)
	serveJSON(response, w)
}

// GET /Users/Public
//
// usersHandler returns list of public users, none
func (j *Jellyfin) usersPublicHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFUser{}
	serveJSON(response, w)
}

func genJFUser(user *database.User) JFUser {
	return JFUser{
		Id:                        user.ID,
		Name:                      user.Username,
		ServerId:                  serverID,
		HasPassword:               true,
		HasConfiguredPassword:     true,
		HasConfiguredEasyPassword: false,
		EnableAutoLogin:           false,
		LastLoginDate:             time.Now().UTC(),
		LastActivityDate:          time.Now().UTC(),
	}
}
