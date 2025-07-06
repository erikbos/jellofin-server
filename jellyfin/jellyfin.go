package jellyfin

import (
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
	Db           *database.DatabaseRepo
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
	db           *database.DatabaseRepo
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

// API definitions: https://swagger.emby.media/ & https://api.jellyfin.org/
// Docs: https://github.com/mediabrowser/emby/wiki

func New(o *Options) *Jellyfin {
	j := &Jellyfin{
		collections:        o.Collections,
		db:                 o.Db,
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
			// fallback to a fixed ID if hostname cannot be determined}
			j.serverID = "2b11644442754f02a0c1e45d2a9f5c71"
		}
	}
	if j.serverName == "" {
		j.serverName = "Jellofin"
	}
	return j
}

func (j *Jellyfin) RegisterHandlers(s *mux.Router) {
	r := s.UseEncodedPath()

	r.Use(lowercaseQueryParamNames)
	// middleware for endpoints to check valid auth token
	middleware := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(j.authmiddleware(http.HandlerFunc(handler)))
	}

	r.Handle("/System/Ping", http.HandlerFunc(j.systemPingHandler))
	r.Handle("/System/Info", middleware(j.systemInfoHandler))
	r.Handle("/System/Info/Public", http.HandlerFunc(j.systemInfoPublicHandler))
	r.Handle("/Plugins", http.HandlerFunc(j.pluginsHandler))

	r.Handle("/Users/AuthenticateByName", http.HandlerFunc(j.usersAuthenticateByNameHandler)).Methods("POST")
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
	r.Handle("/Users/{user}/Items/Latest", middleware(j.usersItemsLatestHandler))
	r.Handle("/Users/{user}/Items/Resume", middleware(j.usersItemsResumeHandler))
	r.Handle("/Users/{user}/Items/Suggestions", middleware(j.usersItemsSuggestionsHandler))
	r.Handle("/Users/{user}/Items/{item}", middleware(j.usersItemHandler))

	r.Handle("/UserViews", middleware(j.usersViewsHandler))
	r.Handle("/UserViews/GroupingOptions", middleware(j.usersGroupingOptionsHandler))

	r.Handle("/UserItems/Resume", middleware(j.usersItemsResumeHandler))
	r.Handle("/UserItems/{item}/Userdata", middleware(j.usersItemUserDataHandler))

	r.Handle("/DisplayPreferences/usersettings", middleware(j.displayPreferencesHandler))

	r.Handle("/Library/VirtualFolders", middleware(j.libraryVirtualFoldersHandler))

	r.Handle("/Shows/NextUp", middleware(j.showsNextUpHandler))
	r.Handle("/Shows/{show}/Seasons", middleware(j.showsSeasonsHandler))
	r.Handle("/Shows/{show}/Episodes", middleware(j.showsEpisodesHandler))

	r.Handle("/Items", middleware(j.usersItemsHandler))
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
	r.Handle("/Items/{item}/PlaybackInfo", middleware(j.itemsPlaybackInfoHandler))
	r.Handle("/Items/{item}/Similar", middleware(j.usersItemsSimilarHandler))

	r.Handle("/Genres", middleware(j.genresHandler))
	r.Handle("/Genres/{genre}", middleware(j.genreHandler))

	r.Handle("/Search/Hints", middleware(j.searchHintsHandler))

	r.Handle("/MediaSegments/{item}", middleware(j.mediaSegmentsHandler))
	r.Handle("/Videos/{item}/stream", middleware(j.videoStreamHandler))
	r.Handle("/Videos/{item}/stream.{container}", middleware(j.videoStreamHandler))
	// required for Vidhub to work
	r.Handle("/videos/{item}/stream", middleware(j.videoStreamHandler))
	r.Handle("/videos/{item}/stream.{container}", middleware(j.videoStreamHandler))

	r.Handle("/Persons", middleware(j.personsHandler))

	// userdata
	r.Handle("/Sessions/Playing", middleware(j.sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", middleware(j.sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", middleware(j.sessionsPlayingStoppedHandler)).Methods("POST")
	r.Handle("/UserPlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/UserPlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/UserFavoriteItems/{item}", middleware(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/UserFavoriteItems/{item}", middleware(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")
	// userdata legacy endpoints for Jellyfin <10.9
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Users/{user}/FavoriteItems/{item}", middleware(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/FavoriteItems/{item}", middleware(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")

	// sessions
	r.Handle("/Sessions", middleware(j.sessionsHandler))
	r.Handle("/Sessions/Capabilities", middleware(j.sessionsCapabilitiesHandler))
	r.Handle("/Sessions/Capabilities/Full", middleware(j.sessionsCapabilitiesFullHandler))

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
	r.Handle("/Branding/Configuration", middleware(j.brandingConfigurationHandler))
	r.Handle("/Branding/Css", middleware(j.brandingCssHandler))
	r.Handle("/Branding/Css.css", middleware(j.brandingCssHandler))

	// Localization
	r.Handle("/Localization/Countries", middleware(j.localizationCountriesHandler))
	r.Handle("/Localization/Cultures", middleware(j.localizationCulturesHandler))
	r.Handle("/Localization/Options", middleware(j.localizationOptionsHandler))
	r.Handle("/Localization/ParentalRatings", middleware(j.localizationParentalRatingsHandler))
}

// lowercaseQueryParamNames lower cases the firstcharacter of each query parametername
// this is to handle Infuse's incorrect naming of query parameters:
//
// ParentId -> parentId
// SeasonId -> seasonId
func lowercaseQueryParamNames(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Replace the query parameters with lowercased names
		newParams := url.Values{}
		for key, values := range r.URL.Query() {
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
