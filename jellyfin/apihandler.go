package jellyfin

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
)

type contextKey string

const (
	// Context key holding access token details within a request
	contextAccessTokenDetails contextKey = "AccessTokenDetails"
)

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Views?IncludeExternalContent=false'
// and
// /UserViews
//
// usersViewsHandler returns the collections available to the user as items
func (j *Jellyfin) usersViewsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		if item, err := j.makeJFItemCollection(c.ID); err == nil {
			items = append(items, item)
		}
	}

	// Add favorites and playlist collections
	favoriteCollection, err := j.makeJFItemCollectionFavorites(r.Context(), accessToken.UserID)
	if err == nil {
		items = append(items, favoriteCollection)
	}
	playlistCollection, err := j.makeJFItemCollectionPlaylist(r.Context(), accessToken.UserID)
	if err == nil {
		items = append(items, playlistCollection)
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
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(c.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		collection := JFCollection{
			Name: collectionItem.Name,
			ID:   collectionItem.ID,
		}
		collections = append(collections, collection)
	}
	serveJSON(collections, w)
}

// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/f137a2dd21bbc1b99aa5c0f6bf02a805?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
//
// /Items/f137a2dd21bbc1b99aa5c0f6bf02a805?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
//
// usersItemHandler returns details for a specific item
func (j *Jellyfin) usersItemHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	switch {
	case isJFRootID(itemID):
		collectionItem, err := j.makeJFItemRoot()
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		}
		serveJSON(collectionItem, w)
		return
	// Try special collection items first, as they have the same prefix as regular collections
	case isJFCollectionFavoritesID(itemID):
		favoritesCollectionItem, err := j.makeJFItemCollectionFavorites(r.Context(), accessToken.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		}
		serveJSON(favoritesCollectionItem, w)
		return
	case isJFCollectionPlaylistID(itemID):
		PlayListCollectionItem, err := j.makeJFItemCollectionPlaylist(r.Context(), accessToken.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		serveJSON(PlayListCollectionItem, w)
		return
	case isJFCollectionID(itemID):
		collectionItem, err := j.makeJFItemCollection(trimPrefix(itemID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return

		}
		serveJSON(collectionItem, w)
		return
	case isJFPlaylistID(itemID):
		playlistItem, err := j.makeJFItemPlaylist(r.Context(), accessToken.UserID, trimPrefix(itemID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		serveJSON(playlistItem, w)
		return
	case isJFSeasonID(itemID):
		seasonItem, err := j.makeJFItemSeason(r.Context(), accessToken.UserID, trimPrefix(itemID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		serveJSON(seasonItem, w)
		return
	case isJFEpisodeID(itemID):
		episodeItem, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, trimPrefix(itemID))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		serveJSON(episodeItem, w)
		return
	}

	// Try to fetch individual item: movie or show
	c, i := j.collections.GetItemByID(itemID)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	serveJSON(j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, false), w)
}

// /UserItems/1d57ee2251656c5fb9a05becdf0e62a3/Userdata
//
// usersItemUserDataHandler returns the user data for a specific item
func (j *Jellyfin) usersItemUserDataHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	playstate, err := j.db.UserDataRepo.Get(r.Context(), accessToken.UserID, trimPrefix(itemID))
	if err != nil {
		// TODO: should we return an empty object or a 404?
		playstate = database.UserData{}
	}

	userData := j.makeJFUserData(accessToken.UserID, itemID, &playstate)
	serveJSON(userData, w)
}

// /Items
//
// /Users/{user}/Items
//
// usersItemsHandler returns list of items based upon provided quary params
//
// Supported query params:
// - ParentId, if provided scope result set to this collection
// - SearchTerm, substring to match on
// - StartIndex, index of first result item
// - Limit=50, number of items to return
func (j *Jellyfin) usersItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")

	items := make([]JFItem, 0)
	var err error
	var parentFound bool

	if parentID != "" {
		switch {
		// Return favorites collection items if requested
		case isJFCollectionFavoritesID(parentID):
			items, err = j.makeJFItemFavoritesOverview(r.Context(), accessToken.UserID)
			if err != nil {
				http.Error(w, "Could not find favorites collection", http.StatusNotFound)
				return
			}
			parentFound = true
		// Return list of playlists if requested
		case isJFCollectionPlaylistID(parentID):
			log.Printf("1")
			items, err = j.makeJFItemPlaylistOverview(r.Context(), accessToken.UserID)
			if err != nil {
				http.Error(w, "Could not find playlist collection", http.StatusNotFound)
				return
			}
			parentFound = true
		// Return specific playlist if requested
		case isJFPlaylistID(parentID):
			log.Printf("2")
			playlistID := trimPrefix(parentID)
			items, err = j.makeJFItemPlaylistItemList(r.Context(), accessToken.UserID, playlistID)
			if err != nil {
				http.Error(w, "Could not find playlist", http.StatusNotFound)
				return
			}
			parentFound = true
		}
	}

	// Search for items in case items was not yet populated earlier
	if !parentFound {
		var searchC *collection.Collection
		if parentID != "" {
			collectionid := strings.TrimPrefix(parentID, itemprefix_collection)
			searchC = j.collections.GetCollection(collectionid)
		}

		for _, c := range j.collections.GetCollections() {
			// Skip if we are searching in one particular collection?
			if searchC != nil && searchC.ID != c.ID {
				continue
			}
			for _, i := range c.Items {
				jfitem := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, true)
				if j.applyItemFilter(&jfitem, queryparams) {
					items = append(items, jfitem)
				}
			}
		}
	}

	totalItemCount := len(items)
	responseItems, startIndex := j.applyItemPaginating(j.applyItemSorting(items, queryparams), queryparams)
	response := UserItemsResponse{
		Items:            responseItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Items/ecd73bbc2244591343737b626e91418e/Ancestors
//
// usersItemsAncestorsHandler returns array with parent and root item
func (j *Jellyfin) usersItemsAncestorsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	c, i := j.collections.GetItemByID(itemID)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	collectionItem, err := j.makeJFItemCollection(c.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	root, _ := j.makeJFItemRoot()

	response := []JFItem{
		collectionItem,
		root,
	}
	serveJSON(response, w)
}

// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Latest?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&ParentId=f137a2dd21bbc1b99aa5c0f6bf02a805&StartIndex=0&Limit=20'
//
// usersItemsLatestHandler returns list of new items based upon provided quary params
//
// Supported query params:
// - ParentId, if provided scope result set to this collection
// - StartIndex, index of first result item
// - Limit=50, number of items to return
func (j *Jellyfin) usersItemsLatestHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	parentID := queryparams.Get("parentId")
	var searchC *collection.Collection
	if parentID != "" {
		collectionid := strings.TrimPrefix(parentID, itemprefix_collection)
		searchC = j.collections.GetCollection(collectionid)
	}

	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		// Skip if we are searching in one particular collection
		if searchC != nil && searchC.ID != c.ID {
			continue
		}
		for _, i := range c.Items {
			jfitem := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, true)
			if j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
		}
	}

	// Sort by premieredate to list most recent releases first
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PremiereDate.After(items[j].PremiereDate)
	})

	items, _ = j.applyItemPaginating(items, queryparams)

	serveJSON(items, w)
}

