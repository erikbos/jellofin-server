// contains common and shared item functions
package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/jxskiss/base62"

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
	itemTypeMusicAlbum       = "MusicAlbum"
	itemTypeAudio            = "Audio"

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
	if _, show := j.collections.GetShowByID(trimPrefix(parentID)); show != nil {
		if items, err := j.makeJFSeasonsOverview(ctx, userID, show); err != nil {
			return items, nil
		}
		return []JFItem{}, errors.New("could not get seasons overview for show")
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
		return j.makeJFItemCollection(ctx, trimPrefix(itemID))
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
		ETag:                  idhash.Hash(filename),
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
		// log.Printf("Item %s/%s has unknown video codec %s", item.ID(), item.FileName(), item.VideoCodec())
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
		// log.Printf("Item %s/%s has unknown audio channel configuration %d", item.ID(), item.FileName(), audiostream.Channels)
	}

	switch strings.ToLower(item.AudioCodec()) {
	case "ac3":
		audiostream.Codec = "ac3"
		audiostream.CodecTag = "ac-3"
	case "aac":
		audiostream.Codec = "aac"
		audiostream.CodecTag = "mp4a"
	case "eac3":
		audiostream.Codec = "eac3"
		audiostream.CodecTag = "ec-3"
	case "wma":
		audiostream.Codec = "wmapro"
	default:
		audiostream.Codec = "unknown"
		// log.Printf("Item %s/%s has unknown audio codec %s", item.ID(), item.FileName(), item.AudioCodec())
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
	itemprefix_user                 = "u_"
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

// trimPrefix removes the type prefix from an item id.
func trimPrefix(s string) string {
	if i := strings.Index(s, itemprefix_separator); i != -1 {
		return s[i+1:]
	}
	return s
}

// encodeExternalName encodes a name with a prefix to create an external ID.
// the prefix helps to determine the type of ID when receiving it from a client.
// the name is base62 encoded to avoid special characters in the ID.
func encodeExternalName(itemprefix, name string) string {
	return itemprefix + base62.StdEncoding.EncodeToString([]byte(name))
}

// decodeExternalName decodes an external ID to get the original name.
func decodeExternalName(itemprefix, id string) (string, error) {
	if !strings.HasPrefix(id, itemprefix) {
		return "", errors.New("invalid id")
	}
	b, err := base62.StdEncoding.DecodeString(trimPrefix(id))
	if err != nil {
		return "", errors.New("cannot decode id")
	}
	return string(b), nil
}

// itemIsHD checks if the provided item is HD (720p or higher)
func itemIsHD(item collection.Item) bool {
	return item.VideoHeight() >= 720
}

// itemIs4K checks if the provided item is 4K (2160p or higher)
func itemIs4K(item collection.Item) bool {
	return item.VideoHeight() >= 1500
}
