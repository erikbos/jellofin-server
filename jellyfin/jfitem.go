// contains all item related functions
package jellyfin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/collection/metadata"
	"github.com/erikbos/jellofin-server/database/model"
	"github.com/erikbos/jellofin-server/idhash"
)

const (
	// Top-level root ID, parent IDof all collections
	collectionRootID = "e9d5075a555c1cbc394eec4cef295274"
	// ID of dynamically generated Playlist collection
	playlistCollectionID = "2f0340563593c4d98b97c9bfa21ce23c"
	// ID of dynamically generated favorites collection
	favoritesCollectionID    = "f4a0b1c2d3e5c4b8a9e6f7d8e9a0b1c2"
	collectionTypeMovies     = "movies"
	collectionTypeTVShows    = "tvshows"
	collectionTypePlaylists  = "playlists"
	itemTypeUserRootFolder   = "UserRootFolder"
	itemTypeCollectionFolder = "CollectionFolder"
	itemTypeUserView         = "UserView"
	itemTypeMovie            = "Movie"
	itemTypeShow             = "Series"
	itemTypeSeason           = "Season"
	itemTypeEpisode          = "Episode"
	itemTypePlaylist         = "Playlist"
	itemTypeGenre            = "Genre"
	itemTypeStudio           = "Studio"
	itemTypePerson           = "Person"

	// imagetag prefix will get HTTP-redirected
	tagprefix_redirect = "redirect_"
)

// getJFItemsByParentID returns list of all items with a specific parentID
func (j *Jellyfin) getJFItemsByParentID(ctx context.Context, userID, parentID string) ([]JFItem, error) {
	switch {
	// List favorites collection items requested?
	case isJFCollectionFavoritesID(parentID):
		items, err := j.makeJFItemFavoritesOverview(ctx, userID)
		if err != nil {
			return []JFItem{}, errors.New("could not find favorites collection")
		}
		return items, nil

	// List of playlists requested?
	case isJFCollectionPlaylistID(parentID):
		items, err := j.makeJFItemPlaylistOverview(ctx, userID)
		if err != nil {
			return []JFItem{}, errors.New("could not find playlist collection")
		}
		return items, nil

	// Specific playlist requests?
	case isJFPlaylistID(parentID):
		playlistID := trimPrefix(parentID)
		items, err := j.makeJFItemPlaylistItemList(ctx, userID, playlistID)
		if err != nil {
			return []JFItem{}, errors.New("could not find playlist")
		}
		return items, nil

	// List by genre requested?
	case isJFGenreID(parentID):
		items, err := j.getJFItemsAll(ctx, userID)
		if err != nil {
			return []JFItem{}, errors.New("could not get all items")
		}
		// Make a new list with only items where genreid matches provided parentID
		genreItems := make([]JFItem, 0, len(items))
		for _, item := range items {
			for _, genre := range item.GenreItems {
				if genre.ID == parentID {
					genreItems = append(genreItems, item)
				}
			}
		}
		return genreItems, nil

	// List by studio?
	case isJFStudioID(parentID):
		items, err := j.getJFItemsAll(ctx, userID)
		if err != nil {
			return []JFItem{}, errors.New("could not get all items")
		}
		// Make a new list with only items where studioid matches provided parentID
		studioItems := make([]JFItem, 0, len(items))
		for _, item := range items {
			for _, studio := range item.Studios {
				if studio.ID == parentID {
					studioItems = append(studioItems, item)
				}
			}
		}
		return studioItems, nil

	// List by person?
	// case isJFPersonID(parentID):
	// 	return []JFItem{}, nil

	// Specific collection requested?
	case isJFCollectionID(parentID):
		c := j.collections.GetCollection(strings.TrimPrefix(parentID, itemprefix_collection))
		if c == nil {
			return []JFItem{}, errors.New("could not find collection")
		}
		items := make([]JFItem, 0, len(c.Items))
		for _, i := range c.Items {
			jfitem, err := j.makeJFItem(ctx, userID, i, c.ID)
			if err != nil {
				return []JFItem{}, err
			}
			items = append(items, jfitem)
		}
		return items, nil

	// Check if parentID is a show or season to generate overviews
	default:
		if _, i := j.collections.GetItemByID(trimPrefix(parentID)); i != nil {
			switch v := i.(type) {
			case *collection.Show:
				items, err := j.makeJFSeasonsOverview(ctx, userID, v)
				if err != nil {
					return []JFItem{}, errors.New("could not find parent show")
				}
				return items, nil
			case *collection.Season:
				items, err := j.makeJFEpisodesOverview(ctx, userID, v)
				if err != nil {
					return []JFItem{}, errors.New("could not find season")
				}
				return items, nil
			// Other item types are not supported as parentID
			default:
				log.Printf("getJFItemsByParentID: unsupported parentID %s of type %T \n", i.ID(), i)
				return []JFItem{}, errors.New("unable to generate items for parentID")
			}
		}
	}
	return []JFItem{}, errors.New("parentID not found")
}

// getJFItemsAll returns list of all items
func (j *Jellyfin) getJFItemsAll(ctx context.Context, userID string) ([]JFItem, error) {
	items := make([]JFItem, 0)
	for _, c := range j.collections.GetCollections() {
		for _, i := range c.Items {
			jfitem, err := j.makeJFItem(ctx, userID, i, c.ID)
			if err != nil {
				return []JFItem{}, err
			}
			items = append(items, jfitem)
		}
	}
	return items, nil
}

