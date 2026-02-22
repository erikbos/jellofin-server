package jellyfin

import (
	"net/http"

	"github.com/erikbos/jellofin-server/database/model"
)

// /Devices
//
// devicesGetHandler returns a list of devices known to the server.
func (j *Jellyfin) devicesGetHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	// Get all access tokens for this user
	accessTokens, err := j.repo.GetAccessTokens(r.Context(), reqCtx.User.ID)
	if err != nil {
		apierror(w, "error retrieving devices", http.StatusInternalServerError)
		return
	}
	// Build device list based upon access tokens
	var devices []JFDeviceItem
	for _, t := range accessTokens {
		d := j.makeJFDeviceItem(t, reqCtx.User.Username)

		devices = append(devices, d)
	}
	response := JFDeviceInfoResponse{
		Items:            devices,
		StartIndex:       0,
		TotalRecordCount: len(devices),
	}
	serveJSON(response, w)
}

// /Devices DELETE
//
// devicesDeleteHandler handles deleting a device for the user.
func (j *Jellyfin) devicesDeleteHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	queryparams := r.URL.Query()
	id := queryparams.Get("id")
	if id == "" {
		apierror(w, "device id missing", http.StatusBadRequest)
		return
	}

	// Get all access tokens for this user
	accessTokens, err := j.repo.GetAccessTokens(r.Context(), reqCtx.User.ID)
	if err != nil {
		apierror(w, "error retrieving sessions", http.StatusInternalServerError)
		return
	}
	for _, t := range accessTokens {
		if t.DeviceId == id {
			err := j.repo.DeleteAccessToken(r.Context(), t.Token)
			if err != nil {
				apierror(w, "error deleting device", http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}
	apierror(w, "device not found", http.StatusNotFound)
}

// /Devices/Info
//
// devicesInfoHandler returns device info for the user based upon device id.
func (j *Jellyfin) devicesInfoHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	queryparams := r.URL.Query()
	id := queryparams.Get("id")
	if id == "" {
		apierror(w, "device id missing", http.StatusBadRequest)
		return
	}
	// Get all access tokens for this user
	accessTokens, err := j.repo.GetAccessTokens(r.Context(), reqCtx.User.ID)
	if err != nil {
		apierror(w, "error retrieving sessions", http.StatusInternalServerError)
		return
	}
	// Find access token for the requested device id
	var deviceToken model.AccessToken
	var found bool
	for _, t := range accessTokens {
		if t.DeviceId == id {
			deviceToken = t
			found = true
			break
		}
	}
	if !found {
		apierror(w, "Device not found", http.StatusNotFound)
		return
	}
	device := j.makeJFDeviceItem(deviceToken, reqCtx.User.Username)
	serveJSON(device, w)
}

// /Devices/Options
//
// devicesOptionsHandler returns device options for the user.
func (j *Jellyfin) devicesOptionsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}

	queryparams := r.URL.Query()
	id := queryparams.Get("id")
	if id == "" {
		apierror(w, "Device id missing", http.StatusBadRequest)
		return
	}

	type DevicesOptionsReponse struct {
		DeviceID         string `json:"DeviceId"`
		CustomName       string `json:"CustomName"`
		DisableAutoLogin bool   `json:"DisableAutoLogin"`
	}

	// Currently no options are supported, return empty response
	response := DevicesOptionsReponse{
		DeviceID:         id,
		CustomName:       reqCtx.Token.DeviceName,
		DisableAutoLogin: false,
	}
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFDeviceItem(accessToken model.AccessToken, user string) JFDeviceItem {
	return JFDeviceItem{
		ID:           accessToken.DeviceId,
		LastUserID:   accessToken.UserID,
		LastUserName: user,
		Name:         accessToken.DeviceName,
		AppName:      accessToken.ApplicationName,
		AppVersion:   accessToken.ApplicationVersion,
		Capabilities: JFSessionResponseCapabilities{
			PlayableMediaTypes:           []string{},
			SupportedCommands:            []string{},
			SupportsMediaControl:         false,
			SupportsPersistentIdentifier: true,
		},
		DateLastActivity: accessToken.LastUsed,
	}
}
