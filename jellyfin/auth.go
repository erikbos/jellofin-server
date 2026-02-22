package jellyfin

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
)

// Authentication specs:
// Emby - https://dev.emby.media/doc/restapi/User-Authentication.html.
// Jellyfin - https://gist.github.com/nielsvanvelzen/ea047d9028f676185832e51ffaf12a6f

type contextKey string

const (
	// requestContextKey key holds the requestContext struct in the request context
	requestContextKey contextKey = "requestContext"
)

// requestContext holds auth details for a request in flight
type requestContext struct {
	Token *model.AccessToken
	User  *model.User
}

// authSchemeValues holds parsed jellyfin authorization scheme values
type authSchemeValues struct {
	device        string
	deviceID      string
	token         string
	client        string
	clientVersion string
}

// POST /Users/AuthenticateByName
//
// usersAuthenticateByNameHandler authenticates a user by name
func (j *Jellyfin) usersAuthenticateByNameHandler(w http.ResponseWriter, r *http.Request) {
	var request JFAuthenticateUserByNameRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		apierror(w, ErrInvalidJSONPayload, http.StatusUnauthorized)
		return
	}

	if len(request.Username) == 0 || len(request.Pw) == 0 {
		apierror(w, "username and password required", http.StatusUnauthorized)
		return
	}

	// username is case insensitive
	request.Username = strings.ToLower(request.Username)

	// Get user from database
	user, err := j.repo.GetUser(r.Context(), request.Username)
	if err == nil {
		// User found, verify password
		if err = validatePassword(user.Password, request.Pw); err != nil {
			apierror(w, "Invalid username/password", http.StatusUnauthorized)
			return
		}
	}

	// Try to auto-register user if not found and auto-register is enabled
	if user == nil && j.autoRegister {
		user, err = j.createUser(r.Context(), request.Username, request.Pw)
		if err != nil || user == nil {
			apierror(w, "Failed to auto-register user", http.StatusInternalServerError)
			return
		}
	}
	// Update user's last login and last used time
	user.LastLogin = time.Now().UTC()
	user.LastUsed = time.Now().UTC()
	if err = j.repo.UpsertUser(r.Context(), user); err != nil {
		apierror(w, "Failed to update user last login & used time", http.StatusInternalServerError)
		return
	}

	// Try to get a few client details from auth header
	authHeader, err := j.parseAuthHeader(r)
	if err != nil || authHeader == nil {
		log.Printf("No valid authorization header or apikey found in request: %+v", r.Header)
		authHeader = &authSchemeValues{}
	}

	var token *model.AccessToken
	existingToken, err := j.repo.GetAccessTokenByDeviceID(r.Context(), authHeader.deviceID)
	if err == nil && existingToken != nil {
		log.Printf("Existing access token found for user %s deviceID: %s, token: %s\n", user.Username, authHeader.deviceID, existingToken.Token)
		token = existingToken
	} else {
		//Create a new access token authentication
		token = &model.AccessToken{
			Token:   rand.Text(),
			UserID:  user.ID,
			Created: time.Now().UTC(),
			// Remaining fields will be populated by updateTokenDetails()
		}
		log.Printf("Creating new token for user %s deviceID: %s, token: %s\n", user.Username, authHeader.deviceID, token.Token)
	}

	// Populate token details from auth header if available
	token.LastUsed = time.Now().UTC()
	updateTokenDetails(token, r, authHeader)

	err = j.repo.UpsertAccessToken(r.Context(), *token)
	if err != nil {
		apierror(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	response := JFAuthenticateByNameResponse{
		AccessToken: token.Token,
		SessionInfo: j.makeJFSessionInfo(token, user.Username),
		ServerId:    j.serverID,
		User:        j.makeJFUser(r.Context(), user),
	}
	log.Printf("User %s authenticated successfully, deviceid: %s, client: %s, token: %s\n", user.Username, token.DeviceId, token.ApplicationName, token.Token)
	serveJSON(response, w)
}

// updateTokenDetails updates token details from request in case of any changed fields.
func updateTokenDetails(t *model.AccessToken, r *http.Request, authHeader *authSchemeValues) bool {
	var changed bool

	if authHeader != nil {
		if authHeader.device != t.DeviceName {
			t.DeviceName = authHeader.device
			changed = true
		}
		if authHeader.deviceID != t.DeviceId {
			t.DeviceId = authHeader.deviceID
			changed = true
		}
		if authHeader.client != t.ApplicationName {
			t.ApplicationName = authHeader.client
			changed = true
		}
		if authHeader.clientVersion != t.ApplicationVersion {
			t.ApplicationVersion = authHeader.clientVersion
			changed = true
		}
	}
	remoteAddress, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteAddress = r.RemoteAddr
	}
	if t.RemoteAddress != remoteAddress {
		t.RemoteAddress = remoteAddress
		changed = true
	}
	return changed
}

