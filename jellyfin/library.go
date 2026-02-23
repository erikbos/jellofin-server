package jellyfin

import (
	"context"
	"errors"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/idhash"
)

// /Library/VirtualFolders
//
// libraryVirtualFoldersHandler returns the available collections as virtual folders
func (j *Jellyfin) libraryVirtualFoldersHandler(w http.ResponseWriter, r *http.Request) {
	response := []JFMediaLibrary{}
	// todo: should this take EnabledFolders into account? Or is that only for the /UserViews endpoint?
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(r.Context(), c.ID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
			return
		}
		l := JFMediaLibrary{
			Name:           collectionItem.Name,
			ItemId:         collectionItem.ID,
			CollectionType: collectionItem.Type,
			Locations: []string{
				// stub directory path
				"/" + strings.ToLower(strings.Join(strings.Fields(collectionItem.Name), "")),
			},
		}
		if _, err := j.repo.HasImage(r.Context(), collectionItem.ID, imageTypePrimary); err == nil {
			l.PrimaryImageItemId = collectionItem.ID
		}
		response = append(response, l)
	}
	serveJSON(response, w)
}

// /UserViews
// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Views?IncludeExternalContent=false
//
// this is the same as /Library/MediaFolders, but a user configured ordering of items is applied
//
// usersViewsHandler returns collection list in order as configured by user
func (j *Jellyfin) usersViewsHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	items, err := j.makeJFCollectionRootOverview(r.Context(), reqCtx.User.ID)
	if err != nil {
		apierror(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("usersViewsHandler: EnableAllFolders: %v, EnabledFolders: %v, OrderedViews: %v, MyMediaExcludes: %v",
		reqCtx.User.Properties.EnableAllFolders, reqCtx.User.Properties.EnabledFolders, reqCtx.User.Properties.OrderedViews, reqCtx.User.Properties.MyMediaExcludes)

	for _, item := range items {
		log.Printf("usersViewsHandler: before filtering item: %s, DisplayPreferencesID: %s", item.ID, item.DisplayPreferencesID)
	}

	// If EnableAllFolders is false, we need to filter the items based on EnabledFolders
	if !reqCtx.User.Properties.EnableAllFolders {
		filteredItems := make([]JFItem, 0, len(items))
		for _, item := range items {
			if slices.Contains(reqCtx.User.Properties.EnabledFolders, item.ID) {
				filteredItems = append(filteredItems, item)
			}
		}
		items = filteredItems

		for _, item := range items {
			log.Printf("usersViewsHandler: after filtering item: %s, DisplayPreferencesID: %s", item.ID, item.DisplayPreferencesID)
		}
	}

	queryparams := r.URL.Query()
	includeHidden := queryparams.Get("includeHidden") == "true"

	// If the user has configured an order of views, we need to order the items based on that.
	// And exclude collection items unless includeHidden is true
	// Any items that are not in the user's ordered views will be added at the end.
	if len(reqCtx.User.Properties.OrderedViews) != 0 {
		// Order the items based on user preferences, and exclude items in my media excludes
		orderedItems := make([]JFItem, 0, len(items))
		seenItems := make(map[string]struct{})
		for _, displayPreferenceID := range reqCtx.User.Properties.OrderedViews {
			// If includeHidden is false, we need to exclude items that are in MyMediaExcludes
			if !includeHidden && slices.Contains(reqCtx.User.Properties.MyMediaExcludes, displayPreferenceID) {
				continue
			}
			for _, item := range items {
				if item.DisplayPreferencesID == displayPreferenceID {
					orderedItems = append(orderedItems, item)
					seenItems[item.ID] = struct{}{}
					break
				}
			}
		}
		// Append any items that were not included in the user's ordered views at the end of the list
		for _, item := range items {
			if _, exists := seenItems[item.ID]; !exists {
				orderedItems = append(orderedItems, item)
			}
		}
		items = orderedItems
	}

	for _, item := range items {
		log.Printf("usersViewsHandler: after ordering item: %s, DisplayPreferencesID: %s", item.ID, item.DisplayPreferencesID)
	}

	response := JFUserViewsResponse{
		Items:            items,
		TotalRecordCount: len(items),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Users/2b1ec0a52b09456c9823a367d84ac9e5/GroupingOptions
//
// usersGroupingOptionsHandler returns the available collections as grouping options
func (j *Jellyfin) usersGroupingOptionsHandler(w http.ResponseWriter, r *http.Request) {
	collections := []JFCollection{}
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(r.Context(), c.ID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
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

// POST /Library/Refresh
//
// libraryRefreshHandler triggers a library refresh
func (j *Jellyfin) libraryRefreshHandler(w http.ResponseWriter, r *http.Request) {
	reqCtx := j.getRequestCtx(w, r)
	if reqCtx == nil {
		return
	}
	// we just return 204 as we do not support this
	w.WriteHeader(http.StatusNoContent)
}

// makeJFItemRoot creates the top-level root item representing all collections
func (j *Jellyfin) makeJFItemRoot(ctx context.Context, userID string) (response JFItem, e error) {
	var childCount int
	if rootitems, err := j.makeJFCollectionRootOverview(ctx, userID); err == nil {
		childCount = len(rootitems)
	}
	// Build list of genres from all collections.
	var collectionGenres []string
	for _, c := range j.collections.GetCollections() {
		for _, genre := range c.Genres() {
			if !slices.Contains(collectionGenres, genre) {
				collectionGenres = append(collectionGenres, genre)
			}
		}
	}
	rootID := makeJFRootID(collectionRootID)
	response = JFItem{
		Name:                     "Media Folders",
		ServerID:                 j.serverID,
		ID:                       rootID,
		Etag:                     idhash.Hash(collectionRootID),
		DateCreated:              time.Now().UTC(),
		Type:                     itemTypeUserRootFolder,
		IsFolder:                 true,
		CanDelete:                false,
		CanDownload:              false,
		SortName:                 "media folders",
		ExternalUrls:             []JFExternalUrls{},
		Path:                     "/root",
		EnableMediaSourceDisplay: true,
		Taglines:                 []string{},
		PlayAccess:               "Full",
		RemoteTrailers:           []JFRemoteTrailers{},
		ProviderIds:              JFProviderIds{},
		People:                   []JFPeople{},
		Studios:                  []JFStudios{},
		Genres:                   collectionGenres,
		GenreItems:               makeJFGenreItems(collectionGenres),
		LocalTrailerCount:        0,
		ChildCount:               childCount,
		SpecialFeatureCount:      0,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(collectionRootID),
		Tags:                     []string{},
		PrimaryImageAspectRatio:  1.7777777777777777,
		BackdropImageTags:        []string{},
		LocationType:             "FileSystem",
		MediaType:                "Unknown",
		ImageTags:                j.makeJFImageTags(ctx, rootID, imageTypePrimary),
	}
	return
}

// makeJFCollectionRootOverview creates a list of items representing the collections available to the user
func (j *Jellyfin) makeJFCollectionRootOverview(ctx context.Context, userID string) ([]JFItem, error) {
	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		if item, err := j.makeJFItemCollection(ctx, c.ID); err == nil {
			items = append(items, item)
		}
	}
	// Add favorites and playlist collections
	if favoriteCollection, err := j.makeJFItemCollectionFavorites(ctx, userID); err == nil {
		items = append(items, favoriteCollection)
	}
	if playlistCollection, err := j.makeJFItemCollectionPlaylist(ctx, userID); err == nil {
		items = append(items, playlistCollection)
	}
	return items, nil
}

// makeJFItemCollection creates a JFItem representing a collection.
func (j *Jellyfin) makeJFItemCollection(ctx context.Context, collectionID string) (JFItem, error) {
	c := j.collections.GetCollection(collectionID)
	if c == nil {
		return JFItem{}, errors.New("collection not found")
	}
	id := makeJFCollectionID(collectionID)
	collectionGenres := c.Genres()
	response := JFItem{
		Name:                     c.Name,
		ServerID:                 j.serverID,
		ID:                       id,
		ParentID:                 makeJFRootID(collectionRootID),
		Etag:                     idhash.Hash(collectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		Type:                     itemTypeCollectionFolder,
		IsFolder:                 true,
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		CanDelete:                false,
		CanDownload:              true,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(collectionID),
		PlayAccess:               "Full",
		EnableMediaSourceDisplay: true,
		PrimaryImageAspectRatio:  1.7777777777777777,
		ChildCount:               len(c.Items),
		SpecialFeatureCount:      0,
		Genres:                   collectionGenres,
		GenreItems:               makeJFGenreItems(collectionGenres),
		ExternalUrls:             []JFExternalUrls{},
		RemoteTrailers:           []JFRemoteTrailers{},
		ImageTags:                j.makeJFImageTags(ctx, id, imageTypePrimary),
	}
	switch c.Type {
	case collection.CollectionTypeMovies:
		response.CollectionType = collectionTypeMovies
	case collection.CollectionTypeShows:
		response.CollectionType = collectionTypeTVShows
	default:
		log.Printf("makeJItemCollection: unknown collection type: %s", c.Type)
	}
	response.SortName = response.CollectionType
	return response, nil
}

// makeJFItemCollectionFavorites creates a collection item for favorites folder of the user.
func (j *Jellyfin) makeJFItemCollectionFavorites(ctx context.Context, userID string) (JFItem, error) {
	var itemCount int
	if favoriteIDs, err := j.repo.GetFavorites(ctx, userID); err == nil {
		itemCount = len(favoriteIDs)
	}

	id := makeJFCollectionFavoritesID(favoritesCollectionID)
	response := JFItem{
		Name:                     "Favorites",
		ServerID:                 j.serverID,
		ID:                       id,
		ParentID:                 makeJFRootID(collectionRootID),
		Etag:                     idhash.Hash(favoritesCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           collectionTypePlaylists,
		SortName:                 collectionTypePlaylists,
		Type:                     itemTypeUserView,
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               itemCount,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(favoritesCollectionID),
		ExternalUrls:             []JFExternalUrls{},
		PlayAccess:               "Full",
		PrimaryImageAspectRatio:  1.7777777777777777,
		RemoteTrailers:           []JFRemoteTrailers{},
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		ImageTags:                j.makeJFImageTags(ctx, id, imageTypePrimary),
		// PremiereDate should be set based upon most recent item in collection
	}
	return response, nil
}

// makeJFItemFavoritesOverview creates a list of favorite items.
func (j *Jellyfin) makeJFItemFavoritesOverview(ctx context.Context, userID string) ([]JFItem, error) {
	favoriteIDs, err := j.repo.GetFavorites(ctx, userID)
	if err != nil {
		return []JFItem{}, err
	}

	items := []JFItem{}
	for _, itemID := range favoriteIDs {
		if c, i := j.collections.GetItemByID(itemID); c != nil && i != nil {
			// We only add movies and shows in favorites
			switch i.(type) {
			case *collection.Movie, *collection.Show:
				jfitem, err := j.makeJFItem(ctx, userID, i, c.ID)
				if err != nil {
					return []JFItem{}, err
				}
				items = append(items, jfitem)
			}
		}
	}
	return items, nil
}

// makeJFRootID returns an external id for the root folder.
func makeJFRootID(rootID string) string {
	return itemprefix_root + rootID
}

// isJFRootID checks if the provided ID is a root ID.
func isJFRootID(id string) bool {
	return strings.HasPrefix(id, itemprefix_root)
}

// makeJFCollectionID returns an external id for a collection.
func makeJFCollectionID(collectionID string) string {
	return itemprefix_collection + collectionID
}

// isJFCollectionID checks if the provided ID is a collection ID.
func isJFCollectionID(id string) bool {
	return strings.HasPrefix(id, itemprefix_collection)
}

// makeJFCollectionFavoritesID returns an external id for a favorites collection.
func makeJFCollectionFavoritesID(favoritesID string) string {
	return itemprefix_collection_favorites + favoritesID
}

// isJFCollectionFavoritesID checks if the provided ID is a favorites collection ID.
func isJFCollectionFavoritesID(id string) bool {
	return strings.HasPrefix(id, itemprefix_collection_favorites)
}
