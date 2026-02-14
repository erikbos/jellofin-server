package jellyfin

import (
	"encoding/json"
	"fmt"
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
)

// /Items/f137a2dd21bbc1b99aa5c0f6bf02a805
// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/f137a2dd21bbc1b99aa5c0f6bf02a805
//
// usersItemHandler returns details for a specific item
func (j *Jellyfin) usersItemHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]

	response, err := j.makeJFItemByID(r.Context(), accessToken.UserID, itemID)
	if err != nil {
		apierror(w, err.Error(), http.StatusNotFound)
		return
	}
	serveJSON(response, w)
}

// /Items
// /Users/{user}/Items
//
// usersItemsHandler returns list of items based upon provided query params
//
// Supported query params:
// - parentId, if provided scope result set to this collection
// - searchTerm, search term to match items against
// - startIndex, index of first result item
// - limit=50, number of items to return
func (j *Jellyfin) usersItemsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")
	searchTerm := queryparams.Get("searchTerm")

	var items []JFItem
	var err error

	if searchTerm == "" {
		if parentID != "" {
			// Get list of items based upon provided parentID, this means
			// we are fetching items for a specific collection, season or series.
			items, err = j.getJFItemsByParentID(r.Context(), accessToken.UserID, parentID)
			if err != nil {
				apierror(w, err.Error(), http.StatusNotFound)
				return
			}
			// Remove parentID as we do not want applyItemsFilter() to act and filter on this later.
			queryparams.Del("parentId")
		} else {
			// No parentID provided. Now it gets interesting.. #observed api behaviour
			//
			// Case of:
			// 1) "ids" filter provided and found items -> we return those items, do not fetch top-level collection items
			// 2) "ids" did not find any items -> we return top-level collection items, do not apply "ids" filter as it did not find those items
			// 3) "ids" did not find any items and "recursive=true" is provided -> we return all items recursively

			// (1) Handle provided "ids", we fetch these directly by ID.
			var itemsFetchedByIDs bool
			if ids := queryparams.Get("ids"); ids != "" {
				items, err = j.makeJFItemByIDs(r.Context(), accessToken.UserID, strings.Split(ids, ","))
				if err != nil {
					apierror(w, err.Error(), http.StatusInternalServerError)
					return
				}
				if len(items) > 0 {
					itemsFetchedByIDs = true
				}
				// Remove ids as we do not want applyItemsFilter() to act and filter on this later.
				queryparams.Del("ids")
			}

			// (2) Get top-level collection items if no items found by IDs
			if !itemsFetchedByIDs {
				items, err = j.makeJFCollectionRootOverview(r.Context(), accessToken.UserID)
				if err != nil {
					apierror(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			// (3) No items found so far, add all media items recursively
			if !itemsFetchedByIDs && strings.EqualFold(queryparams.Get("recursive"), "true") {
				allitems, err := j.getJFItemsAll(r.Context(), accessToken.UserID)
				if err != nil {
					apierror(w, err.Error(), http.StatusNotFound)
					return
				}
				items = append(items, allitems...)
			}
		}
	}

	// If searchTerm is provided, filter items based on search results
	if searchTerm != "" {
		// If searchTerm is provided we search in whole collection,
		// applyItemFilter() will take care of parentID filtering
		foundItemIDs, err := j.collections.SearchItem(r.Context(), searchTerm)
		if foundItemIDs == nil || err != nil {
			apierror(w, "Search index not available", http.StatusInternalServerError)
			return
		}
		log.Printf("usersItemsHandler: search found %d matching items\n", len(foundItemIDs))
		// Build items list based on search result IDs
		items = make([]JFItem, 0, len(foundItemIDs))
		for _, id := range foundItemIDs {
			c, i := j.collections.GetItemByID(id)
			jfitem, err := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID)
			if err != nil {
				apierror(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, jfitem)
		}
	}

	items = j.applyItemsFilter(items, queryparams)

	totalItemCount := len(items)
	responseItems, startIndex := j.applyItemPaginating(j.applyItemSorting(items, queryparams), queryparams)
	response := UserItemsResponse{
		Items:            responseItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Items/Latest
// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Latest?Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount&ParentId=f137a2dd21bbc1b99aa5c0f6bf02a805&StartIndex=0&Limit=20
//
// Phyn
// /Items/Latest?enableImageTypes=Primary&enableImageTypes=Backdrop&enableImageTypes=Thumb&fields=PrimaryImageAspectRatio&fields=CanDelete&fields=ProviderIds&fields=SeasonUserData&groupItems=true&imageTypeLimit=1&limit=20&parentId=collection_1&userId=XAOVn7iqiBujnIQY8sd0
//
// Supported query params:
// - ParentId, if provided scope result set to this collection
// - StartIndex, index of first result item
// - Limit=50, number of items to return
//
// usersItemsLatestHandler returns list of new items based upon provided query params
func (j *Jellyfin) usersItemsLatestHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")

	var items []JFItem
	var err error
	// Get list of items based upon provided parentID
	if parentID != "" {
		items, err = j.getJFItemsByParentID(r.Context(), accessToken.UserID, parentID)
		if err != nil {
			apierror(w, err.Error(), http.StatusNotFound)
			return
		}
	} else {
		// All items recursively
		items, err = j.getJFItemsAll(r.Context(), accessToken.UserID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	items = j.applyItemsFilter(items, queryparams)

	// Sort by premieredate to list most recent releases first
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].PremiereDate.After(items[j].PremiereDate)
	})

	// Limit to returning max 50 items for latest releases
	if queryparams.Get("limit") == "" {
		queryparams.Set("limit", "50")
	}

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
			jfitem, err := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID)
			if err != nil {
				apierror(w, err.Error(), http.StatusInternalServerError)
				return
			}
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
		apierror(w, "Item not found", http.StatusNotFound)
		return
	}

	collectionItem, err := j.makeJFItemCollection(r.Context(), c.ID)
	if err != nil {
		apierror(w, err.Error(), http.StatusNotFound)
		return
	}
	root, _ := j.makeJFItemRoot(r.Context(), accessToken.UserID)

	response := []JFItem{
		collectionItem,
		root,
	}
	serveJSON(response, w)
}

