package jellyfin

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"

	"github.com/miquels/notflix-server/collection"
	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/idhash"
	"github.com/miquels/notflix-server/imageresize"
)

type Configuration struct {
	// Indicates if we should auto-register Jellyfin users
	AutoRegister bool
	// JPEG quality for posters
	ImageQualityPoster int
}

type Jellyfin struct {
	config       *Configuration
	Collections  []collection.Collection
	imageresizer *imageresize.Resizer
}

// API definitions: https://swagger.emby.media/ & https://api.jellyfin.org/
// Docs: https://github.com/mediabrowser/emby/wiki

func New(conf *Configuration, collection []collection.Collection,
	ir *imageresize.Resizer) *Jellyfin {
	return &Jellyfin{
		config:       conf,
		Collections:  collection,
		imageresizer: ir,
	}
}

func (j *Jellyfin) RegisterHandlers(s *mux.Router) {
	r := s.UseEncodedPath()

	// Endpoints without auth
	r.Handle("/System/Info/Public", http.HandlerFunc(j.systemInfoHandler))
	r.Handle("/Users/AuthenticateByName", http.HandlerFunc(j.usersAuthenticateByNameHandler)).Methods("POST")

	middleware := func(handler http.HandlerFunc) http.Handler {
		return handlers.CompressHandler(j.authmiddleware(http.HandlerFunc(handler)))
	}

	// Endpoints with auth and gzip middleware
	r.Handle("/DisplayPreferences/usersettings", middleware(j.displayPreferencesHandler))
	r.Handle("/Users/{user}", middleware(j.usersHandler))
	r.Handle("/Users/{user}/Views", middleware(j.usersViewsHandler))
	r.Handle("/Users/{user}/GroupingOptions", middleware(j.usersGroupingOptionsHandler))
	r.Handle("/Users/{user}/Items", middleware(j.usersItemsHandler))
	r.Handle("/Users/{user}/Items/Latest", middleware(j.usersItemsLatestHandler))
	r.Handle("/Users/{user}/Items/{item}", middleware(j.usersItemHandler))
	r.Handle("/Users/{user}/Items/Resume", middleware(j.usersItemsResumeHandler))
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsPostHandler)).Methods("POST")
	r.Handle("/Users/{user}/PlayedItems/{item}", middleware(j.usersPlayedItemsDeleteHandler)).Methods("DELETE")

	r.Handle("/Library/VirtualFolders", middleware(j.libraryVirtualFoldersHandler))
	r.Handle("/Shows/NextUp", middleware(j.showsNextUpHandler))
	r.Handle("/Shows/{show}/Seasons", middleware(j.showsSeasonsHandler))
	r.Handle("/Shows/{show}/Episodes", middleware(j.showsEpisodesHandler))

	r.Handle("/Items/{item}", middleware(j.itemsDeleteHandler)).Methods("DELETE")
	r.Handle("/Items/{item}/Images/{type}", middleware(j.itemsImagesHandler))
	r.Handle("/Items/{item}/PlaybackInfo", middleware(j.itemsPlaybackInfoHandler))
	r.Handle("/MediaSegments/{item}", middleware(j.mediaSegmentsHandler))
	r.Handle("/Videos/{item}/stream", middleware(j.videoStreamHandler))

	r.Handle("/Persons", middleware(j.personsHandler))

	r.Handle("/Sessions/Playing", middleware(j.sessionsPlayingHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Progress", middleware(j.sessionsPlayingProgressHandler)).Methods("POST")
	r.Handle("/Sessions/Playing/Stopped", middleware(j.sessionsPlayingStoppedHandler)).Methods("POST")
}

type contextKey string

const (
	// Misc IDs for api responses
	serverID             = "2b11644442754f02a0c1e45d2a9f5c71"
	collectionRootID     = "e9d5075a555c1cbc394eec4cef295274"
	displayPreferencesID = "f137a2dd21bbc1b99aa5c0f6bf02a805"
	CollectionMovies     = "movies"
	CollectionTVShows    = "tvshows"

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
func (j *Jellyfin) systemInfoHandler(w http.ResponseWriter, r *http.Request) {
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
func (j *Jellyfin) usersAuthenticateByNameHandler(w http.ResponseWriter, r *http.Request) {
	var request JFAuthenticateUserByName
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusUnauthorized)
		return
	}

	if len(*request.Username) == 0 || len(*request.Pw) == 0 {
		http.Error(w, "username and password required", http.StatusUnauthorized)
		return
	}

	embyHeader, err := j.parseAuthHeader(r)
	if err != nil || embyHeader == nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	user, err := database.UserValidate(request.Username, request.Pw)
	if err != nil {
		if err == database.ErrUserNotFound && j.config.AutoRegister {
			user, err = database.UserInsert(*request.Username, *request.Pw)
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
func (j *Jellyfin) authmiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		embyHeader, err := j.parseAuthHeader(r)
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
func (j *Jellyfin) getAccessTokenDetails(r *http.Request) *AccessToken {
	// This should have been populated by authmiddleware(j.)
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
func (j *Jellyfin) parseAuthHeader(r *http.Request) (*AuthHeaderValues, error) {
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

// curl -v http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5
func (j *Jellyfin) usersHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
func (j *Jellyfin) usersViewsHandler(w http.ResponseWriter, r *http.Request) {
	items := []JFItem{}
	for _, c := range j.Collections {
		if item, err := j.buildJFItemCollection(genCollectionID(c.SourceId)); err == nil {
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
func (j *Jellyfin) usersGroupingOptionsHandler(w http.ResponseWriter, r *http.Request) {
	collections := []JFCollection{}
	for _, c := range j.Collections {
		collectionItem, _ := j.buildJFItemCollection(genCollectionID(c.SourceId))
		collection := JFCollection{
			Name: c.Name_,
			ID:   collectionItem.ID,
		}
		collections = append(collections, collection)
	}
	serveJSON(collections, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Resume?Limit=12&MediaTypes=Video&Recursive=true&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
func (j *Jellyfin) usersItemsResumeHandler(w http.ResponseWriter, r *http.Request) {
	response := JFUsersItemsResumeResponse{
		Items:            []string{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/f137a2dd21bbc1b99aa5c0f6bf02a805?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// handle individual item: any type: collection, a movie/show or individual file
func (j *Jellyfin) usersItemHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
			collectionItem, err := j.buildJFItemCollection(itemId)
			if err != nil {
				http.Error(w, "Could not find collection", http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case "season":
			seasonItem, err := j.buildJFItemSeason(accessTokenDetails.session.UserId, itemId)
			if err != nil {
				http.Error(w, "Could not find season", http.StatusNotFound)
				return
			}
			serveJSON(seasonItem, w)
			return
		case "episode":
			episodeItem, err := j.buildJFItemEpisode(accessTokenDetails.session.UserId, itemId)
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
	c, i := collection.GetItemByID(itemId)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	serveJSON(j.buildJFItem(accessTokenDetails.session.UserId, i, idhash.IdHash(c.Name_), c.Type, false), w)
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
func (j *Jellyfin) usersItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	queryparams := r.URL.Query()
	searchTerm := queryparams.Get("SearchTerm")

	searchCollection := queryparams.Get("ParentId")
	var searchC *collection.Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = collection.GetCollection(collectionid)
	}

	items := []JFItem{}
	for _, c := range j.Collections {
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
				items = append(items, j.buildJFItem(accessTokenDetails.session.UserId, i, idhash.IdHash(c.Name_), c.Type, true))
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
func (j *Jellyfin) usersItemsLatestHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	queryparams := r.URL.Query()
	searchCollection := queryparams.Get("ParentId")
	var searchC *collection.Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = collection.GetCollection(collectionid)
	}

	items := []JFItem{}
	for _, c := range j.Collections {
		// Skip if we are searching in one particular collection
		if searchC != nil && searchC.SourceId != c.SourceId {
			continue
		}
		for _, i := range c.Items {
			items = append(items, j.buildJFItem(accessTokenDetails.session.UserId, i, idhash.IdHash(c.Name_), c.Type, true))
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
func (j *Jellyfin) libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	libraries := []JFMediaLibrary{}
	for _, c := range j.Collections {
		collectionItem, _ := j.buildJFItemCollection(genCollectionID(c.SourceId))
		l := JFMediaLibrary{
			Name:               c.Name_,
			ItemId:             collectionItem.ID,
			PrimaryImageItemId: collectionItem.ID,
			Locations:          []string{"/"},
			CollectionType:     collectionItem.Type,
		}
		// switch c.Type {
		// case collection.CollectionMovies:
		// 	l.CollectionType = CollectionMovies
		// case collection.CollectionShows:
		// 	l.CollectionType = CollectionTVShows
		// }
		libraries = append(libraries, l)
	}
	serveJSON(libraries, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/4QBdg3S803G190AgFrBf/Seasons?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// generate season overview
func (j *Jellyfin) showsSeasonsHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	showId := vars["show"]
	_, i := collection.GetItemByID(showId)
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}
	// Create API response
	seasons := []JFItem{}
	for _, s := range i.Seasons {
		season, err := j.buildJFItemSeason(accessTokenDetails.session.UserId, s.Id)
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
func (j *Jellyfin) showsEpisodesHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	_, i := collection.GetItemByID(vars["show"])
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
			episode, err := j.buildJFItemEpisode(accessTokenDetails.session.UserId, episodeId)
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

func (j *Jellyfin) itemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not implemented", http.StatusForbidden)
}

// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Backdrop?tag=7cec54f0c8f362c75588e83d76fefa75'
// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Logo?tag=e28fbe648d2dbb76b65c14f14e6b1d72'
// curl -v 'http://127.0.0.1:9090/Items/q2e2UzCOd9zkmJenIOph/Images/Primary?tag=70931a7d8c147c9e2c0aafbad99e03e5'
// curl -v 'http://127.0.0.1:9090/Items/rVFG3EzPthk2wowNkqUl/Images/Primary?tag=268b80952354f01d5a184ed64b36dd52'
// curl -v 'http://127.0.0.1:9090/Items/2vx0ZYKeHxbh5iWhloIB/Images/Primary?tag=redirect_https://image.tmdb.org/t/p/original/3E4x5doNuuu6i9Mef6HPrlZjNb1.jpg'

func (j *Jellyfin) itemsImagesHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
		j.serveFile(w, r, strings.TrimPrefix(tag, tagprefix_file))
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
			c, item, season := collection.GetSeasonByID(trimPrefix(itemId))
			if season == nil {
				http.Error(w, "Could not find season", http.StatusNotFound)
				return
			}
			switch imageType {
			case "Primary":
				w.Header().Set("cache-control", "max-age=2592000")
				j.serveImage(w, r, c.Directory+"/"+item.Name+"/"+season.Poster,
					j.config.ImageQualityPoster)
				return
			default:
				log.Printf("Image request %s, unknown type %s", itemId, imageType)
				return
			}
		case "episode":
			c, item, _, episode := collection.GetEpisodeByID(trimPrefix(itemId))
			if episode == nil {
				http.Error(w, "Item not found (could not find episode)", http.StatusNotFound)
				return
			}
			j.serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Thumb)
			return
		default:
			log.Printf("Image request for unknown prefix %s!", itemId)
			http.Error(w, "Unknown image item prefix", http.StatusInternalServerError)
			return
		}
	}

	c, i := collection.GetItemByID(itemId)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch vars["type"] {
	case "Primary":
		w.Header().Set("cache-control", "max-age=2592000")
		j.serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Poster, j.config.ImageQualityPoster)
		return
	case "Backdrop":
		w.Header().Set("cache-control", "max-age=2592000")
		j.serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Fanart)
		return
		// We do not have artwork on disk for logo requests
		// case "Logo":
		// return
	}
	log.Printf("Unknown image type requested: %s\n", vars["type"])
	http.Error(w, "Item image not found", http.StatusNotFound)
}

// curl -v 'http://127.0.0.1:9090/Items/68d73f6f48efedb7db697bf9fee580cb/PlaybackInfo?UserId=2b1ec0a52b09456c9823a367d84ac9e5'
func (j *Jellyfin) itemsPlaybackInfoHandler(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// itemId := vars["item"]

	// c, i := getItemByID(itemId)
	// if i == nil || i.Video == "" {
	// 	http.Error(w, "Item not found", http.StatusNotFound)
	// 	return
	// }
	// item := buildJFItem(c, i, true)

	response := JFUsersPlaybackInfoResponse{
		MediaSources: j.buildMediaSource("test.mp4", nil),
		// TODO this static id should be generated based upon authenticated user
		// this id is used when submitting playstate via /Sessions/Playing endpoints
		PlaySessionID: "fc3b27127bf84ed89a300c6285d697e2",
	}
	serveJSON(response, w)
}

// return information about intro, commercial, preview, recap, outro segments
// of an item, not supported.
func (j *Jellyfin) mediaSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v -I 'http://127.0.0.1:9090/Videos/NrXTYiS6xAxFj4QAiJoT/stream'
func (j *Jellyfin) videoStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemId := vars["item"]

	// Is episode?
	if strings.HasPrefix(itemId, itemprefix_episode) {
		c, item, _, episode := collection.GetEpisodeByID(trimPrefix(itemId))
		if episode == nil {
			http.Error(w, "Could not find episode", http.StatusNotFound)
			return
		}
		j.serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Video)
		return
	}

	c, i := collection.GetItemByID(vars["item"])
	if i == nil || i.Video == "" {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	j.serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Video)
}

// return list of actors (hit by Infuse's search)
// not supported
func (j *Jellyfin) personsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/NextUp?UserId=2b1ec0a52b09456c9823a367d84ac9e5&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&StartIndex=0&Limit=20'
func (j *Jellyfin) showsNextUpHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	c, i := collection.GetItemByID("rVFG3EzPthk2wowNkqUl")
	response := JFShowsNextUpResponse{
		Items: []JFItem{
			j.buildJFItem(accessTokenDetails.session.UserId, i, idhash.IdHash(c.Name_), c.Type, true),
		},
		TotalRecordCount: 1,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

func (j *Jellyfin) usersPlayedItemsPostHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]

	j.playStateUpdate(accessTokenDetails.session.UserId, itemId, 0, true)
	w.WriteHeader(http.StatusOK)
}

func (j *Jellyfin) usersPlayedItemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
	if accessTokenDetails == nil {
		http.Error(w, "accesstoken not found in context", http.StatusUnauthorized)
		return
	}

	vars := mux.Vars(r)
	itemId := vars["item"]

	j.playStateUpdate(accessTokenDetails.session.UserId, itemId, 0, false)
	w.WriteHeader(http.StatusOK)
}

// PositionTicks are in micro seconds
const TicsToSeconds = 10000000

func (j *Jellyfin) sessionsPlayingHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
	j.playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) sessionsPlayingProgressHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
	j.playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) sessionsPlayingStoppedHandler(w http.ResponseWriter, r *http.Request) {
	accessTokenDetails := j.getAccessTokenDetails(r)
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
	j.playStateUpdate(accessTokenDetails.session.UserId, request.ItemId, request.PositionTicks, false)
	w.WriteHeader(http.StatusNoContent)
}

func (j *Jellyfin) playStateUpdate(userId, itemId string, positionTicks int, markAsWatched bool) (err error) {
	log.Printf("playStateUpdate UserId: %s, ItemId: %s, Progress: %d sec\n",
		userId, itemId, positionTicks/TicsToSeconds)

	var duration int
	if strings.HasPrefix(itemId, itemprefix_episode) {
		_, _, _, episode := collection.GetEpisodeByID(trimPrefix(itemId))

		// fix me: we should not have to load NFO here
		j.lazyLoadNFO(&episode.Nfo, episode.NfoPath)

		duration = episode.Nfo.FileInfo.StreamDetails.Video.DurationInSeconds
	} else {
		_, item := collection.GetItemByID(itemId)
		duration = item.Nfo.Runtime * 60
	}

	playstate := database.PlayStateEntry{
		Timestamp: time.Now().UTC(),
	}

	position := positionTicks / TicsToSeconds
	playedPercentage := 100 * position / duration

	// Mark as watched in case > 98% of the item is played
	if markAsWatched || playedPercentage >= 98 {
		playstate.Position = 0
		playstate.PlayedPercentage = 0
		playstate.Played = true
	} else {
		playstate.Position = position
		playstate.PlayedPercentage = playedPercentage
		playstate.Played = false
	}

	database.PlayState.Update(userId, itemId, playstate)
	return nil
}

func (j *Jellyfin) serveFile(w http.ResponseWriter, r *http.Request, filename string) {
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

func (j *Jellyfin) serveImage(w http.ResponseWriter, r *http.Request, filename string, imageQuality int) {
	file, err := j.imageresizer.OpenFile(w, r, filename, imageQuality)
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

func trimPrefix(s string) string {
	if i := strings.Index(s, itemprefix_separator); i != -1 {
		return s[i+1:]
	}
	return s
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

func serveJSON(obj interface{}, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	j := json.NewEncoder(w)
	j.SetIndent("", "  ")
	j.Encode(obj)
}