func (j *Jellyfin) makeJFItemRoot(ctx context.Context, userID string) (response JFItem, e error) {
	var childCount int
	if rootitems, err := j.makeJFCollectionRootOverview(ctx, userID); err == nil {
		childCount = len(rootitems)
	}

	genres := j.collections.Details().Genres

	response = JFItem{
		Name:                     "Media Folders",
		ServerID:                 j.serverID,
		ID:                       makeJFRootID(collectionRootID),
		Etag:                     idhash.IdHash(collectionRootID),
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
		Genres:                   genres,
		GenreItems:               makeJFGenreItems(genres),
		LocalTrailerCount:        0,
		ChildCount:               childCount,
		SpecialFeatureCount:      0,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(collectionRootID),
		Tags:                     []string{},
		PrimaryImageAspectRatio:  1.7777777777777777,
		BackdropImageTags:        []string{},
		LocationType:             "FileSystem",
		MediaType:                "Unknown",
		// ImageTags: &JFImageTags{
		// 	Primary: collectionRootID,
		// },
	}
	return
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

func (j *Jellyfin) makeJFItemCollection(collectionID string) (response JFItem, e error) {
	c := j.collections.GetCollection(collectionID)
	if c == nil {
		e = errors.New("collection not found")
		return
	}
	collectionGenres := c.Details().Genres

	id := makeJFCollectionID(collectionID)

	response = JFItem{
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
	return
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
		Etag:                     idhash.IdHash(favoritesCollectionID),
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
		// PremiereDate should be set based upon most recent item in collection
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
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

// makeJFItemCollectionPlaylist creates a top level collection item representing all playlists of the user
func (j *Jellyfin) makeJFItemCollectionPlaylist(ctx context.Context, userID string) (JFItem, error) {
	var itemCount int

	// Get total item count across all playlists
	if playlistIDs, err := j.repo.GetPlaylists(ctx, userID); err == nil {
		for _, ID := range playlistIDs {
			playlist, err := j.repo.GetPlaylist(ctx, userID, ID)
			if err == nil && playlist != nil {
				itemCount += len(playlist.ItemIDs)
			}
		}
	}

	id := makeJFCollectionPlaylistID(playlistCollectionID)

	response := JFItem{
		Name:                     "Playlists",
		ServerID:                 j.serverID,
		ID:                       id,
		ParentID:                 makeJFRootID(collectionRootID),
		Etag:                     idhash.IdHash(playlistCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           collectionTypePlaylists,
		SortName:                 collectionTypePlaylists,
		Type:                     itemTypeUserView,
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               itemCount,
		DisplayPreferencesID:     makeJFDisplayPreferencesID(playlistCollectionID),
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
		// PremiereDate should be set based upon most recent item in collection
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	return response, nil
}

// makeJFItemPlaylist creates a playlist item from the provided playlistID
func (j *Jellyfin) makeJFItemPlaylist(ctx context.Context, userID, playlistID string) (JFItem, error) {
	playlist, err := j.repo.GetPlaylist(ctx, userID, playlistID)
	if err != nil || playlist == nil {
		return JFItem{}, errors.New("could not find playlist")
	}

	response := JFItem{
		Type:                     itemTypePlaylist,
		ID:                       makeJFPlaylistID(playlist.ID),
		ParentID:                 makeJFCollectionPlaylistID(playlistCollectionID),
		ServerID:                 j.serverID,
		Name:                     playlist.Name,
		SortName:                 playlist.Name,
		IsFolder:                 true,
		Path:                     "/playlist",
		Etag:                     idhash.IdHash(playlist.ID),
		DateCreated:              time.Now().UTC(),
		CanDelete:                true,
		CanDownload:              true,
		PlayAccess:               "Full",
		RecursiveItemCount:       len(playlist.ItemIDs),
		ChildCount:               len(playlist.ItemIDs),
		LocationType:             "FileSystem",
		MediaType:                "Video",
		DisplayPreferencesID:     makeJFDisplayPreferencesID(playlistCollectionID),
		EnableMediaSourceDisplay: true,
	}
	return response, nil
}

// makeJFItemPlaylistOverview creates a list of playlists of the user.
func (j *Jellyfin) makeJFItemPlaylistOverview(ctx context.Context, userID string) ([]JFItem, error) {
	playlistIDs, err := j.repo.GetPlaylists(ctx, userID)
	if err != nil {
		return []JFItem{}, err
	}

	items := []JFItem{}
	for _, ID := range playlistIDs {
		if playlistItem, err := j.makeJFItemPlaylist(ctx, userID, ID); err == nil {
			items = append(items, playlistItem)
		}
	}
	return items, nil
}

// makeJFItemPlaylistItemList creates an item list of one playlist of the user.
func (j *Jellyfin) makeJFItemPlaylistItemList(ctx context.Context, userID, playlistID string) ([]JFItem, error) {

	playlist, err := j.repo.GetPlaylist(ctx, userID, playlistID)
	log.Printf("makeJFItemPlaylistItemList: %+v, %+v", playlistID, err)
	if err != nil {
		return []JFItem{}, err
	}

	items := []JFItem{}
	for _, itemID := range playlist.ItemIDs {
		c, i := j.collections.GetItemByID(itemID)
		if i != nil {
			item, err := j.makeJFItem(ctx, userID, i, c.ID)
			if err != nil {
				return []JFItem{}, err
			}
			items = append(items, item)
		}
	}
	return items, nil
}

// makeJFItemByID creates a JFItem based on the provided itemID
func (j *Jellyfin) makeJFItemByID(ctx context.Context, userID, itemID string) (JFItem, error) {
	// Handle special items first
	switch {
	case isJFRootID(itemID):
		return j.makeJFItemRoot(ctx, userID)
	// Try special collection items first, as they have the same prefix as regular collections
	case isJFCollectionFavoritesID(itemID):
		return j.makeJFItemCollectionFavorites(ctx, userID)
	case isJFCollectionPlaylistID(itemID):
		return j.makeJFItemCollectionPlaylist(ctx, userID)
	case isJFCollectionID(itemID):
		return j.makeJFItemCollection(trimPrefix(itemID))
	case isJFPlaylistID(itemID):
		return j.makeJFItemPlaylist(ctx, userID, trimPrefix(itemID))
	case isJFPersonID(itemID):
		return j.makeJFItemPerson(ctx, userID, trimPrefix(itemID))
	}

	// Try to fetch individual item: movie, show, episode
	c, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		return JFItem{}, errors.New("item not found")
	}
	return j.makeJFItem(ctx, userID, i, c.ID)
}

// makeJFItem make movie or show from provided item
func (j *Jellyfin) makeJFItem(ctx context.Context, userID string, item collection.Item, parentID string) (response JFItem, e error) {
	switch i := item.(type) {
	case *collection.Movie:
		return j.makeJFItemMovie(ctx, userID, i, parentID)
	case *collection.Show:
		return j.makeJFItemShow(ctx, userID, i, parentID)
	case *collection.Season:
		return j.makeJFItemSeason(ctx, userID, i, parentID)
	case *collection.Episode:
		return j.makeJFItemEpisode(ctx, userID, i, parentID)
	}
	log.Printf("makeJFItem: item %s has unknown type %T", item.ID(), item)
	return JFItem{}, fmt.Errorf("item %s unknown type %T", item.ID(), item)
}

// makeJFItem make movie item
func (j *Jellyfin) makeJFItemMovie(ctx context.Context, userID string, movie *collection.Movie, parentID string) (response JFItem, e error) {

	response = JFItem{
		Type:                    itemTypeMovie,
		ID:                      movie.ID(),
		ParentID:                makeJFCollectionID(parentID),
		ServerID:                j.serverID,
		Name:                    movie.Name(),
		OriginalTitle:           movie.Name(),
		SortName:                movie.SortName(),
		ForcedSortName:          movie.SortName(),
		Genres:                  movie.Metadata.Genres(),
		GenreItems:              makeJFGenreItems(movie.Metadata.Genres()),
		Studios:                 makeJFStudios(movie.Metadata.Studios()),
		IsHD:                    itemIsHD(movie),
		Is4K:                    itemIs4K(movie),
		RunTimeTicks:            makeRuntimeTicks(movie.Duration()),
		IsFolder:                false,
		LocationType:            "FileSystem",
		Path:                    "file.mp4",
		Etag:                    idhash.IdHash(movie.ID()),
		MediaType:               "Video",
		VideoType:               "VideoFile",
		Container:               "mov,mp4,m4a",
		DateCreated:             movie.Created().UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
		PlayAccess:              "Full",
		ImageTags: &JFImageTags{
			Primary:  movie.ID(),
			Backdrop: movie.ID(),
		},
		// Required to have Infuse load backdrop of episode
		BackdropImageTags: []string{movie.ID()},
		Width:             movie.VideoWidth(),
		Height:            movie.VideoHeight(),
		Overview:          movie.Metadata.Plot(),
		OfficialRating:    movie.Metadata.OfficialRating(),
		CommunityRating:   movie.Metadata.Rating(),
		ProductionYear:    movie.Metadata.Year(),
		ProviderIds:       makeJFProviderIds(movie.Metadata.ProviderIDs()),
		ChannelID:         nil,
		Chapters:          []JFChapter{},
		ExternalUrls:      []JFExternalUrls{},
		People:            j.makeJFPeople(ctx, movie.Metadata, userID),
		RemoteTrailers:    []JFRemoteTrailers{},
		Tags:              []string{},
		Taglines:          []string{movie.Metadata.Tagline()},
		Trickplay:         []string{},
		LockedFields:      []string{},
	}

	// Metadata might have a better title
	if movie.Metadata.Title() != "" {
		response.Name = movie.Metadata.Title()
	}

	// Set premiere date from metadata if available else from file timestamp
	if !movie.Metadata.Premiered().IsZero() {
		response.PremiereDate = movie.Metadata.Premiered().UTC()
	} else {
		response.PremiereDate = movie.Created().UTC()
	}

	// listview = true, movie carousel return both primary and BackdropImageTags
	// non-listview = false, remove primary (thumbnail) image reference
	// if !listView {
	// 	response.ImageTags = nil
	// }

	response.MediaSources = j.makeMediaSource(movie)
	response.MediaStreams = response.MediaSources[0].MediaStreams

	if playstate, err := j.repo.GetUserData(ctx, userID, movie.ID()); err == nil {
		response.UserData = j.makeJFUserData(userID, movie.ID(), playstate)
	} else {
		response.UserData = j.makeJFUserData(userID, movie.ID(), nil)
	}

	return response, nil
}

// makeJFItemShow makes show item
func (j *Jellyfin) makeJFItemShow(ctx context.Context, userID string, show *collection.Show, parentID string) (response JFItem, e error) {
	response = JFItem{
		Type:                    itemTypeShow,
		ID:                      show.ID(),
		ParentID:                makeJFCollectionID(parentID),
		ServerID:                j.serverID,
		Name:                    show.Name(),
		OriginalTitle:           show.Name(),
		SortName:                show.SortName(),
		ForcedSortName:          show.SortName(),
		Genres:                  show.Metadata.Genres(),
		GenreItems:              makeJFGenreItems(show.Metadata.Genres()),
		Studios:                 makeJFStudios(show.Metadata.Studios()),
		IsFolder:                true,
		Etag:                    idhash.IdHash(show.ID()),
		DateCreated:             show.FirstVideo().UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
		PlayAccess:              "Full",
		ImageTags: &JFImageTags{
			Primary:  show.ID(),
			Backdrop: show.ID(),
		},
		// Required to have Infuse load backdrop of episode
		BackdropImageTags: []string{
			show.ID(),
		},
		Overview:        show.Metadata.Plot(),
		OfficialRating:  show.Metadata.OfficialRating(),
		CommunityRating: show.Metadata.Rating(),
		ChannelID:       nil,
		Chapters:        []JFChapter{},
		ExternalUrls:    []JFExternalUrls{},
		People:          j.makeJFPeople(ctx, show.Metadata, userID),
		RemoteTrailers:  []JFRemoteTrailers{},
		Tags:            []string{},
		Taglines:        []string{show.Metadata.Tagline()},
		Trickplay:       []string{},
		LockedFields:    []string{},
	}

	// Show logo tends to be optional
	if show.Logo() != "" {
		response.ImageTags.Logo = show.ID()
	}

	// Metadata might have a better title
	if show.Metadata.Title() != "" {
		response.Name = show.Metadata.Title()
	}

	if show.Metadata.Year() != 0 {
		response.ProductionYear = show.Metadata.Year()
	}

	if !show.Metadata.Premiered().IsZero() {
		response.PremiereDate = show.Metadata.Premiered().UTC()
	} else {
		response.PremiereDate = show.FirstVideo().UTC()
	}

	// Get playstate of the show itself
	playstate, err := j.repo.GetUserData(ctx, userID, show.ID())
	if err != nil {
		playstate = &model.UserData{
			Timestamp: time.Now().UTC(),
		}
	}
	response.UserData = j.makeJFUserData(userID, show.ID(), playstate)

	// Set child count to number of seasons
	response.ChildCount = len(show.Seasons)
	// In case show does not have any seasons no need to calculate userdata
	if response.ChildCount == 0 {
		return response, nil
	}

	// Set recursive item count to number of episodes
	for _, s := range show.Seasons {
		response.RecursiveItemCount += len(s.Episodes)
	}

	// Calculate the number of episodes and played episode in the show
	var playedEpisodes, totalEpisodes int
	var lastestPlayed time.Time
	for _, s := range show.Seasons {
		for _, e := range s.Episodes {
			totalEpisodes++
			// Get playstate of episode
			episodePlaystate, err := j.repo.GetUserData(ctx, userID, e.ID())
			if err == nil && episodePlaystate != nil {
				if episodePlaystate.Played {
					playedEpisodes++
					if episodePlaystate.Timestamp.After(lastestPlayed) {
						lastestPlayed = episodePlaystate.Timestamp
					}
				}
			}
		}
	}

	// In case show has played episodes get playstate of the show itself
	if totalEpisodes != 0 {
		response.UserData.UnplayedItemCount = totalEpisodes - playedEpisodes
		response.UserData.PlayedPercentage = 100 * playedEpisodes / totalEpisodes
		response.UserData.LastPlayedDate = lastestPlayed
		response.UserData.Key = response.ID
		// Mark show as played when all episodes are played
		if playedEpisodes == totalEpisodes {
			response.UserData.Played = true
		}
	}
	return response, nil
}

// makeJFSeasonsOverview generates all season items for a show
func (j *Jellyfin) makeJFSeasonsOverview(ctx context.Context, userID string, show *collection.Show) ([]JFItem, error) {
	seasons := make([]JFItem, 0, len(show.Seasons))
	for _, s := range show.Seasons {
		if jfitem, err := j.makeJFItemSeason(ctx, userID, &s, show.ID()); err == nil {
			seasons = append(seasons, jfitem)
		}
	}

	// Always sort seasons by number, no user provided sortBy option.
	// This way season 99, Specials ends up last.
	sort.SliceStable(seasons, func(i, j int) bool {
		return seasons[i].IndexNumber < seasons[j].IndexNumber
	})

	return seasons, nil
}

// makeJFItemSeason makes a season item
func (j *Jellyfin) makeJFItemSeason(ctx context.Context, userID string, season *collection.Season, _ string) (response JFItem, err error) {
	_, show, season := j.collections.GetSeasonByID(season.ID())
	if season == nil {
		err = errors.New("could not find season")
		return
	}

	response = JFItem{
		Type:               itemTypeSeason,
		ID:                 makeJFSeasonID(season.ID()),
		SeriesID:           show.ID(),
		SeriesName:         show.Name(),
		ParentID:           show.ID(),
		ParentLogoItemId:   show.ID(),
		ServerID:           j.serverID,
		IsFolder:           true,
		LocationType:       "FileSystem",
		Etag:               idhash.IdHash(season.ID()),
		MediaType:          "Unknown",
		ChildCount:         len(season.Episodes),
		RecursiveItemCount: len(season.Episodes),
		DateCreated:        time.Now().UTC(),
		PremiereDate:       time.Now().UTC(),
		CanDelete:          false,
		CanDownload:        true,
		PlayAccess:         "Full",
		ImageTags: &JFImageTags{
			Primary: makeJFSeasonID(season.ID()),
		},
		ChannelID:      nil,
		Chapters:       []JFChapter{},
		ExternalUrls:   []JFExternalUrls{},
		People:         []JFPeople{},
		RemoteTrailers: []JFRemoteTrailers{},
		Tags:           []string{},
		Taglines:       []string{},
		Trickplay:      []string{},
		LockedFields:   []string{},
	}
	// Regular season? (>0)
	seasonNumber := season.Number()
	if seasonNumber != 0 {
		response.IndexNumber = seasonNumber
		response.Name = makeSeasonName(seasonNumber)
		response.SortName = fmt.Sprintf("%04d", seasonNumber)
	} else {
		// Specials tend to have season number 0, set season
		// number to 99 to make it sort at the end
		response.IndexNumber = 99
		response.Name = makeSeasonName(seasonNumber)
		response.SortName = "9999"
	}

	// Set season premiere date to first episode airdate if available
	if len(season.Episodes) != 0 {
		response.PremiereDate = season.Episodes[0].Metadata.Premiered()
	}

	// Get playstate of the season itself
	playstate, err := j.repo.GetUserData(ctx, userID, season.ID())
	if err != nil {
		playstate = &model.UserData{
			Timestamp: time.Now().UTC(),
		}
	}
	response.UserData = j.makeJFUserData(userID, season.ID(), playstate)

	// Calculate the number of played episodes in the season
	var playedEpisodes int
	var lastestPlayed time.Time
	for _, e := range season.Episodes {
		episodePlaystate, err := j.repo.GetUserData(ctx, userID, e.ID())
		if err == nil {
			if episodePlaystate.Played {
				playedEpisodes++
				if episodePlaystate.Timestamp.After(lastestPlayed) {
					lastestPlayed = episodePlaystate.Timestamp
				}
			}
		}
	}

	// Populate playstate fields with playstate of episodes in the season
	response.UserData.UnplayedItemCount = response.ChildCount - playedEpisodes
	response.UserData.PlayedPercentage = 100 * playedEpisodes / response.ChildCount
	response.UserData.LastPlayedDate = lastestPlayed
	// Mark season as played when all episodes are played
	if playedEpisodes == response.ChildCount {
		response.UserData.Played = true
	}

	return response, nil
}

func makeSeasonName(seasonNo int) string {
	// Regular season? (>0)
	if seasonNo != 0 {
		return fmt.Sprintf("Season %d", seasonNo)
	} else {
		return "Specials"
	}
}

// makeJFEpisodesOverview generates all episode items for one season of a show
func (j *Jellyfin) makeJFEpisodesOverview(ctx context.Context, userID string, season *collection.Season) ([]JFItem, error) {
	episodes := make([]JFItem, 0, len(season.Episodes))
	for _, e := range season.Episodes {
		if episode, err := j.makeJFItemEpisode(ctx, userID, &e, season.ID()); err == nil {
			episodes = append(episodes, episode)
		}
	}
	return episodes, nil
}

// makeJFItemEpisode makes an episode item
func (j *Jellyfin) makeJFItemEpisode(ctx context.Context, userID string, episode *collection.Episode, parentID string) (response JFItem, err error) {
	_, show, season, episode := j.collections.GetEpisodeByID(episode.ID())
	if episode == nil {
		err = errors.New("could not find episode")
		return
	}

	response = JFItem{
		Type:              itemTypeEpisode,
		ID:                makeJFEpisodeID(episode.ID()),
		SeasonID:          makeJFSeasonID(season.ID()),
		SeasonName:        makeSeasonName(season.Number()),
		SeriesID:          show.ID(),
		SeriesName:        show.Name(),
		ParentLogoItemId:  show.ID(),
		ServerID:          j.serverID,
		ParentIndexNumber: season.Number(),
		IndexNumber:       episode.Number(),
		Overview:          episode.Metadata.Plot(),
		IsHD:              itemIsHD(episode),
		Is4K:              itemIs4K(episode),
		RunTimeTicks:      makeRuntimeTicks(episode.Duration()),
		IsFolder:          false,
		LocationType:      "FileSystem",
		Path:              "episode.mp4",
		Etag:              idhash.IdHash(episode.ID()),
		MediaType:         "Video",
		VideoType:         "VideoFile",
		Container:         "mov,mp4,m4a",
		DateCreated:       episode.Created().UTC(),
		HasSubtitles:      true,
		CanDelete:         false,
		CanDownload:       true,
		PlayAccess:        "Full",
		Width:             episode.VideoWidth(),
		Height:            episode.VideoHeight(),
		ProductionYear:    episode.Metadata.Year(),
		CommunityRating:   episode.Metadata.Rating(),
		ProviderIds:       makeJFProviderIds(episode.Metadata.ProviderIDs()),
		ChannelID:         nil,
		Chapters:          []JFChapter{},
		ExternalUrls:      []JFExternalUrls{},
		People:            j.makeJFPeople(ctx, episode.Metadata, userID),
		RemoteTrailers:    []JFRemoteTrailers{},
		Tags:              []string{},
		Taglines:          []string{},
		Trickplay:         []string{},
		LockedFields:      []string{},
	}

	if episode.Poster() != "" {
		response.ImageTags = &JFImageTags{
			Primary: episode.ID(),
		}
	}

	// Get genres from episode, if not available use show genres
	genres := episode.Metadata.Genres()
	if len(genres) == 0 {
		genres = show.Metadata.Genres()
	}
	response.Genres = genres
	response.GenreItems = makeJFGenreItems(genres)

	// Get studios from episode, if not available use show studios
	studios := episode.Metadata.Studios()
	if len(studios) == 0 {
		studios = show.Metadata.Studios()
	}
	response.Studios = makeJFStudios(studios)

	// Metadata might have a better title
	if episode.Metadata.Title() != "" {
		response.Name = episode.Metadata.Title()
	}

	if !show.Metadata.Premiered().IsZero() {
		response.PremiereDate = show.Metadata.Premiered()
	} else {
		response.PremiereDate = episode.Created().UTC()
	}

	response.MediaSources = j.makeMediaSource(episode)
	response.MediaStreams = response.MediaSources[0].MediaStreams

	if playstate, err := j.repo.GetUserData(ctx, userID, episode.ID()); err == nil {
		response.UserData = j.makeJFUserData(userID, episode.ID(), playstate)
	} else {
		response.UserData = j.makeJFUserData(userID, episode.ID(), nil)
	}
	return response, nil
}

func (j *Jellyfin) makeJFItemGenre(_ context.Context, genre string) (response JFItem) {
	response = JFItem{
		ID:           makeJFGenreID(genre),
		ServerID:     j.serverID,
		Type:         itemTypeGenre,
		Name:         genre,
		SortName:     genre,
		Etag:         makeJFGenreID(genre),
		DateCreated:  time.Now().UTC(),
		PremiereDate: time.Now().UTC(),
		LocationType: "FileSystem",
		MediaType:    "Unknown",
		ChildCount:   1,
	}

	if genreItemCount := j.collections.GenreItemCount(); genreItemCount != nil {
		if genreCount, ok := genreItemCount[genre]; ok {
			response.ChildCount = genreCount
		}
	}

	return
}

// makeJFPeople creates a list of people (actors, directors, writers) for the item
func (j *Jellyfin) makeJFPeople(_ context.Context, m metadata.Metadata, userID string) []JFPeople {
	if userID != "XAOVn7iqiBujnIQY8sd0" {
		return []JFPeople{}
	}

	var people []JFPeople
	for name, role := range m.Actors() {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: role, Type: "Actor", PrimaryImageTag: id})
	}
	for _, name := range m.Directors() {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: "Director", Type: "Director", PrimaryImageTag: id})
	}
	for _, name := range m.Writers() {
		id := makeJFPersonID(name)
		people = append(people, JFPeople{ID: id, Name: name, Role: "Screenplay", Type: "Writer", PrimaryImageTag: id})
	}
	return people
}

