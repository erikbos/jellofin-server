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
	for _, c := range j.collections.GetCollections() {
		collectionItem, err := j.makeJFItemCollection(c.ID)
		if err != nil {
			apierror(w, err.Error(), http.StatusInternalServerError)
			return
		}
		l := JFMediaLibrary{
			Name:               collectionItem.Name,
			ItemId:             collectionItem.ID,
			PrimaryImageItemId: collectionItem.ID,
			CollectionType:     collectionItem.Type,
			Locations:          []string{"/"},
		}
		response = append(response, l)
	}
	serveJSON(response, w)
}

// /UserViews
// /Users/2b1ec0a52b09456c9823a367d84ac9e5/Views?IncludeExternalContent=false
//
// usersViewsHandler returns the collections available to the user
func (j *Jellyfin) usersViewsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	items, err := j.makeJFCollectionRootOverview(r.Context(), accessToken.UserID)
	if err != nil {
		apierror(w, err.Error(), http.StatusInternalServerError)
		return
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
		collectionItem, err := j.makeJFItemCollection(c.ID)
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

// makeJFCollectionRootOverview creates a list of items representing the collections available to the user
func (j *Jellyfin) makeJFCollectionRootOverview(ctx context.Context, userID string) ([]JFItem, error) {
	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		if item, err := j.makeJFItemCollection(c.ID); err == nil {
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
func (j *Jellyfin) makeJFItemCollection(collectionID string) (JFItem, error) {
	c := j.collections.GetCollection(collectionID)
	if c == nil {
		return JFItem{}, errors.New("collection not found")
	}

	// Build list of genres from collection.
	var collectionGenres []string
	for _, i := range c.Items {
		for _, genre := range i.Genres() {
			if !slices.Contains(collectionGenres, genre) {
				collectionGenres = append(collectionGenres, genre)
			}
		}
	}

	id := makeJFCollectionID(collectionID)
	response := JFItem{
		Name:                     c.Name,
		ServerID:                 j.serverID,
		ID:                       id,
		ParentID:                 makeJFRootID(collectionRootID),
		Etag:                     idhash.IdHash(collectionID),
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
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
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