// /Search/Hints?includeItemTypes=Episode&limit=10&searchTerm=alien
//
// searchHintsHandler
func (j *Jellyfin) searchHintsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	// Return playlist collection items if requested
	parentID := queryparams.Get("parentId")
	if isJFCollectionPlaylistID(parentID) {
		response, _ := j.makeJFItemPlaylistOverview(r.Context(), accessToken.UserID)
		serveJSON(response, w)
		return
	}

	var searchC *collection.Collection
	if parentID != "" {
		collectionid := strings.TrimPrefix(parentID, itemprefix_collection)
		searchC = j.collections.GetCollection(collectionid)
	}

	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		// Skip if we are searching in one particular collection?
		if searchC != nil && searchC.ID != c.ID {
			continue
		}

		for _, i := range c.Items {
			jfitem := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, true)
			if j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
		}
	}

	totalItemCount := len(items)
	searchItems, _ := j.applyItemPaginating(j.applyItemSorting(items, queryparams), queryparams)

	response := SearchHintsResponse{
		SearchHints:      searchItems,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Shows/NextUp?
// 	enableImageTypes=Primary&
// 	enableImageTypes=Backdrop&
// 	enableImageTypes=Thumb&
// 	enableResumable=false&
// 	fields=MediaSourceCount&limit=20&

func (j *Jellyfin) showsNextUpHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	recentlyWatchedIDs, err := j.db.UserDataRepo.GetRecentlyWatched(r.Context(), accessToken.UserID, true)
	if err != nil {
		http.Error(w, "Could not get recently watched items list", http.StatusInternalServerError)
		return
	}
	nextUpItemIDs, err := j.collections.NextUp(recentlyWatchedIDs)
	if err != nil {
		http.Error(w, "Could not get next up items list", http.StatusInternalServerError)
		return
	}

	items := make([]JFItem, 0)
	for _, id := range nextUpItemIDs {
		// Any movies we should include?
		// if c, i := j.collections.GetItemByID(id); c != nil && i != nil {
		// 	if j.applyItemFilter(r.Context(), i, queryparams) {
		// 		items = append(items, j.makeJFItem(accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
		// 	}
		// 	continue
		// }
		if _, i, _, e := j.collections.GetEpisodeByID(id); i != nil {
			jfitem, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID)
			if err == nil && j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
			continue
		}
		log.Printf("usersItemsResumeHandler: item %s not found\n", id)
	}

	// Apply user provided sorting
	items = j.applyItemSorting(items, queryparams)

	totalItemCount := len(items)
	resumeItems, startIndex := j.applyItemPaginating(items, queryparams)
	response := JFShowsNextUpResponse{
		Items:            resumeItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /UserItems/Resume?userId=XAOVn7iqiBujnIQY8sd0&enableImageTypes=Primary&enableImageTypes=Backdrop&enableImageTypes=Thumb&includeItemTypes=Movie&includeItemTypes=Series&includeItemTypes=Episode
// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Resume?Limit=12&MediaTypes=Video&Recursive=true&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
//
// usersItemsResumeHandler returns a list of items that are resumable
func (j *Jellyfin) usersItemsResumeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	resumeItemIDs, err := j.db.UserDataRepo.GetRecentlyWatched(r.Context(), accessToken.UserID, false)
	if err != nil {
		http.Error(w, "Could not get resume items list", http.StatusInternalServerError)
		return
	}

	items := make([]JFItem, 0)
	for _, id := range resumeItemIDs {
		if c, i := j.collections.GetItemByID(id); c != nil && i != nil {
			jfitem := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, true)
			if j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
			continue
		}
		if _, i, _, e := j.collections.GetEpisodeByID(id); i != nil {
			jfitem, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID)
			if err == nil && j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
			continue
		}
		log.Printf("usersItemsResumeHandler: item %s not found\n", id)
	}

	// Apply user provided sorting
	items = j.applyItemSorting(items, queryparams)

	totalItemCount := len(items)
	resumeItems, startIndex := j.applyItemPaginating(items, queryparams)
	response := JFUsersItemsResumeResponse{
		Items:            resumeItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Items/Similar
//
// usersItemsSimilarHandler returns a list of items that are similar
func (j *Jellyfin) usersItemsSimilarHandler(w http.ResponseWriter, r *http.Request) {
	response := JFUsersItemsSimilarResponse{
		Items:            []JFItem{},
		StartIndex:       0,
		TotalRecordCount: 0,
	}
	serveJSON(response, w)
}

// /Items/Suggestions
//
// usersItemsSuggestionsHandler returns a list of items that are suggested for the user
func (j *Jellyfin) usersItemsSuggestionsHandler(w http.ResponseWriter, r *http.Request) {
	response := JFUsersItemsSuggestionsResponse{
		Items:            []JFItem{},
		StartIndex:       0,
		TotalRecordCount: 0,
	}
	serveJSON(response, w)
}

// curl -v http://127.0.0.1:9090/Library/VirtualFolders
func (j *Jellyfin) libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	libraries := []JFMediaLibrary{}
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(c.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		l := JFMediaLibrary{
			Name:               collectionItem.Name,
			ItemId:             collectionItem.ID,
			PrimaryImageItemId: collectionItem.ID,
			CollectionType:     collectionItem.Type,
			Locations:          []string{"/"},
		}
		libraries = append(libraries, l)
	}
	serveJSON(libraries, w)
}