func (j *Jellyfin) makeJFItemStudio(_ context.Context, studio string) JFItem {
	response := JFItem{
		ID:                makeJFStudioID(studio),
		ServerID:          j.serverID,
		Type:              itemTypeStudio,
		Name:              studio,
		SortName:          studio,
		Etag:              makeJFStudioID(studio),
		DateCreated:       time.Now().UTC(),
		PremiereDate:      time.Now().UTC(),
		LocationType:      "FileSystem",
		MediaType:         "Unknown",
		ImageBlurHashes:   &JFImageBlurHashes{},
		ImageTags:         &JFImageTags{},
		BackdropImageTags: []string{},
		UserData:          &JFUserData{},
	}
	return response
}

func makeJFStudios(studios []string) []JFStudios {
	var studioItems []JFStudios
	for _, studio := range studios {
		studioItems = append(studioItems, JFStudios{
			Name: studio, ID: makeJFStudioID(studio),
		})
	}
	return studioItems
}

func makeJFProviderIds(providerIDs map[string]string) JFProviderIds {
	ids := JFProviderIds{}
	for k, v := range providerIDs {
		switch strings.ToLower(k) {
		case "imdb":
			ids.Imdb = v
		case "themoviedb":
			ids.Tmdb = v
		case "tvdb":
			ids.Tvdb = v
		}
	}
	return ids
}

