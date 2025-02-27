package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

// API definitions: https://swagger.emby.media/ & https://api.jellyfin.org/
// Docs: https://github.com/mediabrowser/emby/wiki

func registerJellyfinHandlers(s *mux.Router) {
	r := s.UseEncodedPath()

	// Endpoints without auth
	r.Handle("/System/Info/Public", http.HandlerFunc(systemInfoHandler))
	r.Handle("/Users/AuthenticateByName", http.HandlerFunc(usersAuthenticateByNameHandler)).Methods("POST")

	middleware := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(authMiddleware(http.HandlerFunc(handler)))
	}

	// Endpoints with auth and gzip middleware
	r.Handle("/DisplayPreferences/usersettings", middleware(displayPreferencesHandler))
	r.Handle("/Users/{user}", middleware(usersHandler))
	r.Handle("/Users/{user}/Views", middleware(usersViewsHandler))
	r.Handle("/Users/{user}/GroupingOptions", middleware(usersGroupingOptionsHandler))
	r.Handle("/Users/{user}/Items", middleware(usersItemsHandler))
	r.Handle("/Users/{user}/Items/Latest", middleware(usersItemsLatestHandler))
	r.Handle("/Users/{user}/Items/{item}", middleware(usersItemHandler))
	r.Handle("/Users/{user}/Items/Resume", middleware(usersItemsResumeHandler))
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(usersPlayedItemsDeleteHandler)).Methods("DELETE")

	r.Handle("/Library/VirtualFolders", middleware(libraryVirtualFoldersHandler))
	r.Handle("/Shows/NextUp", middleware(showsNextUpHandler))
	r.Handle("/Shows/{show}/Seasons", middleware(showsSeasonsHandler))
	r.Handle("/Shows/{show}/Episodes", middleware(showsEpisodesHandler))

	r.Handle("/Items/{item}", middleware(itemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Items/{item}/Images/{type}", middleware(itemsImagesHandler))
	r.Handle("/Items/{item}/PlaybackInfo", middleware(itemsPlaybackInfoHandler))
	r.Handle("/MediaSegments/{item}", middleware(mediaSegmentsHandler))
	r.Handle("/Videos/{item}/stream", middleware(videoStreamHandler))

	r.Handle("/Persons", middleware(personsHandler))

	r.Handle("/Sessions/Playing", middleware(sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", middleware(sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", middleware(sessionsPlayingStoppedHandler)).Methods("POST")
}

type contextKey string

const (
	// Misc IDs for api responses
	serverID                  = "2b11644442754f02a0c1e45d2a9f5c71"
	collectionRootID          = "e9d5075a555c1cbc394eec4cef295274"
	displayPreferencesID      = "f137a2dd21bbc1b99aa5c0f6bf02a805"
	JellyfinCollectionMovies  = "movies"
	JellyfinCollectionTVShows = "tvshows"

	// itemid prefixes
	itemprefix_separator  = "_"
	itemprefix_collection = "collection_"
	itemprefix_show       = "show_"
	itemprefix_season     = "season_"
	itemprefix_episode    = "episode_"

	// imagetag prefix will get HTTP-redirected
	tagprefix_redirect = "redirect_"
	// imagetag prefix means we will serve the filename from local disk
	tagprefix_file = "file_"

	// Context key holding access token details within a request
	contextAccessTokenDetails contextKey = "AccessTokenDetails"
)

// curl -v http://127.0.0.1:9090/System/Info/Public
func systemInfoHandler(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()

	response := JFSystemInfoResponse{
		Id:           serverID,
		LocalAddress: "http://" + hostname + ":9090/",
		// Jellyfin native client checks for exact productname :facepalm:
		// https://github.com/jellyfin/jellyfin-expo/blob/7dedbc72fb53fc4b83c3967c9a8c6c071916425b/utils/ServerValidator.js#L82C49-L82C64
		ProductName:            "Jellyfin Server",
		ServerName:             "jellyfin",
		Version:                "10.10.3",
		StartupWizardCompleted: true,
	}
	serveJSON(response, w)
}

// curl -v -X POST http://127.0.0.1:9090/Users/AuthenticateByName
// Authenticates a user by name.
// (POST /Users/AuthenticateByName)
func usersAuthenticateByNameHandler(w http.ResponseWriter, r *http.Request) {
	var request JFAuthenticateUserByName
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusUnauthorized)
		return
	}

	if len(*request.Username) == 0 || len(*request.Pw) == 0 {
		http.Error(w, "username and password required", http.StatusUnauthorized)
		return
	}

	embyHeader, err := parseAuthHeader(r)
	if err != nil || embyHeader == nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	user, err := dbUserValidate(request.Username, request.Pw)
	if err != nil {
		if err == ErrUserNotFound && config.Jellyfin.AutoRegister {
			user, err = dbUserInsert(request.Username, request.Pw)
			if err != nil {
				http.Error(w, "Failed to auto-register user", http.StatusInternalServerError)
				return
			}
		} else {
			http.Error(w, "Invalid username/password", http.StatusUnauthorized)
			return
		}
	}

	remoteAddress, _, _ := net.SplitHostPort(r.RemoteAddr)

	session := &JFSessionInfo{
		Id:                 "e3a869b7a901f8894de8ee65688db6c0",
		UserId:             user.Id,
		UserName:           user.Username,
		Client:             embyHeader.client,
		DeviceName:         embyHeader.device,
		DeviceId:           embyHeader.deviceId,
		ApplicationVersion: embyHeader.version,
		RemoteEndPoint:     remoteAddress,
		LastActivityDate:   time.Now().UTC(),
		IsActive:           true,
	}

	accesstoken := AccessTokens.New(session)

	response := JFAuthenticateByNameResponse{
		AccessToken: accesstoken,
		SessionInfo: session,
		ServerId:    serverID,
		User: JFUser{
			ServerId:                  serverID,
			Id:                        user.Id,
			Name:                      user.Username,
			HasPassword:               true,
			HasConfiguredPassword:     true,
			HasConfiguredEasyPassword: false,
			EnableAutoLogin:           false,
			LastLoginDate:             time.Now().UTC(),
			LastActivityDate:          time.Now().UTC(),
		},
	}
	serveJSON(response, w)
}

