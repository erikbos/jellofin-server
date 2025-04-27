package jellyfin

import (
	"fmt"
	"net/http"
)

// /System/Info/Public
//
// systemInfoHandler returns basic server info
func (j *Jellyfin) systemInfoHandler(w http.ResponseWriter, r *http.Request) {
	protocol := "http"
	if r.TLS != nil {
		protocol = "https"
	}
	response := JFSystemInfoResponse{
		Id:           serverID,
		LocalAddress: fmt.Sprintf("%s://%s", protocol, r.Host),
		// Jellyfin native client checks for exact productname ..
		// https://github.com/jellyfin/jellyfin-expo/blob/7dedbc72fb53fc4b83c3967c9a8c6c071916425b/utils/ServerValidator.js#L82C49-L82C64
		ProductName:            "Jellyfin Server",
		ServerName:             j.serverName,
		Version:                "10.10.3",
		StartupWizardCompleted: true,
	}
	serveJSON(response, w)
}

// /Plugins
//
// pluginsHandler returns emply plugin list, we do not support plugins
func (j *Jellyfin) pluginsHandler(w http.ResponseWriter, r *http.Request) {
	// We do not list InfuseSync plugin as Infuse should be configured to use direct mode
	response := []JFPluginResponse{
		// {
		// 	Name:         "InfuseSync",
		// 	Version:      "1.5.0.0",
		// 	Description:  "Plugin for fast synchronization with Infuse.",
		// 	Id:           "022a3003993f45f1856587d12af2e12a",
		// 	CanUninstall: true,
		// 	HasImage:     true,
		// 	Status:       "Disabled",
		// },
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