// makeJFUserData creates a JFUserData object, and populates from Userdata if provided
func (j *Jellyfin) makeJFUserData(UserID, itemID string, p *model.UserData) (response *JFUserData) {
	response = &JFUserData{
		Key:    UserID + "/" + itemID,
		ItemID: "00000000000000000000000000000000",
	}
	if p != nil {
		response.IsFavorite = p.Favorite
		response.LastPlayedDate = p.Timestamp
		response.PlaybackPositionTicks = p.Position * TicsToSeconds
		response.PlayedPercentage = p.PlayedPercentage
		response.Played = p.Played
	}

	return
}

func (j *Jellyfin) makeMediaSource(item collection.Item) (mediasources []JFMediaSources) {
	filename := item.FileName()
	mediasource := JFMediaSources{
		ID:                    item.ID(),
		ETag:                  idhash.IdHash(filename),
		Name:                  filename,
		Path:                  filename,
		Type:                  "Default",
		Container:             "mp4",
		Protocol:              "File",
		VideoType:             "VideoFile",
		Size:                  item.FileSize(),
		IsRemote:              false,
		ReadAtNativeFramerate: false,
		HasSegments:           false,
		IgnoreDts:             false,
		IgnoreIndex:           false,
		GenPtsInput:           false,
		// We do not support transcoding by server
		SupportsTranscoding:    false,
		SupportsDirectStream:   true,
		SupportsDirectPlay:     true,
		IsInfiniteStream:       false,
		RequiresOpening:        false,
		RequiresClosing:        false,
		RequiresLooping:        false,
		SupportsProbing:        true,
		TranscodingSubProtocol: "http",
		Formats:                []string{},
		MediaAttachments:       []JFMediaAttachments{},
		RunTimeTicks:           makeRuntimeTicks(item.Duration()),
		// File bitrate/s is sum of audio and video bitrate
		Bitrate:      item.VideoBitrate() + item.AudioBitrate(),
		MediaStreams: j.makeJFMediaStreams(item),
		// We assume audio stream is always at index 1 by makeJFMediaStreams()
		DefaultAudioStreamIndex: 1,
	}

	return []JFMediaSources{mediasource}
}

