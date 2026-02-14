package jellyfin

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	serverVersion = "10.11.6"
)

// /health
//
// healthHandler returns health status
func (j *Jellyfin) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("cache-control", "no-cache, no-store")
	w.Write([]byte("Healthy"))
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

// /Plugins
//
// pluginsHandler returns emply plugin list, we do not support plugins at the moment
func (j *Jellyfin) pluginsHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFPluginResponse{}
	serveJSON(response, w)
}

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
	// Block desktop app and Jellyfin IOS as they depends on web assets that we do not have.
	// If we do not do this they hangs.. :(
	ua := r.Header.Get("user-agent")
	if strings.HasPrefix(ua, "Jellyfin/1") && strings.HasPrefix(ua, "JellyfinMediaPlayer") {
		w.WriteHeader(http.StatusTeapot)
		return
	}
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

// /System/Ping
//
// systemPingHandler returns static string
func (j *Jellyfin) systemPingHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("\"Jellyfin Server\""))
}

// /System/Logs
//
// systemLogsHandler returns empty log list, we do not support logs at the moment
func (j *Jellyfin) systemLogsHandler(w http.ResponseWriter, r *http.Request) {
	response := []string{}
	serveJSON(response, w)
}

// /System/Restart
// /System/Shutdown
//
// systemRestartHandler, nop
func (j *Jellyfin) systemRestartHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
}

// GET /ScheduledTasks
//
// scheduledTasksHandler returns empty scheduled task list, we do not support scheduled tasks at the moment
func (j *Jellyfin) scheduledTasksHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFScheduledTasksResponse{
		{
			Name:  "Scan collections",
			State: "Idle",
			ID:    "3a025083141d3c17dd96d5f9b951287b",
			LastExecutionResult: ScheduledTaskLastExecutionResult{
				StartTimeUtc: time.Now().UTC(),
				EndTimeUtc:   time.Now().UTC(),
				Status:       "Completed",
				Name:         "Scan collections",
				Key:          "ScanCollections",
				ID:           "3a025083141d3c17dd96d5f9b951287b",
			},
		},
	}
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

func localAddress(r *http.Request) string {
	protocol := "http"
	if r.TLS != nil {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s", protocol, r.Host)
}
