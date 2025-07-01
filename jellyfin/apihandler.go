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
	"github.com/erikbos/jellofin-server/idhash"
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
		if item, err := j.makeJItemCollection(CollectionIDToString(c.ID)); err == nil {
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
		collectionItem, err := j.makeJItemCollection(CollectionIDToString(c.ID))
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

// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/f137a2dd21bbc1b99aa5c0f6bf02a805?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount'
// handle individual item: any type: collection, a movie/show or individual file
func (j *Jellyfin) usersItemHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	// itemID := vars["item"]

	splitted := strings.Split(vars["item"], itemprefix_separator)
	if len(splitted) == 2 {
		itemprefix := splitted[0] + itemprefix_separator
		itemID := splitted[1]
		switch itemprefix {
		case itemprefix_root:
			collectionItem, err := j.makeJFItemRoot()
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case itemprefix_collection:
			collectionItem, err := j.makeJItemCollection(itemID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case itemprefix_collection_favorites:
			collectionItem, err := j.makeJFItemCollectionFavorites(r.Context(), accessToken.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case itemprefix_collection_playlist:
			collectionItem, err := j.makeJFItemCollectionPlaylist(r.Context(), accessToken.UserID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return

			}
			serveJSON(collectionItem, w)
			return
		case itemprefix_season:
			seasonItem, err := j.makeJFItemSeason(r.Context(), accessToken.UserID, itemID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			serveJSON(seasonItem, w)
			return
		case itemprefix_episode:
			episodeItem, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, itemID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			serveJSON(episodeItem, w)
			return
		case itemprefix_playlist:
			playlistItem, err := j.makeJFItemPlaylist(r.Context(), accessToken.UserID, itemID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			serveJSON(playlistItem, w)
			return
		default:
			log.Print("Item request for unknown prefix!")
			http.Error(w, "Unknown item prefix", http.StatusInternalServerError)
			return
		}
	}

	// Try to find individual item
	c, i := j.collections.GetItemByID(vars["item"])
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}
	serveJSON(j.makeJFItem(r.Context(), accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, false), w)
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
		playstate = database.UserData{}
	}

	userData := j.makeJFUserData(accessToken.UserID, itemID, playstate)
	serveJSON(userData, w)
}

// /Items
//
// /Users/{user}/Items
//
// // curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items?
//
//	ExcludeLocationTypes=Virtual&
//	Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&ParentId=f137a2dd21bbc1b99aa5c0f6bf02a805&
//	SortBy=SortName,ProductionYear&
//	SortOrder=Ascending&
//	IncludeItemTypes=Movie&
//	Recursive=true&
//	StartIndex=0&Limit=50'
//
// find based upon title
// curl -v 'http://127.0.0.1:9090/Users/2b1ec0a52b09456c9823a367d84ac9e5/Items?
//
//	ExcludeLocationTypes=Virtual&
//	Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&
//	SearchTerm=p&
//	Recursive=true&Limit=24
//
// generate list of items based upon provided ParentId or a text searchTerm
// query params:
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
	searchCollection := queryparams.Get("parentId")

	items := make([]JFItem, 0)
	var err error
	var collectionPopulated bool

	// Return favorites collection if requested
	if strings.HasPrefix(searchCollection, itemprefix_collection_favorites) {
		items, err = j.makeJFItemFavoritesOverview(r.Context(), accessToken.UserID)
		if err != nil {
			http.Error(w, "Could not find favorites collection", http.StatusNotFound)
			return
		}
		collectionPopulated = true
	}

	// Return playlist collection if requested
	if strings.HasPrefix(searchCollection, itemprefix_collection_playlist) {
		items, err = j.makeJFItemPlaylistOverview(r.Context(), accessToken.UserID)
		if err != nil {
			http.Error(w, "Could not find playlist collection", http.StatusNotFound)
			return
		}
		collectionPopulated = true
	}

	// Search for items in case favorites or playlist collection not requested
	if !collectionPopulated {
		var searchC *collection.Collection
		if searchCollection != "" {
			collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
			searchC = j.collections.GetCollection(collectionid)
		}

		searchTerm := queryparams.Get("searchTerm")

		for _, c := range j.collections.GetCollections() {
			// Skip if we are searching in one particular collection?
			if searchC != nil && searchC.ID != c.ID {
				continue
			}

			for _, i := range c.Items {
				if searchTerm == "" || strings.Contains(strings.ToLower(i.Name), strings.ToLower(searchTerm)) {
					if j.applyItemFilter(i, queryparams) {
						items = append(items, j.makeJFItem(r.Context(), accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
					}
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
// returns array with parent and root item
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

	collectionItem, err := j.makeJItemCollection(CollectionIDToString(c.ID))
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
// generate list of new items based upon provided ParentId
// query params:
// - ParentId, if provided scope result set to this collection
// - StartIndex, index of first result item
// - Limit=50, number of items to return
func (j *Jellyfin) usersItemsLatestHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	searchCollection := queryparams.Get("parentId")
	var searchC *collection.Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = j.collections.GetCollection(collectionid)
	}

	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		// Skip if we are searching in one particular collection
		if searchC != nil && searchC.ID != c.ID {
			continue
		}
		for _, i := range c.Items {
			if j.applyItemFilter(i, queryparams) {
				items = append(items, j.makeJFItem(r.Context(), accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
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

	// Return playlist collection if requested
	searchCollection := queryparams.Get("parentId")
	if strings.HasPrefix(searchCollection, itemprefix_collection_playlist) {
		response, _ := j.makeJFItemPlaylistOverview(r.Context(), accessToken.UserID)
		serveJSON(response, w)
		return
	}

	var searchC *collection.Collection
	if searchCollection != "" {
		collectionid := strings.TrimPrefix(searchCollection, itemprefix_collection)
		searchC = j.collections.GetCollection(collectionid)
	}

	searchTerm := queryparams.Get("searchTerm")
	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		// Skip if we are searching in one particular collection?
		if searchC != nil && searchC.ID != c.ID {
			continue
		}

		for _, i := range c.Items {
			if searchTerm == "" || strings.Contains(strings.ToLower(i.Name), strings.ToLower(searchTerm)) {
				if j.applyItemFilter(i, queryparams) {
					items = append(items, j.makeJFItem(r.Context(), accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
				}
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
		// if c, i := j.collections.GetItemByID(id); c != nil && i != nil {
		// 	if j.applyItemFilter(i, queryparams) {
		// 		items = append(items, j.makeJFItem(accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
		// 	}
		// 	continue
		// }
		if _, i, _, e := j.collections.GetEpisodeByID(id); i != nil {
			if j.applyItemFilter(i, queryparams) {
				if episode, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID); err == nil {
					items = append(items, episode)
				}
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

	// c, i := j.collections.GetItemByID("rVFG3EzPthk2wowNkqUl")
	// response = JFShowsNextUpResponse{
	// 	Items: []JFItem{
	// 		j.makeJFItem(accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true),
	// 	},
	// 	TotalRecordCount: 1,
	// 	StartIndex:       0,
	// }
	// serveJSON(response, w)
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
			if j.applyItemFilter(i, queryparams) {
				items = append(items, j.makeJFItem(r.Context(), accessToken.UserID, i, idhash.IdHash(c.Name_), c.Type, true))
			}
			continue
		}
		if _, i, _, e := j.collections.GetEpisodeByID(id); i != nil {
			if j.applyItemFilter(i, queryparams) {
				if episode, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID); err == nil {
					items = append(items, episode)
				}
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

// applyItemFilter checks if the item should be included in a result set or not
func (j *Jellyfin) applyItemFilter(i *collection.Item, queryparams url.Values) bool {
	// includeItemTypes can be provided multiple times and contains a comma separated list of types
	// e.g. includeItemTypes=BoxSet&includeItemTypes=Movie,Series
	if includeItemTypes := queryparams["includeItemTypes"]; len(includeItemTypes) > 0 {
		keepItem := false
		for _, includeTypeEntry := range includeItemTypes {
			for includeType := range strings.SplitSeq(includeTypeEntry, ",") {
				if includeType == "Movie" && i.Type == collection.ItemTypeMovie {
					keepItem = true
				}
				if includeType == "Series" && i.Type == collection.ItemTypeShow {
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
				if excludeType == "Movie" && i.Type == collection.ItemTypeMovie {
					keepItem = true
				}
				if excludeType == "Series" && i.Type == collection.ItemTypeShow {
					keepItem = true
				}
			}
		}
		if !keepItem {
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

	// filter on genre name
	if includeGenresID := queryparams.Get("genreIds"); includeGenresID != "" {
		keepItem := false
		for includeType := range strings.SplitSeq(includeGenresID, "|") {
			for _, genre := range i.Genres {
				if idhash.IdHash(genre) == includeType {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on offical rating
	if includeOfficialRating := queryparams.Get("officialRatings"); includeOfficialRating != "" {
		keepItem := false
		for includeType := range strings.SplitSeq(includeOfficialRating, "|") {
			if i.OfficialRating == includeType {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// Do we have to skip item in case year filter is set?
	if filterYears := queryparams.Get("years"); filterYears != "" {
		keepItem := false
		for year := range strings.SplitSeq(filterYears, ",") {
			if intYear, err := strconv.ParseInt(year, 10, 64); err == nil {
				if i.Year == int(intYear) {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

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

// curl -v http://127.0.0.1:9090/Library/VirtualFolders
func (j *Jellyfin) libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	libraries := []JFMediaLibrary{}
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJItemCollection(CollectionIDToString(c.ID))
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
	showID := vars["show"]
	_, i := j.collections.GetItemByID(showID)
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}
	// Create API response
	seasons := make([]JFItem, 0)
	for _, s := range i.Seasons {
		season, err := j.makeJFItemSeason(r.Context(), accessToken.UserID, s.ID)
		if err != nil {
			log.Printf("makeJFItemSeason returned error %s", err)
			continue
		}
		seasons = append(seasons, season)
	}

	// Sort seasons, this way season 99, Specials ends up last
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
	_, i := j.collections.GetItemByID(vars["show"])
	if i == nil {
		http.Error(w, "Show not found", http.StatusNotFound)
		return
	}

	// Do we need to filter down overview by a particular season?
	RequestedSeasonID := r.URL.Query().Get("seasonId")
	// FIXME/HACK: vidhub provides wrong season ids, so we cannot use them
	if strings.Contains(r.Header.Get("User-Agent"), "VidHub") {
		RequestedSeasonID = ""
	}

	// Create API response for requested season
	episodes := make([]JFItem, 0)
	for _, s := range i.Seasons {
		// Limit results to a season if id provided
		if RequestedSeasonID != "" && itemprefix_season+s.ID != RequestedSeasonID {
			continue
		}
		for _, e := range s.Episodes {
			if episode, err := j.makeJFItemEpisode(r.Context(), accessToken.UserID, e.ID); err == nil {
				episodes = append(episodes, episode)
			}
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

	splitted := strings.Split(itemID, itemprefix_separator)
	splitted[0] += itemprefix_separator
	if len(splitted) == 2 {
		switch splitted[0] {
		// case "collection":
		// 	collectionItem, err := makeJFItemCollection(itemId)
		// 	if err != nil {
		// 		http.Error(w, "Could not find collection", http.StatusNotFound)
		// 		return

		// 	}
		// 	serveJSON(collectionItem, w)
		// 	return
		case itemprefix_season:
			c, item, season := j.collections.GetSeasonByID(trimPrefix(itemID))
			if season == nil {
				http.Error(w, "Could not find season", http.StatusNotFound)
				return
			}
			switch imageType {
			case "Primary":
				w.Header().Set("cache-control", "max-age=2592000")
				j.serveImage(w, r, c.Directory+"/"+item.Name+"/"+season.Poster,
					j.imageQualityPoster)
				return
			default:
				log.Printf("Image request %s, unknown type %s", itemID, imageType)
				return
			}
		case itemprefix_episode:
			c, item, _, episode := j.collections.GetEpisodeByID(trimPrefix(itemID))
			if episode == nil {
				http.Error(w, "Item not found (could not find episode)", http.StatusNotFound)
				return
			}
			j.serveFile(w, r, c.Directory+"/"+item.Name+"/"+episode.Thumb)
			return
		case itemprefix_collection:
			fallthrough
		case itemprefix_collection_favorites:
			fallthrough
		case itemprefix_collection_playlist:
			log.Printf("Image request for collection %s!", itemID)
			http.Error(w, "Image request for collection not yet supported", http.StatusNotFound)
			return
		default:
			log.Printf("Image request for unknown prefix %s!", itemID)
			http.Error(w, "Unknown image item prefix", http.StatusInternalServerError)
			return
		}
	}

	c, i := j.collections.GetItemByID(itemID)
	if i == nil {
		http.Error(w, "Item not found", http.StatusNotFound)
		return
	}

	switch strings.ToLower(vars["type"]) {
	case "primary":
		if i.Poster != "" {
			w.Header().Set("cache-control", "max-age=2592000")
			j.serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Poster, j.imageQualityPoster)
		} else {
			http.Error(w, "Poster not found", http.StatusNotFound)
		}
		return
	case "backdrop":
		if i.Fanart != "" {
			w.Header().Set("cache-control", "max-age=2592000")
			j.serveFile(w, r, c.Directory+"/"+i.Name+"/"+i.Fanart)
		} else {
			http.Error(w, "Backdrop not found", http.StatusNotFound)
		}
		return
	case "logo":
		if i.Logo != "" {
			w.Header().Set("cache-control", "max-age=2592000")
			j.serveImage(w, r, c.Directory+"/"+i.Name+"/"+i.Logo, j.imageQualityPoster)
		} else {
			http.Error(w, "Logo not found", http.StatusNotFound)
		}
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

	if strings.HasPrefix(itemID, itemprefix_episode) {
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
	if strings.HasPrefix(itemID, itemprefix_episode) {
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
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
}

// trimPrefix removes the type prefix from an item id.
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

func serveJSON(obj any, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	j := json.NewEncoder(w)
	j.SetIndent("", "  ")
	j.Encode(obj)
}
