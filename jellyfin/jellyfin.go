package jellyfin

import (
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/idhash"
	"github.com/erikbos/jellofin-server/imageresize"
)

// API definitions: https://swagger.emby.media/ & https://api.jellyfin.org/
// Docs: https://github.com/mediabrowser/emby/wiki

type Options struct {
	Collections  *collection.CollectionRepo
	Repo         database.Repository
	Imageresizer *imageresize.Resizer
	// Unique ID of this server, used in API responses
	ServerID string
	// ServerName is name of server returned in info responses
	ServerName string
	// ServerPort is the port of the server
	ServerPort string
	// Indicates if we should auto-register Jellyfin users
	AutoRegister bool
	// JPEG quality for posters
	ImageQualityPoster int
}

type Jellyfin struct {
	collections  *collection.CollectionRepo
	repo         database.Repository
	imageresizer *imageresize.Resizer
	// Unique ID of this server, used in API responses
	serverID string
	// serverName is name of server returned in info responses
	serverName string
	// Indicates if we should auto-register Jellyfin users
	autoRegister bool
	// JPEG quality for posters
	imageQualityPoster int
}

func New(o *Options) *Jellyfin {
	j := &Jellyfin{
		collections:        o.Collections,
		repo:               o.Repo,
		serverID:           o.ServerID,
		serverName:         o.ServerName,
		imageresizer:       o.Imageresizer,
		autoRegister:       o.AutoRegister,
		imageQualityPoster: o.ImageQualityPoster,
	}
	if j.serverID == "" {
		if hostname, err := os.Hostname(); err == nil {
			j.serverID = idhash.IdHash(hostname)
		} else {
			log.Printf("Failed to get hostname for server ID generation: %v", err)
		}
	}
	if j.serverName == "" {
		j.serverName = "Jellofin"
	}
	return j
}

