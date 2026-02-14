package jellyfin

import (
	"net/http"

	"github.com/gorilla/mux"
)

// /DisplayPreferences/usersettings?userId=2b1ec0a52b09456c9823a367d84ac9e5&client=emby'
//
// displayPreferencesHandler returns the display preferences for the user
func (j *Jellyfin) displayPreferencesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	response := DisplayPreferencesResponse{
		ID:                 id,
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
	}
	serveJSON(response, w)
}

// makeJFDisplayPreferencesID returns an external id for display preferences.
func makeJFDisplayPreferencesID(dpID string) string {
	return itemprefix_displaypreferences + dpID
}