// makeJFMediaStreams creates media stream information for the provided item
func (j *Jellyfin) makeJFMediaStreams(item collection.Item) []JFMediaStreams {
	videostream := JFMediaStreams{
		Index:              0,
		Type:               "Video",
		IsDefault:          true,
		Language:           item.AudioLanguage(),
		AverageFrameRate:   item.VideoFrameRate(),
		RealFrameRate:      item.VideoFrameRate(),
		RefFrames:          1,
		TimeBase:           "1/16000",
		Height:             item.VideoHeight(),
		Width:              item.VideoWidth(),
		Codec:              item.VideoCodec(),
		AspectRatio:        "2.35:1",
		VideoRange:         "SDR",
		VideoRangeType:     "SDR",
		Profile:            "High",
		IsAnamorphic:       false,
		BitDepth:           8,
		BitRate:            item.VideoBitrate(),
		AudioSpatialFormat: "None",
	}
	switch strings.ToLower(item.VideoCodec()) {
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
	case "vc1":
		videostream.Codec = "vc1"
		videostream.CodecTag = "wvc1"
	default:
		videostream.Codec = "unknown"
		videostream.CodecTag = "unknown"
		log.Printf("Item %s/%s has unknown video codec %s", item.ID(), item.FileName(), item.VideoCodec())
	}
	videostream.Title = strings.ToUpper(videostream.Codec)
	videostream.DisplayTitle = videostream.Title + " - " + videostream.VideoRange

	audiostream := JFMediaStreams{
		Index:              1,
		Type:               "Audio",
		Language:           item.AudioLanguage(),
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
		Profile:            "LC",
		BitRate:            item.AudioBitrate(),
		Channels:           item.AudioChannels(),
	}

	switch audiostream.Channels {
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
	case 8:
		audiostream.Title = "7.1 Channel"
		audiostream.ChannelLayout = "7.1"
	default:
		audiostream.Title = "Unknown"
		audiostream.ChannelLayout = "unknown"
		log.Printf("Item %s/%s has unknown audio channel configuration %d", item.ID(), item.FileName(), audiostream.Channels)
	}

	switch strings.ToLower(item.AudioCodec()) {
	case "ac3":
		audiostream.Codec = "ac3"
		audiostream.CodecTag = "ac-3"
	case "aac":
		audiostream.Codec = "aac"
		audiostream.CodecTag = "mp4a"
	case "wma":
		audiostream.Codec = "wmapro"
	default:
		audiostream.Codec = "unknown"
		log.Printf("Item %s/%s has unknown audio codec %s", item.ID(), item.FileName(), item.AudioCodec())
	}

	audiostream.DisplayTitle = audiostream.Title + " - " + strings.ToUpper(audiostream.Codec)

	return []JFMediaStreams{videostream, audiostream}
}

