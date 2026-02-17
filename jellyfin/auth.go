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

	"golang.org/x/crypto/bcrypt"

	"github.com/erikbos/jellofin-server/database/model"
)

// Authentication specs:
// Emby - https://dev.emby.media/doc/restapi/User-Authentication.html.
// Jellyfin - https://gist.github.com/nielsvanvelzen/ea047d9028f676185832e51ffaf12a6f

type contextKey string

const (
	// Context key holds access token details of a request in flight
	contextAccessTokenDetails contextKey = "AccessTokenDetails"
)

// authSchemeValues holds parsed emby authorization scheme values
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
		if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(request.Pw)); err != nil {
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
	embyHeader, err := j.parseAuthHeader(r)
	if err != nil || embyHeader == nil {
		log.Printf("No valid emby auth header found in request: %+v", r.Header)
		embyHeader = &authSchemeValues{}
	}

	// We always create a new access token on authentication
	newToken := model.AccessToken{
		Token:    rand.Text(),
		User:     *user,
		Created:  time.Now().UTC(),
		LastUsed: time.Now().UTC(),
	}
	// Populate token details from auth header if available
	updateTokenDetails(&newToken, r, embyHeader)

	err = j.repo.UpsertAccessToken(r.Context(), newToken)
	if err != nil {
		apierror(w, "Failed to generate access token", http.StatusInternalServerError)
		return
	}

	response := JFAuthenticateByNameResponse{
		AccessToken: newToken.Token,
		SessionInfo: j.makeJFSessionInfo(newToken, user.Username),
		ServerId:    j.serverID,
		User:        j.makeJFUser(r.Context(), user),
	}
	serveJSON(response, w)
}

// updateTokenDetails updates token details from request in case of any changed fields.
func updateTokenDetails(tokendetails *model.AccessToken, r *http.Request, embyHeader *authSchemeValues) bool {
	var changed bool

	if embyHeader != nil {
		if tokendetails.DeviceName != embyHeader.device {
			tokendetails.DeviceName = embyHeader.device
			changed = true
		}
		if tokendetails.DeviceId != embyHeader.deviceID {
			tokendetails.DeviceId = embyHeader.deviceID
			changed = true
		}
		if tokendetails.ApplicationName != embyHeader.client {
			tokendetails.ApplicationName = embyHeader.client
			changed = true
		}
		if tokendetails.ApplicationVersion != embyHeader.clientVersion {
			tokendetails.ApplicationVersion = embyHeader.clientVersion
			changed = true
		}
	}
	remoteAddress, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteAddress = r.RemoteAddr
	}
	if tokendetails.RemoteAddress != remoteAddress {
		tokendetails.RemoteAddress = remoteAddress
		changed = true
	}
	return changed
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
	if !found ||
		!(strings.HasPrefix(authHeader, "MediaBrowser ") || strings.HasPrefix(authHeader, "Emby ")) {
		return nil, errEmbyAuthHeader
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
		var token string
		found := false

		embyHeader, err := j.parseAuthHeader(r)
		if err == nil && embyHeader.token != "" {
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
		if t := r.URL.Query().Get("apiKey"); t != "" {
			token = t
			found = true
		}
		// Deprecated: needed for VidhubPro & Streamyfin's embedded VLC
		if t := r.URL.Query().Get("api_key"); t != "" {
			token = t
			found = true
		}
		if !found {
			// log.Printf("no token found in request headers: %+v", r.Header)
			apierror(w, "no token provided", http.StatusUnauthorized)
			return
		}

		tokendetails, err := j.repo.GetAccessToken(r.Context(), token)
		if err != nil {
			log.Printf("invalid access token: %s, %s", token, err)
			apierror(w, "invalid access token", http.StatusUnauthorized)
			return
		}

		// Update token details from auth header if changed and store back to database
		if updateTokenDetails(tokendetails, r, embyHeader) {
			err = j.repo.UpsertAccessToken(r.Context(), *tokendetails)
			if err != nil {
				log.Printf("failed to update access token details: %s", err)
			}
		}

		ctx := context.WithValue(r.Context(), contextAccessTokenDetails, tokendetails)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getAccessTokenDetails returns access token details from the
// request context populated by authmiddleware()
//
// if not found sends an HTTP unauthorized error
func (j *Jellyfin) getAccessTokenDetails(w http.ResponseWriter, r *http.Request) *model.AccessToken {
	// Ctx should have been populated by authmiddleware()
	details, ok := r.Context().Value(contextAccessTokenDetails).(*model.AccessToken)
	if ok {
		return details
	}
	apierror(w, "access token not found", http.StatusUnauthorized)
	return nil
}
