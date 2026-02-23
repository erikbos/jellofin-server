package jellyfin

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"

	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

const (
	quickCodeValidDuration = 10 * time.Minute
)

// GET /QuickConnect/Enabled
//
// quickConnectEnabledHandler returns boolean whether quickconnect is enabled.
func (j *Jellyfin) quickConnectEnabledHandler(w http.ResponseWriter, r *http.Request) {
	serveJSON(j.quickConnectEnabled, w)
}

// POST /QuickConnect/Authorize
//
// quickConnectAuthorizeHandler authorizes a quick connect code for a user.
func (j *Jellyfin) quickConnectAuthorizeHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	if !j.quickConnectEnabled {
		apierror(w, "quickconnect is not enabled", http.StatusUnauthorized)
		return
	}

	queryparams := r.URL.Query()
	code := queryparams.Get("code")

	log.Printf("quickConnectAuthorizeHandler: user: %s, code: %s", reqCtx.User.ID, code)

	quickCode, err := j.repo.GetQuickConnectCodeByCode(r.Context(), code)
	if err != nil || quickCode == nil {
		log.Printf("quick connect code not found: %s", code)
		apierror(w, "quickconnect code unknown", http.StatusNotFound)
		return
	}
	// If code is too old we cannot authorize it
	if time.Since(quickCode.Created) > quickCodeValidDuration {
		log.Printf("quick connect code expired: %s", code)
		apierror(w, "quickconnect code expired", http.StatusNotFound)
		return
	}
	quickCode.Authorized = true
	quickCode.UserID = reqCtx.User.ID
	err = j.repo.UpsertQuickConnectCode(r.Context(), *quickCode)
	if err != nil {
		log.Printf("error updating quick connect code: %v", err)
		apierror(w, "internal server error", http.StatusInternalServerError)
		return
	}
	serveJSON(true, w)
}

// GET /QuickConnect/Connect
//
// quickConnectConnectHandler returns info about a quick connect code.
func (j *Jellyfin) quickConnectConnectHandler(w http.ResponseWriter, r *http.Request) {
	queryparams := r.URL.Query()
	secret := queryparams.Get("secret")
	log.Printf("quickConnectConnectHandler: secret: %s", secret)
	if secret == "" {
		apierror(w, "secret is required", http.StatusBadRequest)
		return
	}
	quickCode, err := j.repo.GetQuickConnectCodeBySecret(r.Context(), secret)
	if err != nil {
		log.Printf("error retrieving quick connect code: %v", err)
		apierror(w, "quickconnect secret unknown", http.StatusNotFound)
		return
	}
	d, _ := j.parseAuthHeader(r)
	response := j.makeJFQuickConnectResponse(*quickCode, d)
	serveJSON(response, w)
}

// POST /QuickConnect/Initiate
//
// quickConnectInitiateHandler initiates a quick connect code for a user.
func (j *Jellyfin) quickConnectInitiateHandler(w http.ResponseWriter, r *http.Request) {
	if !j.quickConnectEnabled {
		apierror(w, "quickconnect is not enabled", http.StatusUnauthorized)
		return
	}
	d, _ := j.parseAuthHeader(r)
	quickCode := model.QuickConnectCode{
		Code:     fmt.Sprintf("%06d", rand.Intn(1000000)),
		Created:  time.Now().UTC(),
		DeviceID: d.deviceID,
		Secret:   idhash.NewRandomID(),
	}
	if err := j.repo.UpsertQuickConnectCode(r.Context(), quickCode); err != nil {
		log.Printf("error upserting quick connect code: %v", err)
		apierror(w, "internal server error", http.StatusInternalServerError)
		return
	}
	response := j.makeJFQuickConnectResponse(quickCode, d)
	serveJSON(response, w)
}

func (j *Jellyfin) makeJFQuickConnectResponse(code model.QuickConnectCode, d *authSchemeValues) JFQuickconnectResponse {
	return JFQuickconnectResponse{
		Code:          code.Code,
		Secret:        code.Secret,
		Authenticated: code.Authorized,
		DateAdded:     code.Created,
		AppName:       d.client,
		AppVersion:    d.clientVersion,
		DeviceID:      d.deviceID,
		DeviceName:    d.device,
	}
}