// makeRuntimeTicks converts a time.Duration to Jellyfin runtime ticks
func makeRuntimeTicks(d time.Duration) int64 {
	return int64(d.Microseconds() * 10)
}

// Most internal IDs get a prefixed when used in an API response. This helps
// to determine the type of response when receiving an ID from
// a client on an endpoints like /Items/{id}.
//
// Movies and shows do not get a prefix for backwards compatibility reasons.

const (
	itemprefix_separator            = "_"
	itemprefix_root                 = "root_"
	itemprefix_collection           = "collection_"
	itemprefix_collection_favorites = "collectionfavorites_"
	itemprefix_collection_playlist  = "collectionplaylist_"
	itemprefix_show                 = "show_"
	itemprefix_season               = "season_"
	itemprefix_episode              = "episode_"
	itemprefix_playlist             = "playlist_"
	itemprefix_genre                = "genre_"
	itemprefix_studio               = "studio_"
	itemprefix_person               = "person_"
	itemprefix_displaypreferences   = "dp_"
)

// makeJFRootID returns an external id for the root folder.
func makeJFRootID(rootID string) string {
	return itemprefix_root + rootID
}

// makeJFCollectionID returns an external id for a collection.
func makeJFCollectionID(collectionID string) string {
	return itemprefix_collection + collectionID
}

