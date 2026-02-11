package jellyfin

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"time"
)

const (
	serverVersion = "10.10.11"
)

// /System/Endpoint
//
// systemEndpointHandler returns endpoint info
func (j *Jellyfin) systemEndpointHandler(w http.ResponseWriter, r *http.Request) {
	// We return false as we do not support local network discovery / optimization
	response := JFSystemEndpointResponse{
		IsLocal:     false,
		IsInNetwork: false,
	}
	serveJSON(response, w)
}

// /System/Info
//
// systemInfoHandler returns server info
func (j *Jellyfin) systemInfoHandler(w http.ResponseWriter, r *http.Request) {
	response := JFSystemInfoResponse{
		Id:                         j.serverID,
		HasPendingRestart:          false,
		IsShuttingDown:             false,
		SupportsLibraryMonitor:     true,
		WebSocketPortNumber:        8096,
		CompletedInstallations:     []string{},
		CanSelfRestart:             true,
		CanLaunchWebBrowser:        false,
		ProgramDataPath:            "/jellyfin",
		WebPath:                    "/jellyfin/web",
		ItemsByNamePath:            "/jellyfin/metadata",
		CachePath:                  "/jellyfin/cache",
		LogPath:                    "/jellyfin/log",
		InternalMetadataPath:       "/jellyfin/metadata",
		TranscodingTempPath:        "/jellyfin/cache/transcodes",
		EncoderLocation:            "System",
		HasUpdateAvailable:         false,
		LocalAddress:               localAddress(r),
		OperatingSystem:            runtime.GOOS,
		OperatingSystemDisplayName: runtime.GOOS,
		ServerName:                 j.serverName,
		SystemArchitecture:         runtime.GOARCH,
		Version:                    serverVersion,
		CastReceiverApplications: []CastReceiverApplication{
			{
				Id:   "F007D354",
				Name: "Stable",
			},
			{
				Id:   "6F511C87",
				Name: "Unstable",
			},
		},
	}
	serveJSON(response, w)
}

// /System/Info/Public
//
// systemInfoPublicHandler returns basic server info
func (j *Jellyfin) systemInfoPublicHandler(w http.ResponseWriter, r *http.Request) {
	response := JFSystemInfoPublicResponse{
		Id:           j.serverID,
		LocalAddress: localAddress(r),
		// Jellyfin ios native client checks for exact productname so we have to return the same name..
		// https://github.com/jellyfin/jellyfin-expo/blob/7dedbc72fb53fc4b83c3967c9a8c6c071916425b/utils/ServerValidator.js#L82C49-L82C64
		ProductName:            "Jellyfin Server",
		ServerName:             j.serverName,
		Version:                serverVersion,
		StartupWizardCompleted: true,
	}
	serveJSON(response, w)
}

// /health
//
// healthHandler returns health status
func (j *Jellyfin) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("cache-control", "no-cache, no-store")
	w.Write([]byte("Healthy"))
}

// /System/Ping
//
// systemPingHandler returns static string
func (j *Jellyfin) systemPingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("\"Jellyfin Server\""))
}

// /Plugins
//
// pluginsHandler returns emply plugin list, we do not support plugins at the moment
func (j *Jellyfin) pluginsHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFPluginResponse{}
	serveJSON(response, w)
}

// /Playback/BitrateTest?size=500000
//
// playbackBitrateTestHandler returns random data of requested size for bitrate testing
func (j *Jellyfin) playbackBitrateTestHandler(w http.ResponseWriter, r *http.Request) {
	size := int64(102400)              // Default to 100 KB if size is not specified
	maxSize := int64(20 * 1024 * 1024) // 20 MB safety cap

	if s := r.URL.Query().Get("size"); s != "" {
		var err error
		size, err = strconv.ParseInt(s, 10, 64)
		if err != nil || size < 0 || size > maxSize {
			http.Error(w, "invalid size", http.StatusBadRequest)
			return
		}
	}
	w.Header().Set("content-type", "application/octet-stream")
	w.Header().Set("content-length", strconv.FormatInt(size, 10))
	io.CopyN(w, rand.Reader, size)
}

// /GetUtcTime
//
// getUtcTimeHandler returns current time in UTC
func (j *Jellyfin) getUtcTimeHandler(w http.ResponseWriter, r *http.Request) {
	t := time.Now().UTC()
	response := JFGetUtcTimeResponse{
		RequestReceptionTime:     t,
		ResponseTransmissionTime: t,
	}
	serveJSON(response, w)
}

// /DisplayPreferences/usersettings?userId=2b1ec0a52b09456c9823a367d84ac9e5&client=emby'
//
// displayPreferencesHandler returns the display preferences for the user
func (j *Jellyfin) displayPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	serveJSON(DisplayPreferencesResponse{
		ID:                 "3ce5b65d-e116-d731-65d1-efc4a30ec35c",
		SortBy:             "SortName",
		RememberIndexing:   false,
		PrimaryImageHeight: 250,
		PrimaryImageWidth:  250,
		CustomPrefs: DisplayPreferencesCustomPrefs{
			ChromecastVersion:          "stable",
			SkipForwardLength:          "30000",
			SkipBackLength:             "10000",
			EnableNextVideoInfoOverlay: "False",
			Tvhome:                     "null",
			DashboardTheme:             "null",
		},
		ScrollDirection: "Horizontal",
		ShowBackdrop:    true,
		RememberSorting: false,
		SortOrder:       "Ascending",
		ShowSidebar:     false,
		Client:          "emby",
	}, w)
}

func localAddress(r *http.Request) string {
	protocol := "http"
	if r.TLS != nil {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s", protocol, r.Host)
}