func (j *Jellyfin) RegisterHandlers(s *mux.Router) {
	r := s.UseEncodedPath()

	r.Use(normalizeJellyfinRequest)

	// middleware for endpoints to check valid auth token
	middleware := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(j.authmiddleware(http.HandlerFunc(handler)))
	}

	r.Handle("/health", http.HandlerFunc(j.healthHandler))
	r.Handle("/System/Endpoint", middleware(j.systemEndpointHandler))
	r.Handle("/System/Ping", http.HandlerFunc(j.systemPingHandler))
	r.Handle("/System/Info", middleware(j.systemInfoHandler))
	r.Handle("/System/Info/Public", http.HandlerFunc(j.systemInfoPublicHandler))
	r.Handle("/Plugins", middleware(j.pluginsHandler))
	r.Handle("/GetUtcTime", http.HandlerFunc(j.getUtcTimeHandler))
	r.Handle("/Playback/BitrateTest", middleware(j.playbackBitrateTestHandler))

	r.Handle("/Users/AuthenticateByName", http.HandlerFunc(j.usersAuthenticateByNameHandler)).Methods("POST")
	r.Handle("/Users/authenticatebyname", http.HandlerFunc(j.usersAuthenticateByNameHandler)).Methods("POST")
	r.Handle("/QuickConnect/Authorize", middleware(j.quickConnectAuthorizeHandler)).Methods("POST")
	r.Handle("/QuickConnect/Connect", http.HandlerFunc(j.quickConnectConnectHandler))
	r.Handle("/QuickConnect/Enabled", http.HandlerFunc(j.quickConnectEnabledHandler))
	r.Handle("/QuickConnect/Initiate", http.HandlerFunc(j.quickConnectInitiateHandler)).Methods("POST")

	r.Handle("/Users", middleware(j.usersAllHandler))
	r.Handle("/Users/Me", middleware(j.usersMeHandler))
	r.Handle("/Users/Public", http.HandlerFunc(j.usersPublicHandler))
	r.Handle("/Users/{user}", middleware(j.usersHandler))

	// Legacy endpoints for Jellyfin <10.9
	r.Handle("/Users/{user}/Views", middleware(j.usersViewsHandler))
	r.Handle("/Users/{user}/GroupingOptions", middleware(j.usersGroupingOptionsHandler))
	r.Handle("/Users/{user}/Items", middleware(j.usersItemsHandler))
	r.Handle("/Users/{user}/Items/Intros", middleware(j.usersItemsIntrosHandler))
	r.Handle("/Users/{user}/Items/Latest", middleware(j.usersItemsLatestHandler))
	r.Handle("/Users/{user}/Items/Resume", middleware(j.usersItemsResumeHandler))
	r.Handle("/Users/{user}/Items/Suggestions", middleware(j.usersItemsSuggestionsHandler))
	r.Handle("/Users/{user}/Items/{item}", middleware(j.usersItemHandler))

	r.Handle("/UserViews", middleware(j.usersViewsHandler))
	r.Handle("/UserViews/GroupingOptions", middleware(j.usersGroupingOptionsHandler))

	r.Handle("/UserItems/Resume", middleware(j.usersItemsResumeHandler))
	r.Handle("/UserItems/{item}/Userdata", middleware(j.usersItemUserDataHandler))

	r.Handle("/DisplayPreferences/usersettings", middleware(j.displayPreferencesHandler))

	r.Handle("/Library/MediaFolders", middleware(j.usersViewsHandler))
	r.Handle("/Library/VirtualFolders", middleware(j.libraryVirtualFoldersHandler))

	r.Handle("/Shows/NextUp", middleware(j.showsNextUpHandler))
	r.Handle("/Shows/{show}/Seasons", middleware(j.showsSeasonsHandler))
	r.Handle("/Shows/{show}/Episodes", middleware(j.showsEpisodesHandler))

	r.Handle("/Items", middleware(j.usersItemsHandler))
	r.Handle("/Items/Counts", middleware(j.usersItemsCountsHandler))
	r.Handle("/Items/Filters", middleware(j.usersItemsFiltersHandler))
	r.Handle("/Items/Filters2", middleware(j.usersItemsFilters2Handler))
	r.Handle("/Items/Latest", middleware(j.usersItemsLatestHandler))
	r.Handle("/Items/Suggestions", middleware(j.usersItemsSuggestionsHandler))
	r.Handle("/Items/{item}", middleware(j.itemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Items/{item}", middleware(j.usersItemHandler))
	r.Handle("/Items/{item}/Ancestors", middleware(j.usersItemsAncestorsHandler))
	// Images can be fetched without auth, https://github.com/jellyfin/jellyfin/issues/13988
	// streamyfin does not yet set auth headers
	r.Handle("/Items/{item}/Images/{type}", http.HandlerFunc(j.itemsImagesHandler)).Methods("GET")
	r.Handle("/Items/{item}/Images/{type}/{index}", http.HandlerFunc(j.itemsImagesHandler)).Methods("GET")
	r.Handle("/Items/{item}/Intros", middleware(j.usersItemsIntrosHandler))
	r.Handle("/Items/{item}/PlaybackInfo", middleware(j.itemsPlaybackInfoHandler))
	r.Handle("/Items/{item}/Refresh", middleware(j.usersItemsRefreshHandler)).Methods("POST")
	r.Handle("/Items/{item}/Similar", middleware(j.usersItemsSimilarHandler))
	r.Handle("/Items/{item}/SpecialFeatures", middleware(j.usersItemsSpecialFeaturesHandler))
	r.Handle("/Items/{item}/ThemeMedia", middleware(j.usersItemsThemeMediaHandler))

	r.Handle("/Genres", middleware(j.genresHandler))
	r.Handle("/Genres/{name}", middleware(j.genreHandler))

	r.Handle("/Studios", middleware(j.studiosHandler))
	r.Handle("/Studios/{name}", middleware(j.studioHandler))

	r.Handle("/Search/Hints", middleware(j.searchHintsHandler))
	r.Handle("/Movies/Recommendations", middleware(j.moviesRecommendationsHandler))

	r.Handle("/MediaSegments/{item}", middleware(j.mediaSegmentsHandler))
	r.Handle("/Videos/{item}/stream", middleware(j.videoStreamHandler))
	r.Handle("/Videos/{item}/stream.{container}", middleware(j.videoStreamHandler))
	// required for Vidhub to work
	r.Handle("/videos/{item}/stream", middleware(j.videoStreamHandler))
	r.Handle("/videos/{item}/stream.{container}", middleware(j.videoStreamHandler))

	r.Handle("/Persons", middleware(j.personsHandler))
	r.Handle("/Persons/{name}", middleware(j.personHandler))

	r.Handle("/Devices/Info", middleware(j.devicesInfoHandler)).Methods("GET")
	r.Handle("/Devices/Options", middleware(j.devicesOptionsHandler)).Methods("GET")
	r.Handle("/Devices", middleware(j.devicesGetHandler)).Methods("GET")
	r.Handle("/Devices", middleware(j.devicesDeleteHandler)).Methods("DELETE")

	r.Handle("/Sessions/Capabilities", middleware(j.sessionsCapabilitiesHandler))
	r.Handle("/Sessions/Capabilities/Full", middleware(j.sessionsCapabilitiesFullHandler))
	r.Handle("/Sessions/Playing", middleware(j.sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", middleware(j.sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", middleware(j.sessionsPlayingStoppedHandler)).Methods("POST")
	r.Handle("/Sessions", middleware(j.sessionsHandler))
	r.Handle("/UserPlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/UserPlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/UserFavoriteItems/{item}", middleware(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/UserFavoriteItems/{item}", middleware(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")
	// userdata legacy endpoints for Jellyfin <10.9
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Users/{user}/FavoriteItems/{item}", middleware(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/FavoriteItems/{item}", middleware(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")

	// playlists
	r.Handle("/Playlists", middleware(j.createPlaylistHandler)).Methods("POST")
	r.Handle("/Playlists/{playlist}", middleware(j.getPlaylistHandler)).Methods("GET")
	r.Handle("/Playlists/{playlist}", middleware(j.updatePlaylistHandler)).Methods("POST")
	r.Handle("/Playlists/{playlist}/Items", middleware(j.getPlaylistItemsHandler)).Methods("GET")
	r.Handle("/Playlists/{playlist}/Items", middleware(j.addPlaylistItemsHandler)).Methods("POST")
	// Infuse posts to path ending with /
	r.Handle("/Playlists/{playlist}/Items/", middleware(j.addPlaylistItemsHandler)).Methods("POST")
	r.Handle("/Playlists/{playlist}/Items", middleware(j.deletePlaylistItemsHandler)).Methods("DELETE")
	r.Handle("/Playlists/{playlist}/Items/{item}/Move/{index}", middleware(j.movePlaylistItemHandler)).Methods("GET")
	r.Handle("/Playlists/{playlist}/Users", middleware(j.getPlaylistAllUsersHandler)).Methods("GET")
	r.Handle("/Playlists/{playlist}/Users/{user}", middleware(j.getPlaylistUsersHandler)).Methods("GET")

	// Branding
	r.HandleFunc("/Branding/Configuration", j.brandingConfigurationHandler)
	r.HandleFunc("/Branding/Css", j.brandingCssHandler)
	r.HandleFunc("/Branding/Css.css", j.brandingCssHandler)

	// Localization
	r.HandleFunc("/Localization/Countries", j.localizationCountriesHandler)
	r.HandleFunc("/Localization/Cultures", j.localizationCulturesHandler)
	r.HandleFunc("/Localization/Options", j.localizationOptionsHandler)
	r.HandleFunc("/Localization/ParentalRatings", j.localizationParentalRatingsHandler)

	// SyncPlay
	r.Handle("/SyncPlay/List", http.HandlerFunc(j.syncPlayListHandler))
	r.Handle("/SyncPlay/New", http.HandlerFunc(j.syncPlayNewHandler))
}

// normalizeJellyfinRequest is a middleware that normalizes requests:
// it normalizes query parameter names by converting the first character.
//
// Note: this middleware runs too late to be able to fix path issues:
// normalizing r.URL.Path is handled in server.go
func normalizeJellyfinRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Lowercase query parameter names. This is to handle incorrect naming of query parameters.
		// E.g. ParentId should have been parentId, SeasonId -> seasonId
		newParams := url.Values{}
		for key, values := range r.URL.Query() {
			// Skip adding "fields" as we return full api response on every reply,
			// and it tends to clutters log entries
			if key == "fields" {
				continue
			}
			for _, value := range values {
				newKey := strings.ToLower(string(key[0])) + key[1:]
				newParams.Add(newKey, value)
			}
		}
		r.URL.RawQuery = newParams.Encode()

		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