// parseAuthHeader parses jellyfin-formated authorization header
func (j *Jellyfin) parseAuthHeader(r *http.Request) (*authSchemeValues, error) {
	errAuthHeader := errors.New("invalid or no authorization header provided")

	var authHeader string
	var found bool
	if authHeader = r.Header.Get("authorization"); authHeader != "" {
		found = true
	} else {
		// todo: remove after Jellyfin 11.12 release
		if authHeader = r.Header.Get("x-emby-authorization"); authHeader != "" {
			found = true
		}
	}
	if !found ||
		!(strings.HasPrefix(authHeader, "MediaBrowser ") || strings.HasPrefix(authHeader, "Emby ")) {
		return nil, errAuthHeader
	}

	// MediaBrowser Client="Jellyfin%20Media%20Player", Device="mbp", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Version="1.11.1", Token="aea78abca5744378b2a2badf710e7307"
	// MediaBrowser Device="Mac", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Token="826c2aa3596b47f2a386dd2811248649", Client="Infuse-Direct", Version="8.0.9"
	// MediaBrowser Client="Jellyflix", Device="MacBookPro18,1", DeviceId="11C750BF-4CE0-54C1-89B8-075C36A97A17", Version="1.0.0", Token="ba644327ee654ef5ac7116367da81fe3"]
	// MediaBrowser Client="JellyWatch", Device="Android", DeviceId="3a9112ee-8a68-4bbb-89dc-2d1ac008f4c7", Version="1.6.REV-90"
	// MediaBrowser Version=1.4.1, DeviceId=iOS_11798B04-7824-46EE-B608-AB4BEB956AD2, Device=iPhone, Client=Swiftfin iOS, Token=LVLWISEHBBEKJDQJURZCCAEJCS

	// Try parsing quoted format: key="value", key="value"
	kvMatchQuoted := `(\w+)="(.*?)"`
	reQuoted := regexp.MustCompile(kvMatchQuoted)
	matches := reQuoted.FindAllStringSubmatch(authHeader, -1)

	if len(matches) == 0 {
		// Try parsing unquoted format: key=value, key=value
		kvMatchUnquoted := `(\w+)=([^,]+)`
		reUnquoted := regexp.MustCompile(kvMatchUnquoted)
		matches = reUnquoted.FindAllStringSubmatch(authHeader, -1)
	}

	var result authSchemeValues
	for _, match := range matches {
		if len(match) == 3 {
			value := strings.TrimSpace(match[2])
			switch match[1] {
			case "Client":
				result.client = value
			case "Version":
				result.clientVersion = value
			case "Device":
				result.device = value
			case "DeviceId":
				result.deviceID = value
			case "Token":
				result.token = value
			}
		}
	}
	return &result, nil
}

// authMiddleware validates auth token, token can be provided in various headers
func (j *Jellyfin) authmiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var requestToken string
		found := false

		embyHeader, err := j.parseAuthHeader(r)
		if err == nil && embyHeader.token != "" {
			requestToken = embyHeader.token
			found = true
		}
		// todo: remove after Jellyfin 11.12 release
		if t := r.Header.Get("x-emby-token"); t != "" {
			requestToken = t
			found = true
		}
		// todo: remove after Jellyfin 11.12 release
		if t := r.Header.Get("x-mediabrowser-token"); t != "" {
			requestToken = t
			found = true
		}
		if t := r.URL.Query().Get("apiKey"); t != "" {
			requestToken = t
			found = true
		}
		// Deprecated: needed for VidhubPro & Streamyfin's embedded VLC
		// todo: remove after Jellyfin 11.12 release
		if t := r.URL.Query().Get("api_key"); t != "" {
			requestToken = t
			found = true
		}
		if !found {
			// log.Printf("no token found in request headers: %+v", r.Header)
			apierror(w, "no token provided", http.StatusUnauthorized)
			return
		}

		token, err := j.repo.GetAccessToken(r.Context(), requestToken)
		if err != nil {
			log.Printf("invalid access token: %s, %s", requestToken, err)
			apierror(w, "invalid access token", http.StatusUnauthorized)
			return
		}
		// Update token details from auth header if changed and store back to database
		if updateTokenDetails(token, r, embyHeader) {
			err = j.repo.UpsertAccessToken(r.Context(), *token)
			if err != nil {
				log.Printf("failed to update access token details: %s", err)
			}
		}
		user, err := j.repo.GetUserByID(r.Context(), token.UserID)
		if err != nil {
			log.Printf("Error retrieving user for access token from db for token: %s, userID: %s: %s\n", requestToken, token.UserID, err)
			apierror(w, "invalid access token", http.StatusUnauthorized)
			return
		}
		requestCtx := &requestContext{
			Token: token,
			User:  user,
		}
		ctx := context.WithValue(r.Context(), requestContextKey, requestCtx)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getRequestCtx returns access token and user details from the request context populated by authmiddleware()
//
// if not found sends an HTTP unauthorized error
func (j *Jellyfin) getRequestCtx(w http.ResponseWriter, r *http.Request) *requestContext {
	// Ctx should have been populated by authmiddleware()
	if details, ok := r.Context().Value(requestContextKey).(*requestContext); ok {
		return details
	}
	apierror(w, "access token not found", http.StatusUnauthorized)
	return nil
}
