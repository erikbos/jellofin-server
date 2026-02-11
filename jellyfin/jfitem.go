// contains common and shared item functions
package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection"
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

// getJFItems returns list of items based on provided parentID or all items if parentID is empty
func (j *Jellyfin) getJFItems(ctx context.Context, userID, parentID string) ([]JFItem, error) {
	if parentID != "" {
		return j.getJFItemsByParentID(ctx, userID, parentID)
	} else {
		return j.getJFItemsAll(ctx, userID)
	}
}

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
	case isJFPersonID(parentID):
		items, err := j.getJFItemsAll(ctx, userID)
		if err != nil {
			return []JFItem{}, errors.New("could not get all items")
		}
		// Make a new list with only items of which a personid matches provided parentID
		personItems := make([]JFItem, 0, len(items))
		for _, item := range items {
			for _, person := range item.People {
				if person.ID == parentID {
					personItems = append(personItems, item)
				}
			}
		}
		return personItems, nil

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

	case isJFSeasonID(parentID):
		_, i := j.collections.GetItemByID(trimPrefix(parentID))
		if i == nil {
			return []JFItem{}, errors.New("could not find season")
		}
		if show, ok := i.(*collection.Season); ok {
			items, err := j.makeJFEpisodesOverview(ctx, userID, show)
			if err != nil {
				return []JFItem{}, errors.New("could not find season")
			}
			return items, nil
		}
	}

	// Check if parentID is a show to generate overviews
	if _, i := j.collections.GetItemByID(trimPrefix(parentID)); i != nil {
		if show, ok := i.(*collection.Show); ok {
			items, err := j.makeJFSeasonsOverview(ctx, userID, show)
			if err != nil {
				return []JFItem{}, errors.New("could not find parent show")
			}
			return items, nil
		}
		log.Printf("getJFItemsByParentID: unsupported parentID %s of type %T \n", i.ID(), i)
		return []JFItem{}, errors.New("unable to generate items for parentID")
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

// GetAllPersonNames returns a list of all person names across all collections
func (j *Jellyfin) GetAllPersonNames(ctx context.Context) ([]string, error) {
	personNames := make(map[string]struct{})
	for _, c := range j.collections.GetCollections() {
		for _, i := range c.Items {
			for _, p := range i.Actors() {
				personNames[p] = struct{}{}
			}
			for _, p := range i.Directors() {
				personNames[p] = struct{}{}
			}
			for _, p := range i.Writers() {
				personNames[p] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(personNames))
	for name := range personNames {
		names = append(names, name)
	}
	return names, nil
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
		for _, i := range c.Items {
			for _, genre := range i.Genres() {
				if !slices.Contains(collectionGenres, genre) {
					collectionGenres = append(collectionGenres, genre)
				}
			}
		}
	}

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
		// ImageTags: &JFImageTags{
		// 	Primary: collectionRootID,
		// },
	}
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

// makeJFItemByIDs creates a list of items based on the provided itemIDs
func (j *Jellyfin) makeJFItemByIDs(ctx context.Context, userID string, itemIDs []string) ([]JFItem, error) {
	items := make([]JFItem, 0, len(itemIDs))
	for _, itemID := range itemIDs {
		if item, err := j.makeJFItemByID(ctx, userID, itemID); err == nil {
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
		return j.makeJFItemPerson(ctx, userID, itemID)
	case isJFGenreID(itemID):
		return j.makeJFItemGenre(ctx, userID, itemID)
	case isJFStudioID(itemID):
		return j.makeJFItemStudio(ctx, userID, itemID)
	}

	// Try to fetch individual item: movie, show, episode
	c, i := j.collections.GetItemByID(trimPrefix(itemID))
	if i == nil {
		return JFItem{}, errors.New("item not found")
	}
	return j.makeJFItem(ctx, userID, i, c.ID)
}

// makeJFItem make movie or show from provided item
func (j *Jellyfin) makeJFItem(ctx context.Context, userID string, item collection.Item, parentID string) (JFItem, error) {
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

// makeJFCollectionFavoritesID returns an external id for a favorites collection.
func makeJFCollectionFavoritesID(favoritesID string) string {
	return itemprefix_collection_favorites + favoritesID
}

// makeJFCollectionPlaylistID returns an external id for a playlist collection.
func makeJFCollectionPlaylistID(playlistCollectionID string) string {
	return itemprefix_collection_playlist + playlistCollectionID
}

// isJFCollectionPlaylistID checks if the provided ID is the playlist collection ID.
func isJFCollectionPlaylistID(id string) bool {
	// There is only one playlist collection id, so we can do a direct comparison
	return id == makeJFCollectionPlaylistID(playlistCollectionID)
}

// makeJFDisplayPreferencesID returns an external id for display preferences.
func makeJFDisplayPreferencesID(dpID string) string {
	return itemprefix_displaypreferences + dpID
}

// trimPrefix removes the type prefix from an item id.
func trimPrefix(s string) string {
	if i := strings.Index(s, itemprefix_separator); i != -1 {
		return s[i+1:]
	}
	return s
}

// isJFCollectionFavoritesID checks if the provided ID is a favorites collection ID.
func isJFCollectionFavoritesID(id string) bool {
	return strings.HasPrefix(id, itemprefix_collection_favorites)
}

// itemIsHD checks if the provided item is HD (720p or higher)
func itemIsHD(item collection.Item) bool {
	return item.VideoHeight() >= 720
}

// itemIs4K checks if the provided item is 4K (2160p or higher)
func itemIs4K(item collection.Item) bool {
	return item.VideoHeight() >= 1500
}
