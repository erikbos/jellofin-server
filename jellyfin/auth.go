package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/erikbos/jellofin/database"
)

// authSchemeValues holds parsed emby authorization scheme values
type authSchemeValues struct {
	device   string
	deviceID string
	token    string
	client   string
	version  string
}

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
		Id:                 sessionID,
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

	accesstoken, err := j.db.AccessTokenRepo.Generate(user.ID)
	if err != nil {
		http.Error(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	response := JFAuthenticateByNameResponse{
		AccessToken: accesstoken,
		SessionInfo: session,
		ServerId:    serverID,
		User:        makeJFUser(user),
	}
	serveJSON(response, w)
}

// parseAuthHeader parses emby authorization header
func (j *Jellyfin) parseAuthHeader(r *http.Request) (*authSchemeValues, error) {
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
	// MediaBrowser Client="Jellyflix", Device="MacBookPro18,1", DeviceId="11C750BF-4CE0-54C1-89B8-075C36A97A17", Version="1.0.0", Token="ba644327ee654ef5ac7116367da81fe3"]

	kvMatch := `(\w+)="(.*?)"`
	re := regexp.MustCompile(kvMatch)
	matches := re.FindAllStringSubmatch(authHeader, -1)

	var result authSchemeValues
	for _, match := range matches {
		if len(match) == 3 {
			switch match[1] {
			case "Client":
				result.client = match[2]
			case "Device":
				result.device = match[2]
			case "DeviceId":
				result.deviceID = match[2]
			case "Version":
				result.version = match[2]
			case "Token":
				result.token = match[2]
			}
		}
	}
	if result.client == "" || result.device == "" || result.deviceID == "" {
		return nil, errEmbyAuthHeader
	}
	return &result, nil
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
		if t := r.Header.Get("x-emby-token"); t != "" {
			token = t
			found = true
		}
		if t := r.Header.Get("x-mediabrowser-token"); t != "" {
			token = t
			found = true
		}
		// Needed for Streamyfin's embedded VLC
		if t := r.URL.Query().Get("api_key"); t != "" {
			token = t
			found = true
		}
		if !found {
			// log.Printf("no token found in request headers: %+v", r.Header)
			http.Error(w, "no token provided", http.StatusUnauthorized)
			return
		}

		tokendetails, err := j.db.AccessTokenRepo.Get(token)
		if err != nil {
			log.Printf("invalid access token: %s", err)
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

// GET /QuickConnect/Enabled
//
// quickConnectEnabledHandler returns boolean whether quickconnect is enabled.
func (j *Jellyfin) quickConnectEnabledHandler(w http.ResponseWriter, r *http.Request) {
	serveJSON(false, w)
}

// POST /QuickConnect/Authorize
//
// usersAuthenticateByNameHandler stores quickconnect code of an authenticated user.
func (j *Jellyfin) quickConnectAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	userID := queryparams.Get("userId")
	code := queryparams.Get("code")

	if userID != accessToken.UserID {
		http.Error(w, "userID does not match access token", http.StatusForbidden)
		return
	}
	log.Printf("quickConnectAuthorizeHandler: user: %s, code: %s", userID, code)
	http.Error(w, "quickconnect code not implemented", http.StatusUnauthorized)
}

// GET /QuickConnect/Connect
//
// quickConnectConnectHandler returns info about a quick connect code.
func (j *Jellyfin) quickConnectConnectHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "quickconnect code not found", http.StatusNotFound)
}

// POST /QuickConnect/Initiate
//
// usersAuthenticateByNameHandler authenticates a user by quick connect code.
func (j *Jellyfin) quickConnectInitiateHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("quickConnectInitiateHandler: %+v", r)
	http.Error(w, "quickconnect code not implemented", http.StatusUnauthorized)
}
