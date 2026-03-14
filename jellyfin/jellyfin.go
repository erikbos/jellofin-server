package jellyfin

import (
	"log"
	"net/http"
	"os"

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
	// Indicates if quickconnect is enabled
	QuickConnect bool
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
	// Indicates if quickconnect is enabled
	quickConnectEnabled bool
	// JPEG quality for posters
	imageQualityPoster int
}

func New(o *Options) *Jellyfin {
	j := &Jellyfin{
		collections:         o.Collections,
		repo:                o.Repo,
		serverID:            o.ServerID,
		serverName:          o.ServerName,
		imageresizer:        o.Imageresizer,
		autoRegister:        o.AutoRegister,
		quickConnectEnabled: o.QuickConnect,
		imageQualityPoster:  o.ImageQualityPoster,
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

	// middleware for endpoints to check valid auth token
	m := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(j.authmiddleware(http.HandlerFunc(handler)))
	}
	// mNoAuth for endpoints that don't require auth token, but should still be compressed
	// https://github.com/jellyfin/jellyfin/issues/5415#issuecomment-2825369811
	mNoAuth := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(http.HandlerFunc(handler))
	}

	r.Handle("/health", mNoAuth(j.healthHandler))
	r.Handle("/GetUtcTime", mNoAuth(j.getUtcTimeHandler))
	r.Handle("/System/Endpoint", m(j.systemEndpointHandler))
	r.Handle("/System/Ping", mNoAuth(j.systemPingHandler))
	r.Handle("/System/Info", m(j.systemInfoHandler))
	r.Handle("/System/Info/Public", mNoAuth(j.systemInfoPublicHandler))
	r.Handle("/System/Logs", m(j.systemLogsHandler))
	r.Handle("/System/Restart", m(j.systemRestartHandler)).Methods("POST")
	r.Handle("/System/Shutdown", m(j.systemRestartHandler)).Methods("POST")
	r.Handle("/Plugins", m(j.pluginsHandler))
	r.Handle("/ScheduledTasks", m(j.scheduledTasksHandler))
	r.Handle("/Playback/BitrateTest", m(j.playbackBitrateTestHandler))

	r.Handle("/Users/AuthenticateByName", mNoAuth(j.usersAuthenticateByNameHandler)).Methods("POST")
	r.Handle("/Users/AuthenticateWithQuickConnect", mNoAuth(j.usersAuthenticateWithQuickConnectHandler)).Methods("POST")
	r.Handle("/QuickConnect/Authorize", m(j.quickConnectAuthorizeHandler)).Methods("POST")
	r.Handle("/QuickConnect/Connect", mNoAuth(j.quickConnectConnectHandler)).Methods("GET")
	r.Handle("/QuickConnect/Enabled", mNoAuth(j.quickConnectEnabledHandler)).Methods("GET")
	r.Handle("/QuickConnect/Initiate", mNoAuth(j.quickConnectInitiateHandler)).Methods("POST")

	r.Handle("/Users", m(j.usersGetHandler)).Methods("GET")
	r.Handle("/Users", m(j.usersPostHandler)).Methods("POST")
	r.Handle("/Users/Me", m(j.usersMeHandler)).Methods("GET")
	r.Handle("/Users/New", m(j.usersNewItemsHandler)).Methods("POST")
	r.Handle("/Users/Password", m(j.usersPasswordHandler)).Methods("POST")
	r.Handle("/Users/Public", mNoAuth(j.usersPublicHandler)).Methods("GET")
	r.Handle("/Users/{userid}", m(j.userGetHandler)).Methods("GET")
	r.Handle("/Users/{userid}", m(j.userDeleteHandler)).Methods("DELETE")
	r.Handle("/Users/{userid}/Configuration", m(j.usersConfigurationHandler)).Methods("POST")
	r.Handle("/Users/{userid}/Policy", m(j.usersPolicyHandler)).Methods("POST")
	r.Handle("/Users/{userid}/Views", m(j.usersViewsHandler))
	r.Handle("/Users/{userid}/GroupingOptions", m(j.usersGroupingOptionsHandler))
	r.Handle("/Users/{userid}/Images/{type}", mNoAuth(j.usersImagesProfileHandler)).Methods("GET", "HEAD")

	r.Handle("/Users/{userid}/Items", m(j.usersItemsHandler))
	r.Handle("/Users/{userid}/Items/Intros", m(j.usersItemsIntrosHandler))
	r.Handle("/Users/{userid}/Items/Latest", m(j.usersItemsLatestHandler))
	r.Handle("/Users/{userid}/Items/Resume", m(j.usersItemsResumeHandler))
	r.Handle("/Users/{userid}/Items/Suggestions", m(j.usersItemsSuggestionsHandler))
	r.Handle("/Users/{userid}/Items/{itemid}", m(j.usersItemHandler))

	r.Handle("/UserViews", m(j.usersViewsHandler))
	r.Handle("/UserViews/GroupingOptions", m(j.usersGroupingOptionsHandler))

	r.Handle("/UserItems/Resume", m(j.usersItemsResumeHandler))
	r.Handle("/UserItems/{itemid}/Userdata", m(j.usersItemUserDataHandler))

	r.Handle("/DisplayPreferences/{id}", m(j.displayPreferencesHandler))

	r.Handle("/Library/MediaFolders", m(j.usersViewsHandler))
	r.Handle("/Library/VirtualFolders", m(j.libraryVirtualFoldersHandler))
	r.Handle("/Library/Refresh", m(j.libraryRefreshHandler)).Methods("POST")

	r.Handle("/Shows/NextUp", m(j.showsNextUpHandler))
	r.Handle("/Shows/{showid}/Seasons", m(j.showsSeasonsHandler))
	r.Handle("/Shows/{showid}/Episodes", m(j.showsEpisodesHandler))

	r.Handle("/Items", m(j.usersItemsHandler))
	r.Handle("/Items/Counts", m(j.usersItemsCountsHandler))
	r.Handle("/Items/Filters", m(j.usersItemsFiltersHandler))
	r.Handle("/Items/Filters2", m(j.usersItemsFilters2Handler))
	r.Handle("/Items/Latest", m(j.usersItemsLatestHandler))
	r.Handle("/Items/Root", m(j.usersItemsRootHandler))
	r.Handle("/Items/Suggestions", m(j.usersItemsSuggestionsHandler))
	r.Handle("/Items/{itemid}", m(j.usersItemHandler)).Methods("GET", "HEAD")
	r.Handle("/Items/{itemid}", m(j.itemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Items/{itemid}/Ancestors", m(j.usersItemsAncestorsHandler))
	// r.Handle("/Items/{itemid}/Download", middleware(j.usersItemsDownloadHandler))
	r.Handle("/Items/{itemid}/Images", mNoAuth(j.itemsImagesHandler))
	r.Handle("/Items/{itemid}/Images/{type}", mNoAuth(j.itemsImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Items/{itemid}/Images/{type}", mNoAuth(j.itemsImagesPostHandler)).Methods("POST")
	r.Handle("/Items/{itemid}/Images/{type}/{index}", mNoAuth(j.itemsImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Items/{itemid}/Images/{type}/{index}", mNoAuth(j.itemsImagesPostHandler)).Methods("POST")
	// r.Handle("/Items/{itemid}/Intros", m(j.usersItemsIntrosHandler))
	r.Handle("/Items/{itemid}/LocalTrailers", m(j.usersItemsLocalTrailersHandler))
	r.Handle("/Items/{itemid}/PlaybackInfo", m(j.itemsPlaybackInfoHandler))
	r.Handle("/Items/{itemid}/Refresh", m(j.usersItemsRefreshHandler)).Methods("POST")
	r.Handle("/Items/{itemid}/RemoteImages", mNoAuth(j.itemsRemoteImagesHandler))
	r.Handle("/Items/{itemid}/RemoteImages/Providers", mNoAuth(j.itemsRemoteImagesProvidersHandler))
	r.Handle("/Items/{itemid}/Similar", m(j.usersItemsSimilarHandler))
	r.Handle("/Items/{itemid}/SpecialFeatures", m(j.usersItemsSpecialFeaturesHandler))
	r.Handle("/Items/{itemid}/ThemeMedia", m(j.usersItemsThemeMediaHandler))

	r.Handle("/Years", m(j.yearsHandler))
	r.Handle("/Years/{year}", m(j.yearHandler))

	r.Handle("/UserImage", mNoAuth(j.userImageGetHandler)).Methods("GET", "HEAD")
	r.Handle("/UserImage", m(j.userImagePostHandler)).Methods("POST")
	r.Handle("/UserImage", m(j.userImageDeleteHandler)).Methods("DELETE")

	r.Handle("/Genres", m(j.genresHandler)).Methods("GET")
	r.Handle("/Genres/{name}", m(j.genreHandler)).Methods("GET")
	r.Handle("/Genres/{name}/Images/{type}", mNoAuth(j.GenresImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Genres/{name}/Images/{type}/{index}", mNoAuth(j.GenresImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Genres/{name}/Images/{type}", mNoAuth(j.GenresImagesPostHandler)).Methods("POST")

	r.Handle("/Studios", m(j.studiosHandler)).Methods("GET")
	r.Handle("/Studios/{name}", m(j.studioHandler)).Methods("GET")
	r.Handle("/Studios/{name}/Images/{type}", mNoAuth(j.StudiosImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Studios/{name}/Images/{type}/{index}", mNoAuth(j.StudiosImagesGetHandler)).Methods("GET", "HEAD")
	r.Handle("/Studios/{name}/Images/{type}", mNoAuth(j.StudiosImagesPostHandler)).Methods("POST")

	r.Handle("/Search/Hints", m(j.searchHintsHandler))
	r.Handle("/Movies/Recommendations", m(j.moviesRecommendationsHandler))

	r.Handle("/MediaSegments/{itemid}", mNoAuth(j.mediaSegmentsHandler))
	r.Handle("/Videos/{itemid}/{stream}", mNoAuth(j.videoStreamHandler))

	r.Handle("/Persons", m(j.personsHandler)).Methods("GET")
	r.Handle("/Persons/{name}", m(j.personHandler)).Methods("GET")

	r.Handle("/Devices", m(j.devicesGetHandler)).Methods("GET")
	r.Handle("/Devices", m(j.devicesDeleteHandler)).Methods("DELETE")
	r.Handle("/Devices/Info", m(j.devicesInfoHandler)).Methods("GET")
	r.Handle("/Devices/Options", m(j.devicesOptionsHandler)).Methods("GET")

	r.Handle("/Sessions", m(j.sessionsHandler)).Methods("GET")
	r.Handle("/Sessions/Capabilities", m(j.sessionsCapabilitiesHandler))
	r.Handle("/Sessions/Capabilities/Full", m(j.sessionsCapabilitiesFullHandler))
	r.Handle("/Sessions/Playing", m(j.sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", m(j.sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", m(j.sessionsPlayingStoppedHandler)).Methods("POST")

	r.Handle("/UserPlayedItems/{itemid}", m(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/UserPlayedItems/{itemid}", m(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/UserFavoriteItems/{itemid}", m(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/UserFavoriteItems/{itemid}", m(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Users/{user}/PlayedItems/{itemid}", m(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{itemid}", m(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Users/{user}/FavoriteItems/{itemid}", m(j.userFavoriteItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/FavoriteItems/{itemid}", m(j.userFavoriteItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/PlayingItems/{itemid}", m(j.playingItemsHandler)).Methods("POST")
	r.Handle("/PlayingItems/{itemid}", m(j.playingItemsDeleteHandler)).Methods("DELETE")
	r.Handle("/PlayingItems/{itemid}/Progress", m(j.playingItemsProgressHandler)).Methods("POST")

	r.Handle("/Playlists", m(j.createPlaylistHandler)).Methods("POST")
	r.Handle("/Playlists/{playlistid}", m(j.getPlaylistHandler)).Methods("GET")
	r.Handle("/Playlists/{playlistid}", m(j.updatePlaylistHandler)).Methods("POST")
	r.Handle("/Playlists/{playlistid}/Items", m(j.getPlaylistItemsHandler)).Methods("GET")
	r.Handle("/Playlists/{playlistid}/Items", m(j.addPlaylistItemsHandler)).Methods("POST")
	r.Handle("/Playlists/{playlistid}/Items", m(j.deletePlaylistItemsHandler)).Methods("DELETE")
	r.Handle("/Playlists/{playlistid}/Items/{itemid}/Move/{index}", m(j.movePlaylistItemHandler)).Methods("GET")
	r.Handle("/Playlists/{playlistid}/Users", m(j.getPlaylistAllUsersHandler)).Methods("GET")
	r.Handle("/Playlists/{playlistid}/Users/{userid}", m(j.getPlaylistUsersHandler)).Methods("GET")

	r.HandleFunc("/Branding/Configuration", j.brandingConfigurationHandler)
	r.HandleFunc("/Branding/Css", j.brandingCssHandler)
	r.HandleFunc("/Branding/Css.css", j.brandingCssHandler)

	r.HandleFunc("/Localization/Countries", j.localizationCountriesHandler)
	r.HandleFunc("/Localization/Cultures", j.localizationCulturesHandler)
	r.HandleFunc("/Localization/Options", j.localizationOptionsHandler)
	r.HandleFunc("/Localization/ParentalRatings", j.localizationParentalRatingsHandler)

	r.Handle("/SyncPlay/List", mNoAuth(j.syncPlayListHandler))
	r.Handle("/SyncPlay/New", mNoAuth(j.syncPlayNewHandler))
}
