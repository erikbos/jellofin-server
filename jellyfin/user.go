package jellyfin

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	"image/png"
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
	// Get all users
	users, err := j.repo.GetAllUsers(r.Context())
	if err != nil {
		apierror(w, "failed to get users", http.StatusInternalServerError)
		return
	}

	queryParams := r.URL.Query()
	filterIsHidden := queryParams.Get("isHidden")
	filterIsDisabled := queryParams.Get("isDisabled")

	response := make([]JFUser, 0, len(users))
	for _, dbuser := range users {
		user := j.makeJFUser(r.Context(), &dbuser)
		// Apply isHidden filter if specified
		if filterIsHidden != "" {
			wantHidden := filterIsHidden == "true"
			if user.Policy.IsHidden != wantHidden {
				continue
			}
		}
		// Apply isDisabled filter if specified
		if filterIsDisabled != "" {
			wantDisabled := filterIsDisabled == "true"
			if user.Policy.IsDisabled != wantDisabled {
				continue
			}
		}
		// Include user if requestor is the user
		if accessToken.User.ID == user.Id ||
			// Include user if requestor has administrator privileges
			accessToken.User.Properties.Admin ||
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
	queryparams := r.URL.Query()
	userID := queryparams.Get("userId")
	// log.Printf("usersPostHandler: accessToken.User.ID=%s, userID=%s\n", accessToken.User.ID, userID)
	// log.Printf("usersPostHandler: Admins=%v\n", accessToken.User.Properties.Admin)

	// Only allow if requester is an administrator or the user themselve
	if !accessToken.User.Properties.Admin && accessToken.User.ID != userID {
		apierror(w, "forbidden to update user policy", http.StatusForbidden)
		return
	}
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	var req JFUser
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	dbuser.Username = strings.ToLower(req.Name)
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
	response := j.makeJFUser(r.Context(), &accessToken.User)
	serveJSON(response, w)
}

// GET /Users/{user}
//
// userGetHandler returns a user
func (j *Jellyfin) userGetHandler(w http.ResponseWriter, r *http.Request) {
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
	// Allow if requester is an administrator or user themselves
	if !accessToken.User.Properties.Admin && accessToken.User.ID != userID {
		apierror(w, "forbidden to access user", http.StatusForbidden)
		return
	}
	response := j.makeJFUser(r.Context(), dbuser)
	serveJSON(response, w)
}