// curl -v 'http://127.0.0.1:9090/Shows/4QBdg3S803G190AgFrBf/Seasons?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// generate season overview
func (j *Jellyfin) showsSeasonsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()

	showID := vars["show"]
	_, i := j.collections.GetItemByID(showID)
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}
	// Create API response
	seasons := make([]JFItem, 0)
	for _, s := range i.Seasons {
		jfitem, err := j.makeJFItemSeason(r.Context(), accessToken.UserID, s.ID)
		if err != nil {
			log.Printf("makeJFItemSeason returned error %s", err)
			continue
		}
		if j.applyItemFilter(&jfitem, queryparams) {
			seasons = append(seasons, jfitem)
		}
	}

	// Always sort seasons by number, no user provided sortBy option.
	// This way season 99, Specials ends up last.
	sort.SliceStable(seasons, func(i, j int) bool {
		return seasons[i].IndexNumber < seasons[j].IndexNumber
	})

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
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()

	c, i := j.collections.GetItemByID(vars["show"])
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}

	// Do we need to filter down overview by a particular season?
	requestedSeasonID := r.URL.Query().Get("seasonId")
	// FIXME/HACK: vidhub provides wrong season ids, so we cannot use them
	if strings.Contains(r.Header.Get("User-Agent"), "VidHub") {
		requestedSeasonID = ""
	}

	// Create API response for requested season
	episodes := make([]JFItem, 0)
	for _, s := range i.Seasons {
		// Limit results to one season if seasionid was provided.
		if requestedSeasonID != "" && requestedSeasonID != itemprefix_season+s.ID {
			continue
		}
		for _, e := range s.Episodes {
			if episode, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID); err == nil {
				jfitem := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID, c.Type, true)
				if j.applyItemFilter(&jfitem, queryparams) {
					episodes = append(episodes, episode)
				}
			}
		}
	}

	// Apply user provided sorting
	episodes = j.applyItemSorting(episodes, queryparams)

	response := UserItemsResponse{
		Items:            episodes,
		TotalRecordCount: len(episodes),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// applyItemFilter checks if the item should be included in a result set or not.
// returns true if the item should be included, false if it should be skipped.
func (j *Jellyfin) applyItemFilter(i *JFItem, queryparams url.Values) bool {
	// log.Printf("applyItemFilter: item %s, name: %s, type %s, parentID %s\n", i.ID, i.Name, i.Type, i.ParentID)

	// filter on search term
	if searchTerm := queryparams["searchTerm"]; searchTerm != nil && searchTerm[0] != "" {
		term := strings.ToLower(searchTerm[0])
		if !strings.Contains(strings.ToLower(i.Name), term) {
			// TODO: we should also search in other fields like overview and actors
			return false
		}
	}

	// media type filtering

	// includeItemTypes can be provided multiple times and contains a comma separated list of types
	// e.g. includeItemTypes=BoxSet&includeItemTypes=Movie,Series
	if includeItemTypes := queryparams["includeItemTypes"]; len(includeItemTypes) > 0 {
		keepItem := false
		for _, includeTypeEntry := range includeItemTypes {
			for includeType := range strings.SplitSeq(includeTypeEntry, ",") {
				if includeType == "Movie" && i.Type == itemTypeMovie {
					keepItem = true
				}
				if includeType == "Series" && i.Type == itemTypeShow {
					keepItem = true
				}
				if includeType == "Season" && i.Type == itemTypeSeason {
					keepItem = true
				}
				if includeType == "Episode" && i.Type == itemTypeEpisode {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// excludeItemTypes can be provided multiple times and contains a comma separated list of types
	// e.g. excludeItemTypes=BoxSet&excludeItemTypes=Movie,Series
	if excludeItemTypes := queryparams["excludeItemTypes"]; len(excludeItemTypes) > 0 {
		keepItem := true
		for _, excludeTypeEntry := range excludeItemTypes {
			for excludeType := range strings.SplitSeq(excludeTypeEntry, ",") {
				if excludeType == "Movie" && i.Type == itemTypeMovie {
					keepItem = false
				}
				if excludeType == "Series" && i.Type == itemTypeShow {
					keepItem = false
				}
				if excludeType == "Season" && i.Type == itemTypeSeason {
					keepItem = false
				}
				if excludeType == "Episode" && i.Type == itemTypeEpisode {
					keepItem = false
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// ID filtering

	// filter on item IDs
	if IDs := queryparams.Get("ids"); IDs != "" {
		keepItem := false
		for id := range strings.SplitSeq(IDs, ",") {
			if i.ID == id {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on item IDs to exclude
	if IDs := queryparams.Get("excludeItemIds"); IDs != "" {
		keepItem := true
		for id := range strings.SplitSeq(IDs, ",") {
			if i.ID == id {
				keepItem = false
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on genre IDs
	if includeGenresID := queryparams.Get("genreIds"); includeGenresID != "" {
		keepItem := false
		for genreID := range strings.SplitSeq(includeGenresID, "|") {
			for _, genre := range i.Genres {
				if makeJFGenreID(genre) == genreID {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on parentId
	if parentID := queryparams.Get("parentId"); parentID != "" {
		if i.ParentID != parentID {
			return false
		}
	}

	// filter on parentIndexNumber
	if parentIndexNumberStr := queryparams.Get("parentIndexNumber"); parentIndexNumberStr != "" {
		if parentIndexNumber, err := strconv.ParseInt(parentIndexNumberStr, 10, 64); err == nil {
			if i.ParentIndexNumber != int(parentIndexNumber) {
				return false
			}
		}
	}

	// filter on indexNumber
	if indexNumberStr := queryparams.Get("indexNumber"); indexNumberStr != "" {
		if indexNumber, err := strconv.ParseInt(indexNumberStr, 10, 64); err == nil {
			if i.IndexNumber != int(indexNumber) {
				return false
			}
		}
	}

	// filter on name prefix, case-insensitive.
	if nameStartsWith := queryparams.Get("nameStartsWith"); nameStartsWith != "" {
		if !strings.HasPrefix(strings.ToLower(i.SortName), strings.ToLower(nameStartsWith)) {
			return false
		}
	}

	// filter on name starting with or lexicographically greater than, case-insensitive.
	if nameStartsWithOrGreater := queryparams.Get("nameStartsWithOrGreater"); nameStartsWithOrGreater != "" {
		if strings.Compare(strings.ToLower(i.SortName), strings.ToLower(nameStartsWithOrGreater)) < 0 {
			return false
		}
	}

	// filter on name starting with or lexicographically less than, case-insensitive.
	if nameStartsWithOrLess := queryparams.Get("nameLessThan"); nameStartsWithOrLess != "" {
		if strings.Compare(strings.ToLower(i.SortName), strings.ToLower(nameStartsWithOrLess)) > 0 {
			return false
		}
	}

	// filter on genre name
	if includeGenres := queryparams.Get("genres"); includeGenres != "" {
		keepItem := false
		for genre := range strings.SplitSeq(includeGenres, "|") {
			if slices.Contains(i.Genres, genre) {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on offical rating
	if includeOfficialRatings := queryparams.Get("officialRatings"); includeOfficialRatings != "" {
		keepItem := false
		for rating := range strings.SplitSeq(includeOfficialRatings, "|") {
			if i.OfficialRating == rating {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on minCommunityRating
	if minCommunityRatingStr := queryparams.Get("minCommunityRating"); minCommunityRatingStr != "" {
		if minCommunityRating, err := strconv.ParseFloat(minCommunityRatingStr, 64); err == nil {
			if i.CommunityRating < minCommunityRating {
				return false
			}
		}
	}

	// filter on minCriticRating
	if minCriticRatingStr := queryparams.Get("minCriticRating"); minCriticRatingStr != "" {
		if minCriticRating, err := strconv.ParseFloat(minCriticRatingStr, 64); err == nil {
			if float64(i.CriticRating) < minCriticRating {
				return false
			}
		}
	}

	// todo: filter on minPremiereDate, maxPremiereDate

	// Filter on year(s)
	if filterYears := queryparams.Get("years"); filterYears != "" {
		keepItem := false
		for year := range strings.SplitSeq(filterYears, ",") {
			if intYear, err := strconv.ParseInt(year, 10, 64); err == nil {
				if i.ProductionYear == int(intYear) {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// Filter based upon isPlayed status
	if filterPlayed := strings.ToLower(queryparams.Get("isPlayed")); filterPlayed != "" {
		// Allow item if it was played
		if filterPlayed == "true" && i.UserData != nil && i.UserData.Played {
			return true
		}
		// Allow item if it was not played
		if filterPlayed == "false" && i.UserData != nil && i.UserData.Played {
			return true
		}
	}

	// Filter based upon isFavorite status
	if filterFavorite := strings.ToLower(queryparams.Get("isFavorite")); filterFavorite != "" {
		// Allow item if it should be favorite
		if filterFavorite == "true" && i.UserData.IsFavorite {
			return true
		}
		// Allow item if it not should be a favorite
		if filterFavorite == "false" && !i.UserData.IsFavorite {
			return true
		}
	}

	// Any other filters that we have to apply?
	if filters := queryparams.Get("filters"); filters != "" {
		for itemFilter := range strings.SplitSeq(filters, ",") {
			// Do we have to skip item in case favorites are requested?
			if itemFilter == "IsFavorite" || itemFilter == "IsFavoriteOrLikes" {
				// Allow item if it is a favorite
				if i.UserData != nil && i.UserData.IsFavorite {
					return true
				}
				// Not a favorite, so skip item
				return false
			}
		}
	}

	// No filters matched, so we keep the item
	return true
}

// applyItemSorting sorts a list of items based on the provided sortBy and sortOrder parameters
func (j *Jellyfin) applyItemSorting(items []JFItem, queryparams url.Values) (sortedItems []JFItem) {
	sortBy := queryparams.Get("sortBy")
	if sortBy == "" {
		return items
	}
	sortFields := strings.Split(sortBy, ",")

	sortOrder := queryparams.Get("sortOrder")
	var sortDescending bool
	if sortOrder == "Descending" {
		sortDescending = true
	}

	sort.SliceStable(items, func(i, j int) bool {
		for _, field := range sortFields {
			// Set sortname if not set so we can sort on it
			if items[i].SortName == "" {
				items[i].SortName = items[i].Name
			}

			switch strings.ToLower(field) {
			case "criticrating":
				if items[i].CriticRating != items[j].CriticRating {
					if sortDescending {
						return items[i].CriticRating > items[j].CriticRating
					}
					return items[i].CriticRating < items[j].CriticRating
				}
			case "datecreated":
				if items[i].DateCreated != items[j].DateCreated {
					if sortDescending {
						return items[i].DateCreated.After(items[j].DateCreated)
					}
					return items[i].DateCreated.Before(items[j].DateCreated)
				}
			case "dateplayed":
				if items[i].UserData != nil && items[j].UserData != nil &&
					items[i].UserData.LastPlayedDate != items[j].UserData.LastPlayedDate {
					if sortDescending {
						return items[i].UserData.LastPlayedDate.After(items[j].UserData.LastPlayedDate)
					}
					return items[i].UserData.LastPlayedDate.Before(items[j].UserData.LastPlayedDate)
				}
				return false
			case "datelastcontentadded":
				if items[i].DateCreated != items[j].DateCreated {
					if sortDescending {
						return items[i].DateCreated.After(items[j].DateCreated)
					}
					return items[i].DateCreated.Before(items[j].DateCreated)
				}
			case "indexnumber":
				if items[i].IndexNumber != items[j].IndexNumber {
					if sortDescending {
						return items[i].IndexNumber > items[j].IndexNumber
					}
					return items[i].IndexNumber < items[j].IndexNumber
				}
			case "isfolder":
				if items[i].IsFolder != items[j].IsFolder {
					if sortDescending {
						return items[i].IsFolder
					}
					return items[j].IsFolder
				}
			case "parentindexnumber":
				if items[i].ParentIndexNumber != items[j].ParentIndexNumber {
					if sortDescending {
						return items[i].ParentIndexNumber > items[j].ParentIndexNumber
					}
					return items[i].ParentIndexNumber < items[j].ParentIndexNumber
				}
			case "productionyear":
				if items[i].ProductionYear != items[j].ProductionYear {
					if sortDescending {
						return items[i].ProductionYear > items[j].ProductionYear
					}
					return items[i].ProductionYear < items[j].ProductionYear
				}
			case "random":
				if items[i].SortName != items[j].SortName {
					if rand.Intn(2) == 0 {
						return items[i].SortName > items[j].SortName
					}
					return items[i].SortName < items[j].SortName
				}
			case "seriessortname":
				fallthrough
			case "sortname":
				fallthrough
			case "default":
				if items[i].SortName != items[j].SortName {
					if sortDescending {
						return items[i].SortName > items[j].SortName
					}
					return items[i].SortName < items[j].SortName
				}
			default:
				log.Printf("applyItemSorting: unknown sortorder %s\n", sortBy)
			}
		}
		return false
	})
	return items
}

// apply pagination to a list of items
func (j *Jellyfin) applyItemPaginating(items []JFItem, queryparams url.Values) (paginatedItems []JFItem, startIndex int) {
	startIndex, startIndexErr := strconv.Atoi(queryparams.Get("startIndex"))
	if startIndexErr == nil && startIndex >= 0 && startIndex < len(items) {
		items = items[startIndex:]
	}
	limit, limitErr := strconv.Atoi(queryparams.Get("limit"))
	if limitErr == nil && limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	return items, startIndex
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
	itemID := vars["item"]
	imageType := vars["type"]

	switch {
	case isJFSeasonID(itemID):
		c, item, season := j.collections.GetSeasonByID(trimPrefix(itemID))
		if season == nil {
			http.Error(w, "Could not find season", http.StatusNotFound)
			return
		}
		switch strings.ToLower(imageType) {
		case "primary":
			// Serve season specific poster
			dir := c.Directory + "/" + item.Name + "/"
			if season.Poster != "" {
				j.serveImage(w, r, dir+season.Poster, j.imageQualityPoster)
				return
			}
			// Serve item season all poster
			if item.SeasonAllPoster != "" {
				j.serveImage(w, r, dir+item.SeasonAllPoster, j.imageQualityPoster)
				return
			}
			// Serve show poster as fallback
			if item.Poster != "" {
				j.serveImage(w, r, dir+item.Poster, j.imageQualityPoster)
				return
			}
			log.Printf("Image request %s, no poster found for season %s", itemID, season.ID)
			http.Error(w, "Poster not found for season", http.StatusNotFound)
			return
		default:
			log.Printf("Image request %s, unknown type %s", itemID, imageType)
			return
		}
	case isJFEpisodeID(itemID):
		c, item, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID))
		if episode == nil {
			http.Error(w, "Item not found (could not find episode)", http.StatusNotFound)
			return
		}
		if episode.Thumb != "" {
			j.serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Thumb)
			return
		}
		log.Printf("Image request %s, no thumbnail for episode %s", itemID, episode.ID)
		http.Error(w, "Thumbnail not found for episode", http.StatusNotFound)
		return
	case isJFCollectionID(itemID):
		fallthrough
	case isJFCollectionFavoritesID(itemID):
		fallthrough
	case isJFCollectionPlaylistID(itemID):
		log.Printf("Image request for collection %s!", itemID)
		http.Error(w, "Image request for collection not yet supported", http.StatusNotFound)
		return
	}

	c, i := j.collections.GetItemByID(itemID)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch strings.ToLower(imageType) {
	case "primary":
		if i.Poster != "" {
			j.serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Poster, j.imageQualityPoster)
			return
		}
		http.Error(w, "Poster not found", http.StatusNotFound)
		return
	case "backdrop":
		if i.Fanart != "" {
			j.serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Fanart)
			return
		}
		http.Error(w, "Backdrop not found", http.StatusNotFound)
		return
	case "logo":
		if i.Logo != "" {
			j.serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Logo, j.imageQualityPoster)
		}
		http.Error(w, "Logo not found", http.StatusNotFound)
		return
	}
	log.Printf("Unknown image type requested: %s\n", vars["type"])
	http.Error(w, "Item image not found", http.StatusNotFound)
}

// curl -v 'http://127.0.0.1:9090/Items/68d73f6f48efedb7db697bf9fee580cb/PlaybackInfo?UserId=2b1ec0a52b09456c9823a367d84ac9e5'
func (j *Jellyfin) itemsPlaybackInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]

	var mediaSource []JFMediaSources

	if _, i := j.collections.GetItemByID(itemID); i != nil {
		mediaSource = j.makeMediaSource(i.Video, i.Nfo)
	}

	if isJFEpisodeID(itemID) {
		if _, _, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID)); episode != nil {
			mediaSource = j.makeMediaSource(episode.Video, episode.Nfo)
		}
	}
	if mediaSource == nil {
		http.Error(w, "Could not find item", http.StatusNotFound)
		return
	}

	response := JFPlaybackInfoResponse{
		MediaSources: mediaSource,
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
	itemID := vars["item"]

	// Is episode?
	if isJFEpisodeID(itemID) {
		c, item, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID))
		if episode == nil {
			http.Error(w, "Could not find episode", http.StatusNotFound)
			return
		}
		j.serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Video)
		return
	}

	c, i := j.collections.GetItemByID(vars["item"])
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
	w.Header().Set("cache-control", "max-age=2592000")
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
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

func serveJSON(obj any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	j := json.NewEncoder(w)
	j.SetIndent("", "  ")
	j.Encode(obj)
}