// makeJFCollectionFavoritesID returns an external id for a favorites collection.
func makeJFCollectionFavoritesID(favoritesID string) string {
	return itemprefix_collection_favorites + favoritesID
}

// makeJFCollectionPlaylistID returns an external id for a playlist collection.
func makeJFCollectionPlaylistID(playlistCollectionID string) string {
	return itemprefix_collection_playlist + playlistCollectionID
}

// makeJFPlaylistID returns an external id for a playlist.
func makeJFPlaylistID(playlistID string) string {
	return itemprefix_playlist + playlistID
}

// makeJFSeasonID returns an external id for a season ID.
func makeJFSeasonID(seasonID string) string {
	return itemprefix_season + seasonID
}

// makeJFEpisodeID returns an external id for an episode ID.
func makeJFEpisodeID(episodeID string) string {
	return itemprefix_episode + episodeID
}

// makeJFDisplayPreferencesID returns an external id for display preferences.
func makeJFDisplayPreferencesID(dpID string) string {
	return itemprefix_displaypreferences + dpID
}

// makeJFGenreID returns an external id for a genre name.
func makeJFGenreID(genre string) string {
	return itemprefix_genre + idhash.IdHash(genre)
}

// makeJFStudioID returns an external id for a studio.
func makeJFStudioID(studio string) string {
	return itemprefix_studio + idhash.IdHash(studio)
}