// authMiddleware extracts and processes emby authorization header
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		embyHeader, err := parseAuthHeader(r)
		if err != nil || embyHeader == nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		// log.Printf("authMiddleware: Token=%s, Client=%s, Device=%s, DeviceId=%s, Version=%s",
		// 	embyHeader.token, embyHeader.client, embyHeader.device, embyHeader.deviceID, embyHeader.version)

		tokendetails := AccessTokens.Lookup(embyHeader.token)
		if tokendetails == nil {
			http.Error(w, "invalid access token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), contextAccessTokenDetails, tokendetails)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// getAccessTokenDetails returns access token details of a request in flight
func getAccessTokenDetails(r *http.Request) *AccessToken {
	// This should have been populated by authMiddleware()
	details, ok := r.Context().Value(contextAccessTokenDetails).(*AccessToken)
	if ok {
		return details
	} else {
		return nil
	}
}

// AuthHeaderValues holds parsed x-emby-authorization header
type AuthHeaderValues struct {
	device   string
	deviceId string
	token    string
	client   string
	version  string
}

// parseAuthHeader parses x-emby-authorization header
func parseAuthHeader(r *http.Request) (*AuthHeaderValues, error) {
	errEmbyAuthHeader := errors.New("invalid or no emby-authorization header provided")

	authHeader := r.Header.Get("x-emby-authorization")
	authHeader = strings.TrimPrefix(authHeader, "MediaBrowser ")
	if authHeader == "" {
		return nil, errEmbyAuthHeader
	}

	// MediaBrowser Client="Jellyfin%20Media%20Player", Device="mbp", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Version="1.11.1", Token="aea78abca5744378b2a2badf710e7307"
	// MediaBrowser Device="Mac", DeviceId="0dabe147-5d08-4e70-adde-d6b778b725aa", Token="826c2aa3596b47f2a386dd2811248649", Client="Infuse-Direct", Version="8.0.9"

	var result AuthHeaderValues
	for _, part := range strings.Split(authHeader, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			v := strings.Trim(kv[1], "\"")
			switch kv[0] {
			case "Device":
				result.device = v
			case "DeviceId":
				result.deviceId = v
			case "Token":
				result.token = v
			case "Client":
				result.client = v
			case "Version":
				result.version = v
			}
		} else {
			return nil, errEmbyAuthHeader
		}
	}
	return &result, nil
}

// curl -v 'http://127.0.0.1:9090/DisplayPreferences/usersettings?userId=2b1ec0a52b09456c9823a367d84ac9e5&client=emby'
func displayPreferencesHandler(w http.ResponseWriter, r *http.Request) {
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

// curl -v http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5
func usersHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken context not found", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	if vars["user"] != accessTokenDetails.session.UserId {
		http.Error(w, "invalid user id", http.StatusNotFound)
		return
	}

	user := JFUser{
		Id:                        accessTokenDetails.session.UserId,
		ServerId:                  serverID,
		Name:                      accessTokenDetails.session.UserName,
		HasPassword:               true,
		HasConfiguredPassword:     true,
		HasConfiguredEasyPassword: false,
		EnableAutoLogin:           false,
		LastLoginDate:             time.Now().UTC(),
		LastActivityDate:          time.Now().UTC(),
	}
	serveJSON(user, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Views?IncludeExternalContent=false'
func usersViewsHandler(w http.ResponseWriter, r *http.Request) {
	items := []JFItem{}
	for _, c := range config.Collections {
		if item, err := buildJFItemCollection(genCollectionID(c.SourceId)); err == nil {
			items = append(items, item)
		}
	}
	response := JFUserViewsResponse{
		Items:            items,
		TotalRecordCount: len(items),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/GroupingOptions
func usersGroupingOptionsHandler(w http.ResponseWriter, r *http.Request) {
	collections := []JFCollection{}
	for _, c := range config.Collections {
		collection := JFCollection{
			Name: c.Name_,
			ID:   genCollectionID(c.SourceId),
		}
		collections = append(collections, collection)
	}
	serveJSON(collections, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Resume?Limit=12&MediaTypes=Video&Recursive=true&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
func usersItemsResumeHandler(w http.ResponseWriter, r *http.Request) {
	response := JFUsersItemsResumeResponse{
		Items:            []string{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/f137a2dd21bbc1b99aa5c0f6bf02a805?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// handle individual item: any type: collection, a movie/show or individual file
func usersItemHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]

	splitted := strings.Split(itemId, "_")
	if len(splitted) == 2 {
		switch splitted[0] {
		case "collection":
			collectionItem, err := buildJFItemCollection(itemId)
			if err != nil {
				http.Error(w, "Could not find collection", http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case "season":
			seasonItem, err := buildJFItemSeason(accessTokenDetails.session.UserId, itemId)
			if err != nil {
				http.Error(w, "Could not find season", http.StatusNotFound)
				return
			}
			serveJSON(seasonItem, w)
			return
		case "episode":
			episodeItem, err := buildJFItemEpisode(accessTokenDetails.session.UserId, itemId)
			if err != nil {
				http.Error(w, "Could not find episode", http.StatusNotFound)
				return
			}
			serveJSON(episodeItem, w)
			return
		default:
			log.Print("Item request for unknown prefix!")
			http.Error(w, "Unknown item prefix", http.StatusInternalServerError)
			return
		}
	}

	// Try to find individual item
	c, i := getItemByID(itemId)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	serveJSON(buildJFItem(accessTokenDetails.session.UserId, i, idHash(c.Name_), c.Type, false), w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items?ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&ParentId=f137a2dd21bbc1b99aa5c0f6bf02a805&SortBy=SortName,ProductionYear&SortOrder=Ascending&IncludeItemTypes=Movie&Recursive=true&StartIndex=0&Limit=50'
// find based upon title
// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items?ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&SearchTerm=p&Recursive=true&Limit=24

// generate list of items based upon provided ParentId or a text searchTerm
// query params:
// - ParentId, if provided scope result set to this collection
// - SearchTerm, substring to match on
// - StartIndex, index of first result item
// - Limit=50, number of items to return
func usersItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	queryparams := r.URL.Query()
	searchTerm := queryparams.Get("SearchTerm")

	searchCollection := queryparams.Get("ParentId")
	var searchC *Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = getCollection(collectionid)
	}

	items := []JFItem{}
	for _, c := range config.Collections {
		// Skip if we are searching in one particular collection
		if searchC != nil && searchC.SourceId != c.SourceId {
			continue
		}

		for _, i := range c.Items {
			if searchTerm == "" || strings.Contains(strings.ToLower(i.Name), strings.ToLower(searchTerm)) {
				// fixup sortname if need so we can sort later
				if i.SortName == "" {
					i.SortName = i.Name
				}
				items = append(items, buildJFItem(accessTokenDetails.session.UserId, i, idHash(c.Name_), c.Type, true))
			}
		}
	}

	// Apply sorting if SortBy is provided
	sortBy := queryparams.Get("SortBy")
	if sortBy != "" {
		sortFields := strings.Split(sortBy, ",")

		sortOrder := queryparams.Get("SortOrder")
		var sortDescending bool
		if sortOrder == "Descending" {
			sortDescending = true
		}

		sort.SliceStable(items, func(i, j int) bool {
			for _, field := range sortFields {
				switch strings.ToLower(field) {
				case "sortname":
					if items[i].SortName != items[j].SortName {
						if sortDescending {
							return items[i].SortName > items[j].SortName
						}
						return items[i].SortName < items[j].SortName
					}
				case "productionyear":
					if items[i].ProductionYear != items[j].ProductionYear {
						if sortDescending {
							return items[i].ProductionYear > items[j].ProductionYear
						}
						return items[i].ProductionYear < items[j].ProductionYear
					}
				case "criticrating":
					if items[i].CriticRating != items[j].CriticRating {
						if sortDescending {
							return items[i].CriticRating > items[j].CriticRating
						}
						return items[i].CriticRating < items[j].CriticRating
					}
				default:
					log.Printf("usersItemsHandler: unknown sortorder %s\n", sortBy)
				}
			}
			return false
		})
	}

	totalItemCount := len(items)

	// Apply pagination
	startIndex, startIndexErr := strconv.Atoi(queryparams.Get("StartIndex"))
	if startIndexErr == nil && startIndex >= 0 && startIndex < len(items) {
		items = items[startIndex:]
	}
	limit, limitErr := strconv.Atoi(queryparams.Get("Limit"))
	if limitErr == nil && limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	response := UserItemsResponse{
		Items:            items,
		TotalRecordCount: totalItemCount,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Latest?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&ParentId=f137a2dd21bbc1b99aa5c0f6bf02a805&StartIndex=0&Limit=20'

// generate list of new items based upon provided ParentId
// query params:
// - ParentId, if provided scope result set to this collection
// - StartIndex, index of first result item
// - Limit=50, number of items to return
func usersItemsLatestHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	queryparams := r.URL.Query()
	searchCollection := queryparams.Get("ParentId")
	var searchC *Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = getCollection(collectionid)
	}

	items := []JFItem{}
	for _, c := range config.Collections {
		// Skip if we are searching in one particular collection
		if searchC != nil && searchC.SourceId != c.SourceId {
			continue
		}
		for _, i := range c.Items {
			items = append(items, buildJFItem(accessTokenDetails.session.UserId, i, idHash(c.Name_), c.Type, true))
		}
	}

	// Sort by premieredate to list most recent releases first
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PremiereDate.After(items[j].PremiereDate)
	})

	// Apply pagination
	startIndex, startIndexErr := strconv.Atoi(queryparams.Get("StartIndex"))
	if startIndexErr == nil && startIndex >= 0 && startIndex < len(items) {
		items = items[startIndex:]
	}
	limit, limitErr := strconv.Atoi(queryparams.Get("Limit"))
	if limitErr == nil && limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	serveJSON(items, w)
}

// curl -v http://127.0.0.1:9090/Library/VirtualFolders
func libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	libraries := []JFMediaLibrary{}
	for _, c := range config.Collections {
		itemId := genCollectionID(c.SourceId)
		l := JFMediaLibrary{
			Name:               c.Name_,
			ItemId:             itemId,
			PrimaryImageItemId: itemId,
			Locations:          []string{"/"},
		}
		switch c.Type {
		case collectionMovies:
			l.CollectionType = JellyfinCollectionMovies
		case collectionShows:
			l.CollectionType = JellyfinCollectionTVShows
		}
		libraries = append(libraries, l)
	}
	serveJSON(libraries, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/4QBdg3S803G190AgFrBf/Seasons?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// generate season overview
func showsSeasonsHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	showId := vars["show"]
	_, i := getItemByID(showId)
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}
	// Create API response
	seasons := []JFItem{}
	for _, s := range i.Seasons {
		season, err := buildJFItemSeason(accessTokenDetails.session.UserId, s.Id)
		if err != nil {
			log.Printf("buildJFItemSeason returned error %s", err)
			continue
		}
		seasons = append(seasons, season)
	}
	response := UserItemsResponse{
		Items:            seasons,
		TotalRecordCount: len(seasons),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/rXlq4EHNxq4HIVQzw3o2/Episodes?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&SeasonId=rXlq4EHNxq4HIVQzw3o2/1'
// generate episode overview for one season of a show
func showsEpisodesHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	_, i := getItemByID(vars["show"])
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}

	// Do we need to filter down overview by a particular season?
	RequestedSeasonId := r.URL.Query().Get("SeasonId")

	// Create API response for requested season
	episodes := []JFItem{}
	for _, s := range i.Seasons {
		// Limit results to a season if id provided
		if RequestedSeasonId != "" && itemprefix_season+s.Id != RequestedSeasonId {
			continue
		}
		for _, e := range s.Episodes {
			episodeId := itemprefix_episode + e.Id
			episode, err := buildJFItemEpisode(accessTokenDetails.session.UserId, episodeId)
			if err != nil {
				log.Printf("buildJFItemEpisode returned error %s", err)
				continue
			}
			episodes = append(episodes, episode)
		}
	}
	response := UserItemsResponse{
		Items:            episodes,
		TotalRecordCount: len(episodes),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

func itemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusForbidden)
}

// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Backdrop?tag=7cec54f0c8f362c75588e83d76fefa75'
// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Logo?tag=e28fbe648d2dbb76b65c14f14e6b1d72'
// curl -v 'http://127.0.0.1:9090/Items/q2e2UzCOd9zkmJenIOph/Images/Primary?tag=70931a7d8c147c9e2c0aafbad99e03e5'
// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Primary?tag=268b80952354f01d5a184ed64b36dd52'
// curl -v 'http://127.0.0.1:9090/Items/2vx0ZYKeHxbh5iWhloIB/Images/Primary?tag=redirect_https://image.tmdb.org/t/p/original/3E4x5doNuuu6i9Mef6HPrlZjNb1.jpg'

func itemsImagesHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	// handle tag-based redirects for item imagery that is external (e.g. external images of actors)
	// for these we do not care about the provided item id
	queryparams := r.URL.Query()
	tag := queryparams.Get("tag")
	if strings.HasPrefix(tag, tagprefix_redirect) {
		w.Header().Set("cache-control", "max-age=2592000")
		http.Redirect(w, r, strings.TrimPrefix(tag, tagprefix_redirect), http.StatusFound)
		return
	}
	if strings.HasPrefix(tag, tagprefix_file) {
		w.Header().Set("cache-control", "max-age=2592000")
		serveFile(w, r, strings.TrimPrefix(tag, tagprefix_file))
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]
	imageType := vars["type"]

	splitted := strings.Split(itemId, "_")
	if len(splitted) == 2 {
		switch splitted[0] {
		// case "collection":
		// 	collectionItem, err := buildJFItemCollection(itemId)
		// 	if err != nil {
		// 		http.Error(w, "Could not find collection", http.StatusNotFound)
		// 		return

		// 	}
		// 	serveJSON(collectionItem, w)
		// 	return
		case "season":
			c, item, season := getSeasonByID(itemId)
			if season == nil {
				http.Error(w, "Could not find season", http.StatusNotFound)
				return
			}
			switch imageType {
			case "Primary":
				w.Header().Set("cache-control", "max-age=2592000")
				serveImage(w, r, c.Directory+"/"+item.Name+"/"+season.Poster,
					config.Jellyfin.ImageQualityPoster)
				return
			default:
				log.Printf("Image request %s, unknown type %s", itemId, imageType)
				return
			}
		case "episode":
			c, item, _, episode := getEpisodeByID(itemId)
			if episode == nil {
				http.Error(w, "Item not found (could not find episode)", http.StatusNotFound)
				return
			}
			serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Thumb)
			return
		default:
			log.Printf("Image request for unknown prefix %s!", itemId)
			http.Error(w, "Unknown image item prefix", http.StatusInternalServerError)
			return
		}
	}

	c, i := getItemByID(itemId)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch vars["type"] {
	case "Primary":
		w.Header().Set("cache-control", "max-age=2592000")
		serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Poster, config.Jellyfin.ImageQualityPoster)
		return
	case "Backdrop":
		w.Header().Set("cache-control", "max-age=2592000")
		serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Fanart)
		return
		// We do not have artwork on disk for logo requests
		// case "Logo":
		// return
	}
	log.Printf("Unknown image type requested: %s\n", vars["type"])
	http.Error(w, "Item image not found", http.StatusNotFound)
}

// curl -v 'http://127.0.0.1:9090/Items/68d73f6f48efedb7db697bf9fee580cb/PlaybackInfo?UserId=2b1ec0a52b09456c9823a367d84ac9e5'
func itemsPlaybackInfoHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// itemId := vars["item"]

	// c, i := getItemByID(itemId)
	// if i == nil || i.Video == "" {
	// 	http.Error(w, "Item not found", http.StatusNotFound)
	// 	return
	// }
	// item := buildJFItem(c, i, true)

	response := JFUsersPlaybackInfoResponse{
		MediaSources: buildMediaSource("test.mp4", nil),
		// TODO this static id should be generated based upon authenticated user
		// this id is used when submitting playstate via /Sessions/Playing endpoints
		PlaySessionID: "fc3b27127bf84ed89a300c6285d697e2",
	}
	serveJSON(response, w)
}

// return information about intro, commercial, preview, recap, outro segments
// of an item, not supported.
func mediaSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v -I 'http://127.0.0.1:9090/Videos/NrXTYiS6xAxFj4QAiJoT/stream'
func videoStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemId := vars["item"]

	// Is episode?
	if strings.HasPrefix(itemId, itemprefix_episode) {
		c, item, _, episode := getEpisodeByID(itemId)
		if episode == nil {
			http.Error(w, "Could not find episode", http.StatusNotFound)
			return
		}
		serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Video)
		return
	}

	c, i := getItemByID(vars["item"])
	if i == nil || i.Video == "" {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Video)
}