// /Items/Counts
//
// usersItemsCountsHandler returns counts of movies, series and episodes
func (j *Jellyfin) usersItemsCountsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	stats := j.collections.GetStatistics()

	response := JFItemCountResponse{
		MovieCount:   stats.MovieCount,
		SeriesCount:  stats.ShowCount,
		EpisodeCount: stats.EpisodeCount,
	}
	serveJSON(response, w)
}

// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Items/Resume?Limit=12&MediaTypes=Video&Recursive=true&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount
// /UserItems/Resume?userId=XAOVn7iqiBujnIQY8sd0&enableImageTypes=Primary&enableImageTypes=Backdrop&enableImageTypes=Thumb&includeItemTypes=Movie&includeItemTypes=Series&includeItemTypes=Episode
//
// Phyn
// /UserItems/Resume?enableImageTypes=Primary&enableImageTypes=Backdrop&enableImageTypes=Thumb&enableImages=true&enableTotalRecordCount=false&excludeActiveSessions=false&fields=PrimaryImageAspectRatio&fields=CanDelete&fields=ProviderIds&fields=Path&imageTypeLimit=1&limit=20&mediaTypes=Audio&userId=XAOVn7iqiBujnIQY8sd0
//
// usersItemsResumeHandler returns a list of items that have not been fully watched and could be resumed
func (j *Jellyfin) usersItemsResumeHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	queryparams := r.URL.Query()

	resumeItemIDs, err := j.repo.GetRecentlyWatched(r.Context(), accessToken.UserID, false)
	if err != nil {
		apierror(w, "Could not get resume items list", http.StatusInternalServerError)
		return
	}

	items := make([]JFItem, 0, len(resumeItemIDs))
	for _, id := range resumeItemIDs {
		if c, i := j.collections.GetItemByID(id); c != nil && i != nil {
			jfitem, err := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID)
			if err != nil {
				apierror(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if j.applyItemFilter(&jfitem, queryparams) {
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

// /Items/{item}/Refresh
//
// usersItemsRefreshHandler refreshes the item metadata
func (j *Jellyfin) usersItemsRefreshHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented, return 204 to indicate refreshing item has been queud
	w.WriteHeader(http.StatusNoContent)
}

// /Items/Similar
//
// usersItemsSimilarHandler returns a list of items that are similar
func (j *Jellyfin) usersItemsSimilarHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	itemID := vars["item"]
	queryparams := r.URL.Query()

	// We support similar items for movies, series and episodes only.
	if isJFPersonID(itemID) ||
		isJFGenreID(itemID) ||
		isJFStudioID(itemID) ||
		isJFCollectionID(itemID) ||
		isJFCollectionFavoritesID(itemID) ||
		isJFCollectionPlaylistID(itemID) ||
		isJFRootID(itemID) {
		response := JFUsersItemsSimilarResponse{
			Items:            []JFItem{},
			StartIndex:       0,
			TotalRecordCount: 0,
		}
		serveJSON(response, w)
		return
	}

	// Retrieve item to find similars for
	c, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		apierror(w, "Item not found", http.StatusNotFound)
		return
	}

	similarItemIDs, err := j.collections.Similar(r.Context(), c, i)
	if err != nil {
		apierror(w, "Could not get similar items list", http.StatusInternalServerError)
		return
	}

	items := make([]JFItem, 0, len(similarItemIDs))
	for _, id := range similarItemIDs {
		c, i := j.collections.GetItemByID(id)
		jfitem, err := j.makeJFItem(r.Context(), accessToken.UserID, i, c.ID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if j.applyItemFilter(&jfitem, queryparams) {
			items = append(items, jfitem)
		}
	}

	totalItemCount := len(items)
	responseItems, startIndex := j.applyItemPaginating(j.applyItemSorting(items, queryparams), queryparams)
	response := JFUsersItemsSimilarResponse{
		Items:            responseItems,
		StartIndex:       startIndex,
		TotalRecordCount: totalItemCount,
	}
	serveJSON(response, w)
}

// /Items/{item}/Intros
// /Users/{user}/Items/{item}/Intros
func (j *Jellyfin) usersItemsIntrosHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented, return empty list
	response := UserItemsResponse{
		Items:            []JFItem{},
		StartIndex:       0,
		TotalRecordCount: 0,
	}
	serveJSON(response, w)
}

// /Items/{item}/SpecialFeatures
//
// usersItemsSpecialFeaturesHandler returns a list of items that are specials
func (j *Jellyfin) usersItemsSpecialFeaturesHandler(w http.ResponseWriter, r *http.Request) {
	// Currently not implemented, return empty list
	response := []JFItem{}
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

// applyItemsFilter applies filtering on a list of JFItems based on provided queryparams
func (j *Jellyfin) applyItemsFilter(items []JFItem, queryparams url.Values) []JFItem {
	// Apply filtering
	resultItems := make([]JFItem, 0, len(items))
	for _, item := range items {
		if j.applyItemFilter(&item, queryparams) {
			resultItems = append(resultItems, item)
		}
	}
	return resultItems
}

// applyItemFilter checks if the item should be included in a result set or not.
// returns true if the item should be included, false if it should be skipped.
func (j *Jellyfin) applyItemFilter(i *JFItem, queryparams url.Values) bool {
	// log.Printf("applyItemFilter: item %s, name: %s, type %s, parentID %s\n", i.ID, i.Name, i.Type, i.ParentID)
	//
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

	// media type filtering, top level categories: audio, video, photo, book
	if mediaType := queryparams.Get("mediaTypes"); mediaType != "" {
		keepItem := false
		for mediaTypeEntry := range strings.SplitSeq(mediaType, ",") {
			if mediaTypeEntry == "Video" &&
				(i.MediaType == itemTypeMovie || i.MediaType == itemTypeShow || i.MediaType == itemTypeSeason || i.MediaType == itemTypeEpisode) {
				keepItem = true
			}
			if mediaTypeEntry == "Audio" && (i.Type == itemTypeMusicAlbum || i.Type == itemTypeAudio) {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// isHd
	if isHD := queryparams.Get("isHd"); isHD != "" {
		switch strings.ToLower(isHD) {
		case "true":
			return i.IsHD
		case "false":
			return !i.IsHD
		}
	}

	// is4K
	if is4K := queryparams.Get("is4K"); is4K != "" {
		switch strings.ToLower(is4K) {
		case "true":
			return i.Is4K
		case "false":
			return !i.Is4K
		}
	}

	// ID filtering

	// filter on item includeItemIDs
	if includeItemIDs := queryparams.Get("ids"); includeItemIDs != "" {
		keepItem := false
		for id := range strings.SplitSeq(includeItemIDs, ",") {
			if i.ID == id {
				keepItem = true
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on item IDs to exclude
	if excludeItemIDs := queryparams.Get("excludeItemIds"); excludeItemIDs != "" {
		keepItem := true
		for excludeID := range strings.SplitSeq(excludeItemIDs, ",") {
			if i.ID == excludeID {
				keepItem = false
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on genre IDs
	if includeGenreIDs := queryparams.Get("genreIds"); includeGenreIDs != "" {
		keepItem := false
		for genreID := range strings.SplitSeq(includeGenreIDs, "|") {
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

	// filter on studio IDs
	if includeStudioIDs := queryparams.Get("studioIds"); includeStudioIDs != "" {
		keepItem := false
		for studioID := range strings.SplitSeq(includeStudioIDs, "|") {
			for _, studio := range i.Studios {
				if studio.ID == studioID {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on seriesId
	if seriesID := queryparams.Get("seriesId"); seriesID != "" {
		if i.SeriesID != seriesID {
			return false
		}
	}

	// filter on seasonId
	if seasonID := queryparams.Get("seasonId"); seasonID != "" {
		if i.SeasonID != seasonID {
			return false
		}
	}

	// filter on personIds
	if personIDs := queryparams.Get("personIds"); personIDs != "" {
		keepItem := false
		for personID := range strings.SplitSeq(personIDs, ",") {
			for _, person := range i.People {
				if person.ID == personID {
					keepItem = true
				}
			}
		}
		if !keepItem {
			return false
		}
	}

	// filter on parentIndexNumber, this usually refers to season number for episodes
	if parentIndexNumberStr := queryparams.Get("parentIndexNumber"); parentIndexNumberStr != "" {
		if parentIndexNumber, err := strconv.ParseInt(parentIndexNumberStr, 10, 64); err == nil {
			if i.ParentIndexNumber != int(parentIndexNumber) {
				return false
			}
		}
	}

	// filter on indexNumber, this usually refers to episode number for episodes
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

	// filter on studio name
	if includeStudios := queryparams.Get("studios"); includeStudios != "" {
		keepItem := false
		for studio := range strings.SplitSeq(includeStudios, "|") {
			for _, s := range i.Studios {
				if s.Name == studio {
					keepItem = true
				}
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
		if minCommunityRating, err := strconv.ParseFloat(minCommunityRatingStr, 32); err == nil {
			if i.CommunityRating < float32(minCommunityRating) {
				return false
			}
		}
	}

	// filter on minCriticRating
	if minCriticRatingStr := queryparams.Get("minCriticRating"); minCriticRatingStr != "" {
		if minCriticRating, err := strconv.ParseFloat(minCriticRatingStr, 32); err == nil {
			if float32(i.CriticRating) < float32(minCriticRating) {
				return false
			}
		}
	}

	// Filter on minPremierDate
	if minPremiereDateStr := queryparams.Get("minPremiereDate"); minPremiereDateStr != "" {
		if minPremiereDate, err := parseISO8601date(minPremiereDateStr); err == nil {
			if i.PremiereDate.Before(minPremiereDate) {
				return false
			}
		}
	}

	// Filter on maxPremierDate
	if maxPremiereDateStr := queryparams.Get("maxPremiereDate"); maxPremiereDateStr != "" {
		if maxPremiereDate, err := parseISO8601date(maxPremiereDateStr); err == nil {
			if i.PremiereDate.After(maxPremiereDate) {
				return false
			}
		}
	}

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
func (j *Jellyfin) applyItemSorting(items []JFItem, queryparams url.Values) []JFItem {
	sortBy := queryparams.Get("sortBy")
	if sortBy == "" {
		// No sorting fields provided, no sorting
		return items
	}
	sortFields := strings.Split(sortBy, ",")

	sortFieldsLowered := make([]string, len(sortFields))
	for i, field := range sortFields {
		sortFieldsLowered[i] = strings.ToLower(field)
	}

	var sortDescending bool
	if strings.ToLower(queryparams.Get("sortOrder")) == "descending" {
		sortDescending = true
	}

	sort.SliceStable(items, func(i, j int) bool {
		// Set sortname if not set so we can sort on it
		if items[i].SortName == "" {
			items[i].SortName = items[i].Name
		}

		for _, field := range sortFieldsLowered {
			switch field {
			case "communityrating":
				if items[i].CommunityRating != items[j].CommunityRating {
					if sortDescending {
						return items[i].CommunityRating > items[j].CommunityRating
					}
					return items[i].CommunityRating < items[j].CommunityRating
				}
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
			case "isfavoriteorliked":
				if items[i].UserData != nil && items[j].UserData != nil &&
					items[i].UserData.IsFavorite != items[j].UserData.IsFavorite {
					log.Printf("applyItemSorting: comparing isfavoriteorliked for items %s (%v) and %s (%v)\n",
						items[i].Name, items[i].UserData.IsFavorite, items[j].Name, items[j].UserData.IsFavorite)
					if sortDescending {
						return items[i].UserData.IsFavorite
					}
					return items[j].UserData.IsFavorite
				}
			case "isfolder":
				if items[i].IsFolder != items[j].IsFolder {
					if sortDescending {
						return items[i].IsFolder
					}
					return items[j].IsFolder
				}
			case "isplayed":
				if items[i].UserData != nil && items[j].UserData != nil &&
					items[i].UserData.Played != items[j].UserData.Played {
					if sortDescending {
						return items[i].UserData.Played
					}
					return items[j].UserData.Played
				}
			case "isunplayed":
				if items[i].UserData != nil && items[j].UserData != nil &&
					items[i].UserData.Played != items[j].UserData.Played {
					if sortDescending {
						return !items[i].UserData.Played
					}
					return !items[j].UserData.Played
				}
			case "officialrating":
				if items[i].OfficialRating != items[j].OfficialRating {
					if sortDescending {
						return items[i].OfficialRating > items[j].OfficialRating
					}
					return items[i].OfficialRating < items[j].OfficialRating
				}
			case "parentindexnumber":
				if items[i].ParentIndexNumber != items[j].ParentIndexNumber {
					if sortDescending {
						return items[i].ParentIndexNumber > items[j].ParentIndexNumber
					}
					return items[i].ParentIndexNumber < items[j].ParentIndexNumber
				}
			case "premieredate":
				if items[i].PremiereDate != items[j].PremiereDate {
					if sortDescending {
						return items[i].PremiereDate.After(items[j].PremiereDate)
					}
					return items[i].PremiereDate.Before(items[j].PremiereDate)
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
			case "runtime":
				if items[i].RunTimeTicks != items[j].RunTimeTicks {
					if sortDescending {
						return items[i].RunTimeTicks > items[j].RunTimeTicks
					}
					return items[i].RunTimeTicks < items[j].RunTimeTicks
				}
			case "name":
				fallthrough
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
				log.Printf("applyItemSorting: unknown sortorder field %s\n", field)
			}
		}
		return false
	})
	return items
}

// apply pagination to a list of items
func (j *Jellyfin) applyItemPaginating(items []JFItem, queryparams url.Values) ([]JFItem, int) {
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

// Items/{item} DELETE
//
// itemsDeleteHandler handles deleting an item
func (j *Jellyfin) itemsDeleteHandler(w http.ResponseWriter, r *http.Request) {
	apierror(w, "Not implemented", http.StatusForbidden)
}

// /Items/68d73f6f48efedb7db697bf9fee580cb/PlaybackInfo?UserId=2b1ec0a52b09456c9823a367d84ac9e5
//
// itemsPlaybackInfoHandler returns playback information about an item, including media sources
func (j *Jellyfin) itemsPlaybackInfoHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]

	_, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		apierror(w, "Could not find item", http.StatusNotFound)
		return
	}
	mediaSource := j.makeMediaSource(i)
	if mediaSource == nil {
		apierror(w, "Could not find item", http.StatusNotFound)
		return
	}

	response := JFPlaybackInfoResponse{
		MediaSources: mediaSource,
		// TODO this static id should be generated based upon authenticated user
		// this id is used when submitting playstate via /Sessions/Playing endpoints
		PlaySessionID: sessionID,
	}
	serveJSON(response, w)
}

// /Items/{item}/ThemeMedia
//
// usersItemsThemeMediaHandler
func (j *Jellyfin) usersItemsThemeMediaHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]

	response := JFItemThemeMediaResponse{
		ThemeVideosResult: JFItemThemeMediaResponseResult{
			OwnerID: itemID,
			Items:   []JFItem{},
		},
		ThemeSongsResult: JFItemThemeMediaResponseResult{
			OwnerID: itemID,
			Items:   []JFItem{},
		},
		SoundtrackSongsResult: JFItemThemeMediaResponseResult{
			OwnerID: "00000000000000000000000000000000",
			Items:   []JFItem{},
		},
	}
	serveJSON(response, w)
}

// /Items/NrXTYiS6xAxFj4QAiJoT/MediaSegments
//
// mediaSegmentsHandler returns information about intro, commercial, preview, recap, outro segments
// of an item, not supported.
func (j *Jellyfin) mediaSegmentsHandler(w http.ResponseWriter, r *http.Request) {
	response := UserItemsResponse{
		Items:            []JFItem{},
		TotalRecordCount: 0,
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Videos/NrXTYiS6xAxFj4QAiJoT/stream
//
// videoStreamHandler streams the actual video file to the client
func (j *Jellyfin) videoStreamHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	itemID := vars["item"]

	c, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil || i.FileName() == "" {
		apierror(w, "Item not found", http.StatusNotFound)
		return
	}
	w.Header().Set("content-type", mimeTypeByExtension(i.FileName()))
	j.serveFile(w, r, c.Directory+"/"+i.Path()+"/"+i.FileName())
}

func (j *Jellyfin) serveFile(w http.ResponseWriter, r *http.Request, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		apierror(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		apierror(w, "Could not retrieve file info", http.StatusInternalServerError)
		return
	}
	http.ServeContent(w, r, fileStat.Name(), fileStat.ModTime(), file)
}

func serveJSON(obj any, w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(obj)
}

// parseISO8601date tries to parse a date string in various ISO 8601 formats
func parseISO8601date(input string) (time.Time, error) {
	timeFormats := []string{
		time.DateTime,
		time.DateOnly,
		time.RFC3339,
		"2006-01",
		"2006",
	}
	for _, format := range timeFormats {
		if parsedTime, err := time.Parse(format, input); err == nil {
			return parsedTime, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time %s", input)
}