// makeJFPersonID returns an external id for a person.
func makeJFPersonID(name string) string {
	// Name is base64 encoded to handle special characters in names.
	// Regular URL encoding is not used as some clients have issues with % characters in IDs.
	return itemprefix_person + base64.RawURLEncoding.EncodeToString([]byte(strings.ToLower(name)))
}

// decodeJFPersonID decodes a person ID to get the original name.
func decodeJFPersonID(id string) (string, error) {
	nameBytes, err := base64.RawURLEncoding.DecodeString(trimPrefix(id))
	if err != nil {
		return "", errors.New("invalid person ID")
	}
	return string(nameBytes), nil
}

// trimPrefix removes the type prefix from an item id.
func trimPrefix(s string) string {
	if i := strings.Index(s, itemprefix_separator); i != -1 {
		return s[i+1:]
	}
	return s
}

// isJFRootID checks if the provided ID is a root ID.
func isJFRootID(id string) bool {
	return strings.HasPrefix(id, itemprefix_root)
}

// isJFCollectionID checks if the provided ID is a collection ID.
func isJFCollectionID(id string) bool {
	return strings.HasPrefix(id, itemprefix_collection)
}

// isJFCollectionFavoritesID checks if the provided ID is a favorites collection ID.
func isJFCollectionFavoritesID(id string) bool {
	return strings.HasPrefix(id, itemprefix_collection_favorites)
}

// isJFCollectionPlaylistID checks if the provided ID is the playlist collection ID.
func isJFCollectionPlaylistID(id string) bool {
	// There is only one playlist collection id, so we can do a direct comparison
	return id == makeJFCollectionPlaylistID(playlistCollectionID)
}

// isJFPlaylistID checks if the provided ID is a playlist ID.
func isJFPlaylistID(id string) bool {
	return strings.HasPrefix(id, itemprefix_playlist)
}

// isJFShowID checks if the provided ID is a show ID.
func isJFShowID(id string) bool {
	return strings.HasPrefix(id, itemprefix_show)
}

// isJFSeasonID checks if the provided ID is a season ID.
func isJFSeasonID(id string) bool {
	return strings.HasPrefix(id, itemprefix_season)
}

// isJFEpisodeID checks if the provided ID is an episode ID.
func isJFEpisodeID(id string) bool {
	return strings.HasPrefix(id, itemprefix_episode)
}

// isJFGenreID checks if the provided ID is a genre ID.
func isJFGenreID(id string) bool {
	return strings.HasPrefix(id, itemprefix_genre)
}

// isJFStudioID checks if the provided ID is a studio ID.
func isJFStudioID(id string) bool {
	return strings.HasPrefix(id, itemprefix_studio)
}

// isJFPersonID checks if the provided ID is a person ID.
func isJFPersonID(id string) bool {
	return strings.HasPrefix(id, itemprefix_person)
}

// itemIsHD checks if the provided item is HD (720p or higher)
func itemIsHD(item collection.Item) bool {
	return item.VideoHeight() >= 720
}

// itemIs4K checks if the provided item is 4K (2160p or higher)
func itemIs4K(item collection.Item) bool {
	return item.VideoHeight() >= 1500
}