// return list of actors (hit by Infuse's search)
// not supported
func personsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/NextUp?UserId=2b1ec0a52b09456c9823a367d84ac9e5&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&StartIndex=0&Limit=20'
func showsNextUpHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	c, i := getItemByID("rVFG3EzPthk2wowNkqUl")
	response := JFShowsNextUpResponse{
		Items: []JFItem{
			buildJFItem(accessTokenDetails.session.UserId, i, idHash(c.Name_), c.Type, true),
		},
		TotalRecordCount: 1,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

func usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]

	playStateUpdate(accessTokenDetails.session.UserId, itemId, 0, true)
	w.WriteHeader(http.StatusOK)
}

func usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]

	playStateUpdate(accessTokenDetails.session.UserId, itemId, 0, false)
	w.WriteHeader(http.StatusOK)
}

// PositionTicks are in micro seconds
const TicsToSeconds = 10000000

func sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingHandler UserId: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks/TicsToSeconds)
	playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken context not found", http.StatusUnauthorized)
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingProgressHandler UserId: %s, ItemId: %s, Progress: %d seconds\n\n",
	// 	accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks/TicsToSeconds)
	playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken context not found", http.StatusUnauthorized)
		return
	}

	var request JFPlayState
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	// log.Printf("\nsessionsPlayingStoppedHandler UserId: %s, ItemId: %s, Progress: %d seconds, canSeek: %t\n\n",
	// 	accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks/TicsToSeconds, request.CanSeek)
	playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func playStateUpdate(userId, itemId string, positionTicks int, markAsWatched bool) (err error) {
	log.Printf("playStateUpdate UserId: %s, ItemId: %s, Progress: %d sec\n",
		userId, itemId, positionTicks/TicsToSeconds)

	var duration int
	if strings.HasPrefix(itemId, itemprefix_episode) {
		_, _, _, episode := getEpisodeByID(itemId)

		// fix me: we should not have to load NFO here
		lazyLoadNFO(&episode.Nfo, episode.NfoPath)

		duration = episode.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
	} else {
		_, item := getItemByID(itemId)
		duration = item.Nfo.Runtime * 60
	}

	playstate := PlayStateEntry{
		timestamp: time.Now().UTC(),
	}

	position := positionTicks / TicsToSeconds
	playedPercentage := 100 * position / duration

	// Mark as watched in case > 98% of the item is played
	if markAsWatched || playedPercentage >= 98 {
		playstate.position = 0
		playstate.playedPercentage = 0
		playstate.played = true
	} else {
		playstate.position = position
		playstate.playedPercentage = playedPercentage
		playstate.played = false
	}

	PlayState.Update(userId, itemId, playstate)
	return nil
}

