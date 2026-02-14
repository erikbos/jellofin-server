package jellyfin

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

const (
	ErrUserIDNotFound = "userid not found"
)

// GET /Users
//
// usersGetHandler returns all users
func (j *Jellyfin) usersGetHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	var administratorRequest bool
	// Does access token have have admininistrator privileges?
	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err == nil {
		administratorRequest = dbuser.Properties.Admin
	}
	// Get all users
	users, err := j.repo.GetAllUsers(r.Context())
	if err != nil {
		apierror(w, "failed to get users", http.StatusInternalServerError)
		return
	}
	response := make([]JFUser, 0, len(users))
	for _, dbuser := range users {
		user := j.makeJFUser(r.Context(), &dbuser)
		// Include user if requestor has administrator privileges
		if administratorRequest ||
			// Include user if requestor is the user
			accessToken.UserID == user.Id ||
			// Include user if user is public
			!user.Policy.IsHidden {
			response = append(response, user)
		}
	}
	serveJSON(response, w)
}

// POST /Users?userId=u_VvbHefPib3oOdj8d4hUW
//
// usersPostHandler updates a user
func (j *Jellyfin) usersPostHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	vars := mux.Vars(r)
	userID := vars["userId"]
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	if accessToken.UserID != userID && !dbuser.Properties.Admin {
		apierror(w, "forbidden to update user", http.StatusForbidden)
		return
	}
	var req JFUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	dbuser.Username = req.Name
	parseJFUserPolicy(req.Policy, &dbuser.Properties)
	parseJFUserConfiguration(req.Configuration, &dbuser.Properties)
	if err = j.repo.UpsertUser(r.Context(), dbuser); err != nil {
		apierror(w, "failed to update user", http.StatusInternalServerError)
		return
	}
	response := j.makeJFUser(r.Context(), dbuser)
	serveJSON(response, w)
}

// GET /Users/Me
//
// usersMeHandler returns the current user
func (j *Jellyfin) usersMeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	dbuser, err := j.repo.GetUserByID(r.Context(), accessToken.UserID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	response := j.makeJFUser(r.Context(), dbuser)
	serveJSON(response, w)
}

// GET /Users/{user}
//
// usersHandler returns a user
func (j *Jellyfin) usersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	vars := mux.Vars(r)
	userID := vars["userid"]
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	// Only allow access to user if requestor is the user or user is public
	if !dbuser.Properties.Public && accessToken.UserID != userID {
		apierror(w, "forbidden to access user", http.StatusForbidden)
		return
	}
	response := j.makeJFUser(r.Context(), dbuser)
	serveJSON(response, w)
}

// GET /Users/Public
//
// usersHandler returns list of public users
func (j *Jellyfin) usersPublicHandler(w http.ResponseWriter, r *http.Request) {
	users, err := j.repo.GetAllUsers(r.Context())
	if err != nil {
		apierror(w, "failed to get users", http.StatusInternalServerError)
		return
	}
	var response []JFUser
	for _, user := range users {
		if user.Properties.Public {
			response = append(response, j.makeJFUser(r.Context(), &user))
		}
	}
	serveJSON(response, w)
}

