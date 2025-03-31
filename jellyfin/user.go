package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/database"
)

// POST /Users/AuthenticateByName
//
// usersAuthenticateByNameHandler authenticates a user by name
func (j *Jellyfin) usersAuthenticateByNameHandler(w http.ResponseWriter, r *http.Request) {
	var request JFAuthenticateUserByNameRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusUnauthorized)
		return
	}

	if len(request.Username) == 0 || len(request.Pw) == 0 {
		http.Error(w, "username and password required", http.StatusUnauthorized)
		return
	}

	embyHeader, err := j.parseAuthHeader(r)
	if err != nil || embyHeader == nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	user, err := j.db.UserRepo.Validate(request.Username, request.Pw)
	if err != nil {
		if err == database.ErrUserNotFound && j.autoRegister {
			user, err = j.db.UserRepo.Insert(request.Username, request.Pw)
			if err != nil {
				http.Error(w, "Failed to auto-register user", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Invalid username/password", http.StatusUnauthorized)
			return
		}
	}

	remoteAddress, _, _ := net.SplitHostPort(r.RemoteAddr)

	session := &JFSessionInfo{
		Id:                 "e3a869b7a901f8894de8ee65688db6c0",
		UserId:             user.ID,
		UserName:           user.Username,
		Client:             embyHeader.client,
		DeviceName:         embyHeader.device,
		DeviceId:           embyHeader.deviceID,
		ApplicationVersion: embyHeader.version,
		RemoteEndPoint:     remoteAddress,
		LastActivityDate:   time.Now().UTC(),
		IsActive:           true,
	}

	accesstoken := j.db.AccessTokenRepo.Generate(user.ID)

	response := JFAuthenticateByNameResponse{
		AccessToken: accesstoken,
		SessionInfo: session,
		ServerId:    serverID,
		User: JFUser{
			ServerId:                  serverID,
			Id:                        user.ID,
			Name:                      user.Username,
			HasPassword:               true,
			HasConfiguredPassword:     true,
			HasConfiguredEasyPassword: false,
			EnableAutoLogin:           false,
			LastLoginDate:             time.Now().UTC(),
			LastActivityDate:          time.Now().UTC(),
		},
	}
	serveJSON(response, w)
}

// authMiddleware validates auth token, token can be provided in various headers
func (j *Jellyfin) authmiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		found := false

		if embyHeader, err := j.parseAuthHeader(r); err == nil {
			token = embyHeader.token
			found = true
		}
		if v := r.Header.Get("x-emby-token"); v != "" {
			token = v
			found = true
		}
		if v := r.Header.Get("x-mediabrowser-token"); v != "" {
			token = v
			found = true
		}

		if !found {
			http.Error(w, "no token provided", http.StatusUnauthorized)
			return
		}

		tokendetails, err := j.db.AccessTokenRepo.Get(token)
		if err != nil {
			http.Error(w, "invalid access token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextAccessTokenDetails, tokendetails)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getAccessTokenDetails returns access token details from the
// request context populated by authmiddleware()
//
// if not found sends an HTTP unauthorized error
func (j *Jellyfin) getAccessTokenDetails(w http.ResponseWriter, r *http.Request) *database.AccessToken {
	// Ctx should have been populated by authmiddleware()
	details, ok := r.Context().Value(contextAccessTokenDetails).(*database.AccessToken)
	if ok {
		return details
	}
	http.Error(w, "access token not found", http.StatusUnauthorized)
	return nil
}

// authHeaderValues holds parsed emby authorization header
type authHeaderValues struct {
	device   string
	deviceID string
	token    string
	client   string
	version  string
}

// parseAuthHeader parses emby authorization header
func (j *Jellyfin) parseAuthHeader(r *http.Request) (*authHeaderValues, error) {
	errEmbyAuthHeader := errors.New("invalid or no authorization header provided")

	var authHeader string
	var found bool
	if authHeader = r.Header.Get("authorization"); authHeader != "" {
		found = true
	} else {
		if authHeader = r.Header.Get("x-emby-authorization"); authHeader != "" {
			found = true
		}
	}
	if !found || !strings.HasPrefix(authHeader, "MediaBrowser ") {
		return nil, errEmbyAuthHeader
	}

	// MediaBrowser Client="Jellyfin%20Media%20Player", Device="mbp", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Version="1.11.1", Token="aea78abca5744378b2a2badf710e7307"
	// MediaBrowser Device="Mac", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Token="826c2aa3596b47f2a386dd2811248649", Client="Infuse-Direct", Version="8.0.9"

	var result authHeaderValues
	authHeader = strings.TrimPrefix(authHeader, "MediaBrowser ")
	for part := range strings.SplitSeq(authHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			v := strings.Trim(kv[1], "\"")
			switch kv[0] {
			case "Device":
				result.device = v
			case "DeviceId":
				result.deviceID = v
			case "Token":
				result.token = v
			case "Client":
				result.client = v
			case "Version":
				result.version = v
			}
		} else {
			return nil, errEmbyAuthHeader
		}
	}
	return &result, nil
}

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