func serveFile(w http.ResponseWriter, r *http.Request, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		http.Error(w, "Could not retrieve file info", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
}

func serveImage(w http.ResponseWriter, r *http.Request, filename string, imageQuality int) {
	file, err := resizer.OpenFile(w, r, filename, imageQuality)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		http.Error(w, "Could not retrieve file info", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
}

func buildJFItemCollection(itemid string) (response JFItem, e error) {
	collectionid := strings.TrimPrefix(itemid, itemprefix_collection)
	c := getCollection(collectionid)
	if c == nil {
		e = errors.New("collection not found")
		return
	}

	itemID := genCollectionID(c.SourceId)
	response = JFItem{
		Name:                     c.Name_,
		ServerID:                 serverID,
		ID:                       itemID,
		Etag:                     idHash(itemID),
		DateCreated:              time.Now().UTC(),
		Type:                     "CollectionFolder",
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               len(c.Items),
		DisplayPreferencesID:     displayPreferencesID,
		ExternalUrls:             []JFExternalUrls{},
		PlayAccess:               "Full",
		PrimaryImageAspectRatio:  1.7777777777777777,
		RemoteTrailers:           []JFRemoteTrailers{},
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		ParentID:                 "e9d5075a555c1cbc394eec4cef295274",
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		// PremiereDate should be set based upon most recent item in collection
		PremiereDate: time.Now().UTC(),
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	switch c.Type {
	case collectionMovies:
		response.CollectionType = JellyfinCollectionMovies
	case collectionShows:
		response.CollectionType = JellyfinCollectionTVShows
	}
	response.SortName = response.CollectionType
	return
}

// buildJFItem builds movie or show from provided item
func buildJFItem(userId string, i *Item, parentId, collectionType string, listView bool) (response JFItem) {
	response = JFItem{
		ID:                      i.Id,
		ParentID:                parentId,
		ServerID:                serverID,
		Name:                    i.Name,
		OriginalTitle:           i.Name,
		SortName:                i.Name,
		ForcedSortName:          i.Name,
		Etag:                    idHash(i.Id),
		DateCreated:             time.Unix(i.FirstVideo/1000, 0).UTC(),
		PremiereDate:            time.Unix(i.FirstVideo/1000, 0).UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
	}

	response.ImageTags = &JFImageTags{
		Primary: "primary_" + i.Id,
	}

	// Required to have Infuse load backdrop of episode
	response.BackdropImageTags = []string{
		response.ID,
	}

	if collectionType == collectionMovies {
		response.Type = "Movie"
		response.IsFolder = false
		response.LocationType = "FileSystem"
		response.Path = "file.mp4"
		response.MediaType = "Video"
		response.VideoType = "VideoFile"
		response.Container = "mov,mp4,m4a"

		lazyLoadNFO(&i.Nfo, i.NfoPath)
		response.MediaSources = buildMediaSource(i.Video, i.Nfo)

		// listview = true, movie carousel return both primary and BackdropImageTags
		// non-listview = false, remove primary (thumbnail) image reference
		if !listView {
			response.ImageTags = nil
		}
	}

	if collectionType == collectionShows {
		response.Type = "Series"
		response.IsFolder = true
		response.ChildCount = len(i.Seasons)

		var playedEpisodes, totalEpisodes int
		var lastestPlayed time.Time
		for _, s := range i.Seasons {
			for _, e := range s.Episodes {
				totalEpisodes++
				episodePlaystate, err := PlayState.Get(userId, e.Id)
				if err == nil {
					if episodePlaystate.played {
						playedEpisodes++
						if episodePlaystate.timestamp.After(lastestPlayed) {
							lastestPlayed = episodePlaystate.timestamp
						}
					}
				}
			}
		}
		if totalEpisodes != 0 {
			response.UserData = &JFUserData{
				UnplayedItemCount: totalEpisodes - playedEpisodes,
				PlayedPercentage:  100 * playedEpisodes / totalEpisodes,
				LastPlayedDate:    lastestPlayed,
				Key:               response.ID,
			}
			if playedEpisodes == response.ChildCount {
				response.UserData.Played = true
			}
		}
	}

	enrichResponseWithNFO(&response, i.Nfo)

	if playstate, err := PlayState.Get(userId, i.Id); err == nil {
		response.UserData = buildJFUserData(playstate)
		response.UserData.Key = i.Id
	}
	return response
}

// buildJFItemSeason builds season
func buildJFItemSeason(userId, seasonId string) (response JFItem, err error) {
	_, show, season := getSeasonByID(seasonId)
	if season == nil {
		err = errors.New("could not find season")
		return
	}

	response = JFItem{
		Type:               "Season",
		ServerID:           serverID,
		ParentID:           show.Id,
		SeriesID:           show.Id,
		ID:                 itemprefix_season + seasonId,
		Etag:               idHash(seasonId),
		SeriesName:         show.Name,
		IndexNumber:        season.SeasonNo,
		Name:               fmt.Sprintf("Season %d", season.SeasonNo),
		SortName:           fmt.Sprintf("%04d", season.SeasonNo),
		IsFolder:           true,
		LocationType:       "FileSystem",
		MediaType:          "Unknown",
		ChildCount:         len(season.Episodes),
		RecursiveItemCount: len(season.Episodes),
		DateCreated:        time.Now().UTC(),
		PremiereDate:       time.Now().UTC(),
		CanDelete:          false,
		CanDownload:        true,
		ImageTags: &JFImageTags{
			Primary: "season",
		},
	}

	var playedEpisodes int
	var lastestPlayed time.Time
	for _, e := range season.Episodes {
		episodePlaystate, err := PlayState.Get(userId, e.Id)
		if err == nil {
			if episodePlaystate.played {
				playedEpisodes++
				if episodePlaystate.timestamp.After(lastestPlayed) {
					lastestPlayed = episodePlaystate.timestamp
				}
			}
		}
	}
	response.UserData = &JFUserData{
		UnplayedItemCount: response.ChildCount - playedEpisodes,
		PlayedPercentage:  100 * playedEpisodes / response.ChildCount,
		LastPlayedDate:    lastestPlayed,
		Key:               response.ID,
	}
	if playedEpisodes == response.ChildCount {
		response.UserData.Played = true
	}

	return response, nil
}

// buildJFItemEpisode builds episode
func buildJFItemEpisode(userId, episodeId string) (response JFItem, err error) {
	_, show, _, episode := getEpisodeByID(episodeId)
	if episode == nil {
		err = errors.New("could not find episode")
		return
	}

	response = JFItem{
		Type:         "Episode",
		ID:           episodeId,
		Etag:         idHash(episodeId),
		ServerID:     serverID,
		SeriesName:   show.Name,
		SeriesID:     idHash(show.Name),
		LocationType: "FileSystem",
		Path:         "episode.mp4",
		IsFolder:     false,
		MediaType:    "Video",
		VideoType:    "VideoFile",
		Container:    "mov,mp4,m4a",
		HasSubtitles: true,
		DateCreated:  time.Unix(episode.VideoTS/1000, 0).UTC(),
		PremiereDate: time.Unix(episode.VideoTS/1000, 0).UTC(),
		CanDelete:    false,
		CanDownload:  true,
		ImageTags: &JFImageTags{
			Primary: "episode",
		},
	}

	// Get a bunch of metadata from show-level nfo
	lazyLoadNFO(&show.Nfo, show.NfoPath)
	if show.Nfo != nil {
		enrichResponseWithNFO(&response, show.Nfo)
	}

	// Remove ratings as we do not want ratings from series apply to an episode
	response.OfficialRating = ""
	response.CommunityRating = 0

	// Enrich and override metadata using episode nfo, if available, as it is more specific than data from show
	lazyLoadNFO(&episode.Nfo, episode.NfoPath)
	if episode.Nfo != nil {
		enrichResponseWithNFO(&response, episode.Nfo)
	}

	// Add some generic mediasource to indicate "720p, stereo"
	response.MediaSources = buildMediaSource(episode.Video, episode.Nfo)

	if playstate, err := PlayState.Get(userId, episodeId); err == nil {
		response.UserData = buildJFUserData(playstate)
		response.UserData.Key = episodeId
	}
	return response, nil
}

func buildJFUserData(p PlayStateEntry) (response *JFUserData) {
	response = &JFUserData{
		PlaybackPositionTicks: p.position * TicsToSeconds,
		PlayedPercentage:      p.playedPercentage,
		Played:                p.played,
		LastPlayedDate:        p.timestamp,
	}
	return
}

func lazyLoadNFO(n **Nfo, filename string) {
	// NFO already loaded and parsed?
	if *n != nil {
		return
	}
	if file, err := os.Open(filename); err == nil {
		defer file.Close()
		*n = decodeNfo(file)
	}
}

func enrichResponseWithNFO(response *JFItem, n *Nfo) {
	if n == nil {
		return
	}

	response.Name = n.Title
	response.Overview = n.Plot
	if n.Tagline != "" {
		response.Taglines = []string{n.Tagline}
	}

	// Handle episode naming & numbering
	if n.Season != "" {
		response.SeasonName = "Season " + n.Season
		response.ParentIndexNumber, _ = strconv.Atoi(n.Season)
	}
	if n.Episode != "" {
		response.IndexNumber, _ = strconv.Atoi(n.Episode)
	}
	if response.ParentIndexNumber != 0 && response.IndexNumber != 0 {
		response.SortName = fmt.Sprintf("%03s - %04s - %s", n.Season, n.Episode, n.Title)
	}

	// TV-14
	response.OfficialRating = n.Mpaa

	if n.Rating != 0 {
		response.CommunityRating = math.Round(float64(n.Rating)*10) / 10
	}

	if len(n.Genre) != 0 {
		normalizedGenres := normalizeGenres(n.Genre)
		// Why do we populate two response fields with same data?
		response.Genres = normalizedGenres
		for _, genre := range normalizedGenres {
			g := JFGenreItems{
				Name: genre,
				ID:   idHash(genre),
			}
			response.GenreItems = append(response.GenreItems, g)
		}
	}

	if n.Studio != "" {
		response.Studios = []JFStudios{
			{
				Name: n.Studio,
				ID:   idHash(n.Studio),
			},
		}
	}

	if len(n.UniqueIDs) != 0 {
		ids := JFProviderIds{}
		for _, id := range n.UniqueIDs {
			switch id.Type {
			case "imdb":
				ids.Imdb = id.Value
			case "themoviedb":
				ids.Tmdb = id.Value
			}
		}
		response.ProviderIds = ids
	}

	// if n.Actor != nil {
	// 	for _, actor := range n.Actor {
	// 		p := JFPeople{
	// 			Type: "Actor",
	// 			Name: actor.Name,
	// 			ID:   idHash(actor.Name),
	// 		}
	// 		if actor.Thumb != "" {
	// 			p.PrimaryImageTag = tagprefix_redirect + actor.Thumb
	// 		}
	// 		response.People = append(response.People, p)
	// 	}
	// }

	if n.Year != 0 {
		response.ProductionYear = n.Year
	}

	if n.Premiered != "" {
		if parsedTime, err := parseTime(n.Premiered); err == nil {
			response.PremiereDate = parsedTime
		}
	}
	if n.Aired != "" {
		if parsedTime, err := parseTime(n.Aired); err == nil {
			response.PremiereDate = parsedTime
		}
	}
}

func buildMediaSource(filename string, n *Nfo) (mediasources []JFMediaSources) {
	mediasource := JFMediaSources{
		ID:                    idHash(filename),
		ETag:                  idHash(filename),
		Name:                  filename,
		Path:                  filename,
		Type:                  "Default",
		Container:             "mp4",
		Protocol:              "File",
		VideoType:             "VideoFile",
		Size:                  4264940672,
		IsRemote:              false,
		ReadAtNativeFramerate: false,
		IgnoreDts:             false,
		IgnoreIndex:           false,
		GenPtsInput:           false,
		SupportsTranscoding:   true,
		SupportsDirectStream:  true,
		SupportsDirectPlay:    true,
		IsInfiniteStream:      false,
		RequiresOpening:       false,
		RequiresClosing:       false,
		RequiresLooping:       false,
		SupportsProbing:       true,
		Formats:               []string{},
	}

	// log.Printf("buildMediaSource: n: %+v, n2: %+v, n3: %+v\n", n, n.FileInfo, n.FileInfo.StreamDetails)
	if n == nil || n.FileInfo == nil || n.FileInfo.StreamDetails == nil {
		return []JFMediaSources{mediasource}
	}

	NfoVideo := n.FileInfo.StreamDetails.Video
	mediasource.Bitrate = NfoVideo.Bitrate
	mediasource.RunTimeTicks = int64(NfoVideo.DurationInSeconds) * 10000000

	// Take first alpha-3 language code, ignore others
	var language string
	if n.FileInfo.StreamDetails.Audio != nil && n.FileInfo.StreamDetails.Audio.Language != "" {
		language = n.FileInfo.StreamDetails.Audio.Language[0:3]
	} else {
		language = "eng"
	}

	// Create video stream with high-level details based upon NFO
	videostream := JFMediaStreams{
		Index:            0,
		Type:             "Video",
		IsDefault:        true,
		Language:         language,
		AverageFrameRate: math.Round(float64(NfoVideo.FrameRate*100)) / 100,
		RealFrameRate:    math.Round(float64(NfoVideo.FrameRate*100)) / 100,
		TimeBase:         "1/16000",
		Height:           NfoVideo.Height,
		Width:            NfoVideo.Width,
		Codec:            NfoVideo.Codec,
		VideoRange:       "SDR",
		VideoRangeType:   "SDR",
	}
	switch strings.ToLower(NfoVideo.Codec) {
	case "avc":
		fallthrough
	case "x264":
		fallthrough
	case "h264":
		videostream.Codec = "h264"
		videostream.CodecTag = "avc1"
	case "x265":
		fallthrough
	case "h265":
		fallthrough
	case "hevc":
		videostream.Codec = "hevc"
		videostream.CodecTag = "hvc1"
	default:
		log.Printf("Nfo of %s has unknown video codec %s", filename, NfoVideo.Codec)
	}

	mediasource.MediaStreams = append(mediasource.MediaStreams, videostream)

	// Create audio stream with high-level details based upon NFO
	audiostream := JFMediaStreams{
		Index:              1,
		Type:               "Audio",
		Language:           language,
		TimeBase:           "1/48000",
		SampleRate:         48000,
		AudioSpatialFormat: "None",
		LocalizedDefault:   "Default",
		LocalizedExternal:  "External",
		IsInterlaced:       false,
		IsAVC:              false,
		IsDefault:          true,
		VideoRange:         "Unknown",
		VideoRangeType:     "Unknown",
	}

	NfoAudio := n.FileInfo.StreamDetails.Audio
	audiostream.BitRate = NfoAudio.Bitrate
	audiostream.Channels = NfoAudio.Channels

	switch NfoAudio.Channels {
	case 1:
		audiostream.Title = "Mono"
		audiostream.ChannelLayout = "mono"
	case 2:
		audiostream.Title = "Stereo"
		audiostream.ChannelLayout = "stereo"
	case 3:
		audiostream.Title = "2.1 Channel"
		audiostream.ChannelLayout = "3.0"
	case 4:
		audiostream.Title = "3.1 Channel"
		audiostream.ChannelLayout = "4.0"
	case 5:
		audiostream.Title = "4.1 Channel"
		audiostream.ChannelLayout = "5.0"
	case 6:
		audiostream.Title = "5.1 Channel"
		audiostream.ChannelLayout = "5.1"
	default:
		log.Printf("Nfo of %s has unknown audio channel configuration %d", filename, NfoAudio.Channels)
	}

	switch strings.ToLower(NfoAudio.Codec) {
	case "ac3":
		audiostream.Codec = "ac3"
		audiostream.CodecTag = "ac-3"
	case "aac":
		audiostream.Codec = "aac"
		audiostream.CodecTag = "mp4a"
	default:
		log.Printf("Nfo of %s has unknown audio codec %s", filename, NfoAudio.Codec)
	}

	audiostream.DisplayTitle = audiostream.Title + " - " + strings.ToUpper(audiostream.Codec)

	mediasource.MediaStreams = append(mediasource.MediaStreams, audiostream)

	return []JFMediaSources{mediasource}
}

func genCollectionID(id int) (collectionID string) {
	collectionID = itemprefix_collection + fmt.Sprintf("%d", id)
	return
}

func getItemByID(itemId string) (c *Collection, i *Item) {
	for _, c := range config.Collections {
		if i = getItem(c.Name_, itemId); i != nil {
			return &c, i
		}
	}
	return nil, nil
}

func getSeasonByID(saesonId string) (*Collection, *Item, *Season) {
	saesonId = strings.TrimPrefix(saesonId, itemprefix_season)

	// fixme: wooho O(n^^3) "just temporarily.."
	for _, c := range config.Collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				if s.Id == saesonId {
					return &c, i, &s
				}
			}
		}
	}
	return nil, nil, nil
}

func getEpisodeByID(episodeId string) (*Collection, *Item, *Season, *Episode) {
	episodeId = strings.TrimPrefix(episodeId, itemprefix_episode)

	// fixme: wooho O(n^^4) "just temporarily.."
	for _, c := range config.Collections {
		for _, i := range c.Items {
			for _, s := range i.Seasons {
				for _, e := range s.Episodes {
					if e.Id == episodeId {
						return &c, i, &s, &e
					}

				}
			}
		}
	}
	return nil, nil, nil, nil
}

func parseTime(input string) (parsedTime time.Time, err error) {
	timeFormats := []string{
		"15:04:05",
		"2006-01-02",
		"2006/01/02",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"02 Jan 2006",
		"02 Jan 2006 15:04:05",
	}

	// Try each format until one succeeds
	for _, format := range timeFormats {
		if parsedTime, err = time.Parse(format, input); err == nil {
			// log.Printf("Parsed: %s as %v\n", input, parsedTime)
			return
		}
	}
	return
}
