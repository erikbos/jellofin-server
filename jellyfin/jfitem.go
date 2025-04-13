package jellyfin

import (
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/miquels/notflix-server/collection"
	"github.com/miquels/notflix-server/database"
	"github.com/miquels/notflix-server/idhash"
	"github.com/miquels/notflix-server/nfo"
)

func (j *Jellyfin) makeJItemCollection(itemid string) (response JFItem, e error) {
	collectionid := strings.TrimPrefix(itemid, itemprefix_collection)
	c := j.collections.GetCollection(collectionid)
	if c == nil {
		e = errors.New("collection not found")
		return
	}
	details := c.Details()

	itemID := itemprefix_collection + collectionid
	response = JFItem{
		Name:                     c.Name_,
		ServerID:                 serverID,
		ID:                       itemID,
		Etag:                     idhash.IdHash(itemID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
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
		Genres:                   details.Genres,
		GenreItems:               arrayToGenreItems(details.Genres),
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	switch c.Type {
	case collection.CollectionMovies:
		response.CollectionType = CollectionMovies
	case collection.CollectionShows:
		response.CollectionType = CollectionTVShows
	}
	response.SortName = response.CollectionType
	return
}

func (j *Jellyfin) makeJFItemCollectionFavorites(userid string) (response JFItem, e error) {
	favoriteIDs, err := j.db.UserDataRepo.GetFavorites(userid)

	// In case of no favorites, we still want to return a collection item
	var itemCount int
	if err == nil {
		itemCount = len(favoriteIDs)
	}
	id := itemprefix_collection_favorites + favoritesCollectionID

	response = JFItem{
		Name:                     "Favorites",
		ServerID:                 serverID,
		ID:                       id,
		Etag:                     idhash.IdHash(id),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           CollectionPlaylists,
		SortName:                 CollectionPlaylists,
		Type:                     "UserView",
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               itemCount,
		DisplayPreferencesID:     displayPreferencesID,
		ExternalUrls:             []JFExternalUrls{},
		PlayAccess:               "Full",
		PrimaryImageAspectRatio:  1.7777777777777777,
		RemoteTrailers:           []JFRemoteTrailers{},
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		ParentID:                 collectionRootID,
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		// PremiereDate should be set based upon most recent item in collection
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	return
}

func (j *Jellyfin) makeJFItemCollectionPlaylist(userid string) (response JFItem, e error) {
	playlistIDs, err := j.db.PlaylistRepo.GetPlaylists(userid)

	// In case of no playlists, we still want to return a collection item
	var itemCount int
	if err == nil {
		itemCount = len(playlistIDs)
	}
	id := itemprefix_collection_playlist + playlistCollectionID

	response = JFItem{
		Name:                     "Playlists",
		ServerID:                 serverID,
		ID:                       id,
		Etag:                     idhash.IdHash(id),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           CollectionPlaylists,
		SortName:                 CollectionPlaylists,
		Type:                     "UserView",
		IsFolder:                 true,
		EnableMediaSourceDisplay: true,
		ChildCount:               itemCount,
		DisplayPreferencesID:     displayPreferencesID,
		ExternalUrls:             []JFExternalUrls{},
		PlayAccess:               "Full",
		PrimaryImageAspectRatio:  1.7777777777777777,
		RemoteTrailers:           []JFRemoteTrailers{},
		LocationType:             "FileSystem",
		Path:                     "/collection",
		LockData:                 false,
		MediaType:                "Unknown",
		ParentID:                 collectionRootID,
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		// PremiereDate should be set based upon most recent item in collection
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	return
}

// makeJFItem make movie or show from provided item
func (j *Jellyfin) makeJFItem(userID string, i *collection.Item, parentId, collectionType string, listView bool) (response JFItem) {
	response = JFItem{
		ID:                      i.ID,
		ParentID:                parentId,
		ServerID:                serverID,
		Name:                    i.Name,
		OriginalTitle:           i.Name,
		SortName:                i.Name,
		ForcedSortName:          i.Name,
		Etag:                    idhash.IdHash(i.ID),
		DateCreated:             time.Unix(i.FirstVideo/1000, 0).UTC(),
		PremiereDate:            time.Unix(i.FirstVideo/1000, 0).UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
		PlayAccess:              "Full",
	}

	response.ImageTags = &JFImageTags{
		Primary: "primary_" + i.ID,
	}

	// Required to have Infuse load backdrop of episode
	response.BackdropImageTags = []string{
		response.ID,
	}

	if collectionType == collection.CollectionMovies {
		response.Type = "Movie"
		response.IsFolder = false
		response.LocationType = "FileSystem"
		response.Path = "file.mp4"
		response.MediaType = "Video"
		response.VideoType = "VideoFile"
		response.Container = "mov,mp4,m4a"

		i.LoadNfo()
		// fix: me, this should come from collection package
		response.MediaSources = j.makeMediaSource(i.Video, i.Nfo)
		response.RunTimeTicks = response.MediaSources[0].RunTimeTicks
		response.MediaStreams = response.MediaSources[0].MediaStreams

		// listview = true, movie carousel return both primary and BackdropImageTags
		// non-listview = false, remove primary (thumbnail) image reference
		if !listView {
			response.ImageTags = nil
		}
	}

	if collectionType == collection.CollectionShows {
		response.Type = "Series"
		response.IsFolder = true
		response.ChildCount = len(i.Seasons)

		var playedEpisodes, totalEpisodes int
		var lastestPlayed time.Time
		for _, s := range i.Seasons {
			for _, e := range s.Episodes {
				totalEpisodes++
				episodePlaystate, err := j.db.UserDataRepo.Get(userID, e.ID)
				if err == nil {
					if episodePlaystate.Played {
						playedEpisodes++
						if episodePlaystate.Timestamp.After(lastestPlayed) {
							lastestPlayed = episodePlaystate.Timestamp
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

	j.enrichResponseWithNFO(&response, i.Nfo)

	if playstate, err := j.db.UserDataRepo.Get(userID, i.ID); err == nil {
		response.UserData = j.makeJFUserData(userID, i.ID, playstate)
	}
	return response
}

// makeJFItemSeason makes a season
func (j *Jellyfin) makeJFItemSeason(userID, seasonID string) (response JFItem, err error) {
	_, show, season := j.collections.GetSeasonByID(trimPrefix(seasonID))
	if season == nil {
		err = errors.New("could not find season")
		return
	}

	response = JFItem{
		Type:               "Season",
		ServerID:           serverID,
		ParentID:           show.ID,
		SeriesID:           show.ID,
		ID:                 itemprefix_season + seasonID,
		Etag:               idhash.IdHash(seasonID),
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
		PlayAccess:         "Full",
		ImageTags: &JFImageTags{
			Primary: "season",
		},
	}

	var playedEpisodes int
	var lastestPlayed time.Time
	for _, e := range season.Episodes {
		episodePlaystate, err := j.db.UserDataRepo.Get(userID, e.ID)
		if err == nil {
			if episodePlaystate.Played {
				playedEpisodes++
				if episodePlaystate.Timestamp.After(lastestPlayed) {
					lastestPlayed = episodePlaystate.Timestamp
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

// makeJFItemEpisode makes an episode
func (j *Jellyfin) makeJFItemEpisode(userID, episodeID string) (response JFItem, err error) {
	_, show, _, episode := j.collections.GetEpisodeByID(trimPrefix(episodeID))
	if episode == nil {
		err = errors.New("could not find episode")
		return
	}

	response = JFItem{
		Type:         "Episode",
		ID:           episodeID,
		Etag:         idhash.IdHash(episodeID),
		ServerID:     serverID,
		SeriesName:   show.Name,
		SeriesID:     idhash.IdHash(show.Name),
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
		PlayAccess:   "Full",
		ImageTags: &JFImageTags{
			Primary: "episode",
		},
	}

	// Get a bunch of metadata from show-level nfo
	show.LoadNfo()
	if show.Nfo != nil {
		j.enrichResponseWithNFO(&response, show.Nfo)
	}

	// Remove ratings as we do not want ratings from series apply to an episode
	response.OfficialRating = ""
	response.CommunityRating = 0

	// Enrich and override metadata using episode nfo, if available, as it is more specific than data from show
	episode.LoadNfo()
	if episode.Nfo != nil {
		j.enrichResponseWithNFO(&response, episode.Nfo)
	}

	// Add some generic mediasource to indicate "720p, stereo"
	response.MediaSources = j.makeMediaSource(episode.Video, episode.Nfo)
	response.RunTimeTicks = response.MediaSources[0].RunTimeTicks
	response.MediaStreams = response.MediaSources[0].MediaStreams

	if playstate, err := j.db.UserDataRepo.Get(userID, episodeID); err == nil {
		response.UserData = j.makeJFUserData(userID, episodeID, playstate)
	}
	return response, nil
}

// makeJFItemFavoritesOverview creates a list of favorite items
func (j *Jellyfin) makeJFItemFavoritesOverview(userID string) (items []JFItem, err error) {
	favoriteIDs, err := j.db.UserDataRepo.GetFavorites(userID)

	// log.Printf("makeJFItemFavoritesOverview: %+v, %+v", favoriteIDs, err)
	if err != nil {
		return
	}

	for _, itemID := range favoriteIDs {
		c, i := j.collections.GetItemByID(itemID)
		if i != nil {
			item := j.makeJFItem(userID, i, genCollectionID(c.ID), c.Type, false)
			items = append(items, item)
		}
	}
	return
}

func (j *Jellyfin) makeJFItemPlaylistOverview(userId string) (items []JFItem, err error) {
	playlistIDs, err := j.db.PlaylistRepo.GetPlaylists(userId)

	log.Printf("makeJFItemPlaylistOverview: %+v, %+v", playlistIDs, err)
	if err != nil {
		return
	}
	// In case we have playlists populate, otherwise leave empty list
	for _, playlistID := range playlistIDs {
		item, err := j.makeJFItemPlaylist(userId, playlistID)
		if err == nil {
			items = append(items, item)
		}
	}
	return
}

func (j *Jellyfin) makeJFItemPlaylist(userID, playlistID string) (response JFItem, err error) {
	playlist, err := j.db.PlaylistRepo.GetPlaylist(trimPrefix(playlistID))
	if playlist == nil {
		err = errors.New("could not find playlist")
		return
	}

	id := itemprefix_playlist + playlist.ID
	response = JFItem{
		Type:                     "Playlist",
		ID:                       id,
		ServerID:                 serverID,
		ParentID:                 "1071671e7bffa0532e930debee501d2e", // fixme: this should be ID generated by makeJFItemCollectionPlaylist()
		Name:                     playlist.Name,
		SortName:                 playlist.Name,
		Etag:                     idhash.IdHash(id),
		DateCreated:              time.Now().UTC(),
		CanDelete:                true,
		CanDownload:              true,
		Path:                     "/playlist",
		IsFolder:                 true,
		PlayAccess:               "Full",
		RecursiveItemCount:       len(playlist.ItemIDs),
		ChildCount:               len(playlist.ItemIDs),
		LocationType:             "FileSystem",
		MediaType:                "Video",
		DisplayPreferencesID:     displayPreferencesID,
		EnableMediaSourceDisplay: true,
	}

	return
}

// makeJFUserData creates a JFUserData object from PlayState
func (j *Jellyfin) makeJFUserData(UserID, itemID string, p database.UserData) (response *JFUserData) {
	response = &JFUserData{
		PlaybackPositionTicks: p.Position * TicsToSeconds,
		PlayedPercentage:      p.PlayedPercentage,
		Played:                p.Played,
		IsFavorite:            p.Favorite,
		LastPlayedDate:        p.Timestamp,
		Key:                   UserID + "/" + itemID,
		ItemID:                "00000000000000000000000000000000",
	}
	return
}

func (j *Jellyfin) enrichResponseWithNFO(response *JFItem, n *nfo.Nfo) {
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
		normalizedGenres := nfo.NormalizeGenres(n.Genre)
		// Why do we populate two response fields with same data?
		response.Genres = normalizedGenres
		for _, genre := range normalizedGenres {
			g := JFGenreItem{
				Name: genre,
				ID:   idhash.IdHash(genre),
			}
			response.GenreItems = append(response.GenreItems, g)
		}
	}

	if n.Studio != "" {
		response.Studios = []JFStudios{
			{
				Name: n.Studio,
				ID:   idhash.IdHash(n.Studio),
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
	// 			ID:   idhash.IdHash(actor.Name),
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

func (j *Jellyfin) makeMediaSource(filename string, n *nfo.Nfo) (mediasources []JFMediaSources) {
	mediasource := JFMediaSources{
		ID:                    idhash.IdHash(filename),
		ETag:                  idhash.IdHash(filename),
		Name:                  filename,
		Path:                  filename,
		Type:                  "Default",
		Container:             "mp4",
		Protocol:              "File",
		VideoType:             "VideoFile",
		Size:                  4264940672,
		IsRemote:              false,
		ReadAtNativeFramerate: false,
		HasSegments:           false,
		IgnoreDts:             false,
		IgnoreIndex:           false,
		GenPtsInput:           false,
		// We do not support transcoding by server
		SupportsTranscoding:  false,
		SupportsDirectStream: true,
		SupportsDirectPlay:   true,
		IsInfiniteStream:     false,
		RequiresOpening:      false,
		RequiresClosing:      false,
		RequiresLooping:      false,
		SupportsProbing:      true,
		Formats:              []string{},
	}

	// log.Printf("makeMediaSource: n: %+v, n2: %+v, n3: %+v\n", n, n.FileInfo, n.FileInfo.StreamDetails)
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
		AspectRatio:      "2.35:1",
		VideoRange:       "SDR",
		VideoRangeType:   "SDR",
		IsAnamorphic:     false,
		BitDepth:         8,
		BitRate:          5193152,
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
	case "vc1":
		videostream.Codec = "vc1"
		videostream.CodecTag = "wvc1"
	default:
		log.Printf("Nfo of %s has unknown video codec %s", filename, NfoVideo.Codec)
	}
	videostream.Title = strings.ToUpper(videostream.Codec)
	videostream.DisplayTitle = videostream.Title + " - " + videostream.VideoRange

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
	case 8:
		audiostream.Title = "7.1 Channel"
		audiostream.ChannelLayout = "7.1"
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
	case "wma":
		audiostream.Codec = "wmapro"
	default:
		log.Printf("Nfo of %s has unknown audio codec %s", filename, NfoAudio.Codec)
	}

	audiostream.DisplayTitle = audiostream.Title + " - " + strings.ToUpper(audiostream.Codec)

	mediasource.MediaStreams = append(mediasource.MediaStreams, audiostream)

	return []JFMediaSources{mediasource}
}

func genCollectionID(id int) string {
	return itemprefix_collection + fmt.Sprintf("%d", id)
}