// DELETE /Users/{user}
//
// userDeleteHandler deletes a user
func (j *Jellyfin) userDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	vars := mux.Vars(r)
	userID := vars["userid"]
	// Only allow if requester is an administrator and not deleting themselves
	if !accessToken.User.Properties.Admin || accessToken.User.ID == userID {
		apierror(w, "forbidden to delete user", http.StatusForbidden)
		return
	}
	if err := j.repo.DeleteUser(r.Context(), userID); err != nil {
		apierror(w, "failed to delete user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	response := []JFUser{}
	for _, user := range users {
		if !user.Properties.IsHidden {
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
	// Only allow if requester is an administrator or the user themselves
	if !accessToken.User.Properties.Admin && accessToken.User.ID != userID {
		apierror(w, "forbidden to update user policy", http.StatusForbidden)
		return
	}
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	// log.Printf("Looking up user by id: %s\n", userID)
	var req JFUserConfiguration
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// log.Printf("Updating user configuration %+v", req)
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
	// Only allow if requester is an administrator or the user themselves
	if !accessToken.User.Properties.Admin && accessToken.User.ID != userID {
		apierror(w, "forbidden to update user policy", http.StatusForbidden)
		return
	}
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	var req JFUserPolicy
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	// log.Printf("Updating user policy %+v", req)
	parseJFUserPolicy(req, &dbuser.Properties)
	if err = j.repo.UpsertUser(r.Context(), dbuser); err != nil {
		apierror(w, "failed to update user policy", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /Users/Password
//
// usersPasswordHandler updates user password
func (j *Jellyfin) usersPasswordHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	queryparams := r.URL.Query()
	userID := queryparams.Get("userId")
	// Only allow if requester is an administrator or the user themselves
	if !accessToken.User.Properties.Admin && accessToken.User.ID != userID {
		apierror(w, "forbidden to update user password", http.StatusForbidden)
		return
	}
	dbuser, err := j.repo.GetUserByID(r.Context(), userID)
	if err != nil {
		apierror(w, ErrUserIDNotFound, http.StatusNotFound)
		return
	}
	var req JFUserPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.NewPw == "" {
		apierror(w, "new password is required", http.StatusBadRequest)
		return
	}
	hashedPassword, err := hashPassword(req.NewPw)
	if err != nil {
		apierror(w, "failed to hash password", http.StatusInternalServerError)
		return
	}
	dbuser.Password = hashedPassword
	if err = j.repo.UpsertUser(r.Context(), dbuser); err != nil {
		apierror(w, "failed to update user password", http.StatusInternalServerError)
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
		Configuration:             makeJFUserConfiguration(user),
		Policy:                    makeJFUserPolicy(user),
	}
	if !user.LastLogin.IsZero() {
		u.LastLoginDate = &user.LastLogin
	}
	if !user.LastUsed.IsZero() {
		u.LastActivityDate = &user.LastUsed
	}
	// Set imagetag if user has an image
	if _, err := j.repo.HasImage(ctx, user.ID, imageTypeProfile); err == nil {
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

// makeJFUserPolicy creates a JFUserPolicy from the user properties
func makeJFUserPolicy(user *model.User) JFUserPolicy {
	return JFUserPolicy{
		AccessSchedules:                  []string{},
		AllowedTags:                      user.Properties.AllowTags,
		BlockedChannels:                  []string{},
		BlockedMediaFolders:              []string{},
		BlockedTags:                      user.Properties.BlockTags,
		BlockUnratedItems:                []string{},
		EnabledChannels:                  []string{},
		EnabledDevices:                   []string{},
		EnabledFolders:                   user.Properties.EnabledFolders,
		EnableContentDeletionFromFolders: []string{},
		EnableContentDownloading:         user.Properties.EnableDownloads,
		EnableMediaPlayback:              true,
		EnableRemoteAccess:               true,
		EnableAllDevices:                 true,
		EnableAllFolders:                 user.Properties.EnableAllFolders,
		AuthenticationProviderID:         "DefaultAuthenticationProvider",
		PasswordResetProviderID:          "DefaultPasswordResetProvider",
		SyncPlayAccess:                   "CreateAndJoinGroups",
		IsAdministrator:                  user.Properties.Admin,
		IsDisabled:                       user.Properties.Disabled,
		IsHidden:                         user.Properties.IsHidden,
	}
}

// parseJFUserPolicy parses the user policy from the request and updates the user properties
func parseJFUserPolicy(policy JFUserPolicy, props *model.UserProperties) {
	props.AllowTags = policy.AllowedTags
	props.BlockTags = policy.BlockedTags
	props.EnableAllFolders = policy.EnableAllFolders
	props.EnabledFolders = policy.EnabledFolders
	props.EnableDownloads = policy.EnableContentDownloading
	props.Admin = policy.IsAdministrator
	props.Disabled = policy.IsDisabled
	props.IsHidden = policy.IsHidden
}

// createUser creates a new user in the database
func (j *Jellyfin) createUser(context context.Context, username, password string) (*model.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	modelUser := &model.User{
		ID:       idhash.NewRandomID(),
		Username: strings.ToLower(username),
		Password: string(hashedPassword),
		Created:  time.Now().UTC(),
		Properties: model.UserProperties{
			IsHidden:         true,
			EnableAllFolders: true,
			EnableDownloads:  true,
		},
	}
	if err = j.repo.UpsertUser(context, modelUser); err != nil {
		return nil, err
	}
	// Generate and store identicon avatar for this new user
	if avatarMetadata, avatar, err := generateIdenticon(username); err == nil {
		_ = j.repo.StoreImage(context, modelUser.ID, imageTypeProfile, avatarMetadata, avatar)
	}
	return modelUser, nil
}

// hashPassword hashes a password
func hashPassword(password string) (string, error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashedPassword), nil
}

// generateIdenticon generates a PNG avatar based on seed string
func generateIdenticon(seed string) (model.ImageMetadata, []byte, error) {
	const size int = 512

	hash := sha256.Sum256([]byte(seed))
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// background
	bg := color.RGBA{240, 240, 240, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	// derive color
	avatarColor := color.RGBA{R: hash[0], G: hash[1], B: hash[2], A: 255}

	grid := 5        // draw 5x5 grid, mirroring left half to right
	s := size / grid // size of each cell in the grid
	index := 3       // Skip first 3 bytes of hash as they are used for color
	for y := range grid {
		for x := 0; x < (grid+1)/2; x++ {
			if hash[index]%2 == 0 {
				// left cell
				rect := image.Rect(x*s, y*s, (x+1)*s, (y+1)*s)
				draw.Draw(img, rect, &image.Uniform{avatarColor}, image.Point{}, draw.Src)
				// mirrored right cell
				mirrorX := grid - 1 - x
				rectMirror := image.Rect(mirrorX*s, y*s, (mirrorX+1)*s, (y+1)*s)
				draw.Draw(img, rectMirror, &image.Uniform{avatarColor}, image.Point{}, draw.Src)
			}
			index++
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return model.ImageMetadata{}, nil, err
	}
	avatarMeta := model.ImageMetadata{
		MimeType: "image/png",
		FileSize: buf.Len(),
		Etag:     idhash.HashBytes(buf.Bytes()),
		Updated:  time.Now().UTC(),
	}
	return avatarMeta, buf.Bytes(), nil
}