// POST /Users/{user}/Configuration
//
// usersConfigurationHandler updates user configuration
func (j *Jellyfin) usersConfigurationHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	vars := mux.Vars(r)
	userID := vars["userid"]
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	if accessToken.UserID != userID && !dbuser.Properties.Admin {
		apierror(w, "forbidden to update user configuration", http.StatusForbidden)
		return
	}
	log.Printf("Looking up user by id: %s\n", userID)
	var req JFUserConfiguration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Updating user configuration %+v", req)
	parseJFUserConfiguration(req, &dbuser.Properties)
	if err = j.repo.UpsertUser(r.Context(), dbuser); err != nil {
		apierror(w, "failed to update user configuration", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /Users/{user}/Policy
//
// usersPolicyHandler updates user policy
func (j *Jellyfin) usersPolicyHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	vars := mux.Vars(r)
	userID := vars["userid"]
	log.Printf("Looking up user by id: %s\n", userID)
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	if accessToken.UserID != userID && !dbuser.Properties.Admin {
		apierror(w, "forbidden to update user policy", http.StatusForbidden)
		return
	}
	var req JFUserPolicy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	log.Printf("Updating user policy %+v", req)
	parseJFUserPolicy(req, &dbuser.Properties)
	if err = j.repo.UpsertUser(r.Context(), dbuser); err != nil {
		apierror(w, "failed to update user policy", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /Users/New
//
// usersNewItemsHandler creates a new user with the provided username and password
func (j *Jellyfin) usersNewItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	var req JFUserNewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" || req.Password == "" {
		apierror(w, "username and password are required", http.StatusBadRequest)
		return
	}
	if req.Name == "New" || req.Name == "Me" {
		apierror(w, "invalid username", http.StatusBadRequest)
		return
	}
	dbuser, err := j.createUser(r.Context(), req.Name, req.Password)
	if err != nil {
		apierror(w, "failed to create user", http.StatusInternalServerError)
		return
	}
	response := j.makeJFUser(r.Context(), dbuser)
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFUser(ctx context.Context, user *model.User) JFUser {
	u := JFUser{
		Id:                        user.ID,
		Name:                      user.Username,
		ServerId:                  j.serverID,
		HasPassword:               true,
		HasConfiguredPassword:     true,
		HasConfiguredEasyPassword: false,
		EnableAutoLogin:           false,
		LastLoginDate:             time.Now().UTC(),
		LastActivityDate:          time.Now().UTC(),
		Configuration:             makeJFUserConfiguration(user),
		Policy:                    makeJFUserPolicy(user),
	}
	// Set imagetag if user has an image
	if _, err := j.repo.HasImage(ctx, user.ID, ImageTypeProfile); err == nil {
		u.PrimaryImageTag = user.ID
	}
	return u
}

// makeJFUserConfiguration creates a JFUserConfiguration from the user properties
func makeJFUserConfiguration(user *model.User) JFUserConfiguration {
	return JFUserConfiguration{
		CastReceiverId:             "F007D354",
		GroupedFolders:             []string{},
		LatestItemsExcludes:        []string{},
		MyMediaExcludes:            user.Properties.MyMediaExcludes,
		OrderedViews:               user.Properties.OrderedViews,
		SubtitleMode:               "Default",
		PlayDefaultAudioTrack:      true,
		RememberAudioSelections:    true,
		RememberSubtitleSelections: true,
	}
}

// parseJFUserConfiguration parses the user configuration from the request and updates the user properties
func parseJFUserConfiguration(config JFUserConfiguration, props *model.UserProperties) {
	props.MyMediaExcludes = config.MyMediaExcludes
	props.OrderedViews = config.OrderedViews
}

func makeJFUserPolicy(user *model.User) JFUserPolicy {
	return JFUserPolicy{
		AccessSchedules:                  []string{},
		AllowedTags:                      []string{},
		BlockedChannels:                  []string{},
		BlockedMediaFolders:              []string{},
		BlockedTags:                      []string{},
		BlockUnratedItems:                []string{},
		EnabledChannels:                  []string{},
		EnabledDevices:                   []string{},
		EnabledFolders:                   user.Properties.EnabledFolders,
		EnableContentDeletionFromFolders: []string{},
		EnableContentDownloading:         !user.Properties.BlockDownload,
		EnableMediaPlayback:              true,
		EnableRemoteAccess:               true,
		EnableAllDevices:                 true,
		EnableAllFolders:                 true,
		AuthenticationProviderID:         "DefaultAuthenticationProvider",
		PasswordResetProviderID:          "DefaultPasswordResetProvider",
		SyncPlayAccess:                   "CreateAndJoinGroups",
		IsAdministrator:                  user.Properties.Admin,
		IsDisabled:                       user.Properties.Disabled,
		IsHidden:                         !user.Properties.Public,
	}
}

func parseJFUserPolicy(policy JFUserPolicy, props *model.UserProperties) {
	props.EnabledFolders = policy.EnabledFolders
	props.BlockDownload = policy.EnableContentDownloading
	props.Admin = policy.IsAdministrator
	props.Disabled = policy.IsDisabled
	props.Public = !policy.IsHidden
}

// createUser creates a new user in the database
func (j *Jellyfin) createUser(context context.Context, username, password string) (*model.User, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	modelUser := &model.User{
		ID:       idhash.IdHash(username),
		Username: strings.ToLower(username),
		Password: string(hashedPassword),
		Created:  time.Now().UTC(),
		LastUsed: time.Now().UTC(),
	}
	if err = j.repo.UpsertUser(context, modelUser); err != nil {
		return nil, err
	}
	return modelUser, nil
}
