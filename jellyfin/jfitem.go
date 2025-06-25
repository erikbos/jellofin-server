package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database"
	"github.com/erikbos/jellofin-server/idhash"
)

const (
	// Misc IDs for api responses
	serverID                = "2b11644442754f02a0c1e45d2a9f5c71"
	sessionID               = "e3a869b7a901f8894de8ee65688db6c0"
	collectionRootID        = "e9d5075a555c1cbc394eec4cef295274"
	playlistCollectionID    = "2f0340563593c4d98b97c9bfa21ce23c"
	favoritesCollectionID   = "f4a0b1c2d3e5c4b8a9e6f7d8e9a0b1c2"
	displayPreferencesID    = "f137a2dd21bbc1b99aa5c0f6bf02a805"
	collectionTypeMovies    = "movies"
	collectionTypeTVShows   = "tvshows"
	CollectionTypePlaylists = "playlists"

	// itemid prefixes
	itemprefix_separator            = "_"
	itemprefix_root                 = "root_"
	itemprefix_collection           = "collection_"
	itemprefix_collection_favorites = "collectionfavorites_"
	itemprefix_collection_playlist  = "collectionplaylist_"
	itemprefix_show                 = "show_"
	itemprefix_season               = "season_"
	itemprefix_episode              = "episode_"
	itemprefix_playlist             = "playlist_"

	// imagetag prefix will get HTTP-redirected
	tagprefix_redirect = "redirect_"
	// imagetag prefix means we will serve the filename from local disk
	tagprefix_file = "file_"
)

func (j *Jellyfin) makeJFItemRoot() (response JFItem, e error) {
	childCount := len(j.collections.GetCollections())
	// we add the favorites and playlist collections to the child count
	childCount += 2

	genres := j.collections.Details().Genres

	response = JFItem{
		Name:                     "Media Folders",
		ServerID:                 serverID,
		ID:                       itemprefix_root + collectionRootID,
		Etag:                     idhash.IdHash(collectionRootID),
		DateCreated:              time.Now().UTC(),
		Type:                     "UserRootFolder",
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
		DisplayPreferencesID:     displayPreferencesID,
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

func (j *Jellyfin) makeJItemCollection(collectionID string) (response JFItem, e error) {
	// collectionid := strings.TrimPrefix(itemid, itemprefix_collection)
	// fixme: after collectionID is a real ID and not an int, this should be removed
	c := j.collections.GetCollection(strings.TrimPrefix(collectionID, itemprefix_collection))
	if c == nil {
		e = errors.New("collection not found")
		return
	}
	collectionGenres := c.Details().Genres

	response = JFItem{
		Name:                     c.Name_,
		ServerID:                 serverID,
		ID:                       itemprefix_collection + collectionID,
		ParentID:                 itemprefix_root + collectionRootID,
		Etag:                     idhash.IdHash(collectionID),
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
		CanDelete:                false,
		CanDownload:              true,
		SpecialFeatureCount:      0,
		Genres:                   collectionGenres,
		GenreItems:               makeJFGenreItems(collectionGenres),
		// TODO: we do not support images for a collection
		// ImageTags: &JFImageTags{
		// 	Primary: "collection",
		// },
	}
	switch c.Type {
	case collection.CollectionMovies:
		response.CollectionType = collectionTypeMovies
	case collection.CollectionShows:
		response.CollectionType = collectionTypeTVShows
	default:
		log.Printf("makeJItemCollection: unknown collection type: %s", c.Type)
	}
	response.SortName = response.CollectionType
	return
}

func (j *Jellyfin) makeJFItemCollectionFavorites(ctx context.Context, userID string) (response JFItem, e error) {
	var itemCount int
	if favoriteIDs, err := j.db.UserDataRepo.GetFavorites(ctx, userID); err == nil {
		itemCount = len(favoriteIDs)
	}

	response = JFItem{
		Name:                     "Favorites",
		ServerID:                 serverID,
		ID:                       itemprefix_collection_favorites + favoritesCollectionID,
		Etag:                     idhash.IdHash(favoritesCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           CollectionTypePlaylists,
		SortName:                 CollectionTypePlaylists,
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

func (j *Jellyfin) makeJFItemCollectionPlaylist(ctx context.Context, userID string) (response JFItem, e error) {
	playlistIDs, err := j.db.PlaylistRepo.GetPlaylists(ctx, userID)

	// In case of no playlists, we still want to return a collection item
	var itemCount int
	if err == nil {
		itemCount = len(playlistIDs)
	}

	response = JFItem{
		Name:                     "Playlists",
		ServerID:                 serverID,
		ID:                       itemprefix_collection_playlist + playlistCollectionID,
		Etag:                     idhash.IdHash(playlistCollectionID),
		DateCreated:              time.Now().UTC(),
		PremiereDate:             time.Now().UTC(),
		CollectionType:           CollectionTypePlaylists,
		SortName:                 CollectionTypePlaylists,
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
func (j *Jellyfin) makeJFItem(ctx context.Context, userID string, item *collection.Item, parentID, collectionType string, listView bool) (response JFItem) {
	switch collectionType {
	case collection.CollectionMovies:
		return j.makeJFItemMovie(ctx, userID, item, parentID, listView)
	case collection.CollectionShows:
		return j.makeJFItemShow(ctx, userID, item, parentID)
	}
	log.Printf("makeJFItem: unknown item type: %+v", item)
	return JFItem{}
}

// makeJFItem make movie item
func (j *Jellyfin) makeJFItemMovie(ctx context.Context, userID string, i *collection.Item, parentID string, listView bool) (response JFItem) {
	response = JFItem{
		Type:                    "Movie",
		ID:                      i.ID,
		ParentID:                parentID,
		ServerID:                serverID,
		Name:                    i.Name,
		OriginalTitle:           i.Name,
		SortName:                i.Name,
		ForcedSortName:          i.Name,
		IsFolder:                false,
		LocationType:            "FileSystem",
		Path:                    "file.mp4",
		MediaType:               "Video",
		VideoType:               "VideoFile",
		Container:               "mov,mp4,m4a",
		Etag:                    idhash.IdHash(i.ID),
		DateCreated:             time.Unix(i.FirstVideo/1000, 0).UTC(),
		PremiereDate:            time.Unix(i.FirstVideo/1000, 0).UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
		PlayAccess:              "Full",
		ImageTags: &JFImageTags{
			Primary:  "primary_" + i.ID,
			Backdrop: "backdrop_" + i.ID,
		},
		// Required to have Infuse load backdrop of episode
		BackdropImageTags: []string{
			"backdrop_" + i.ID,
		},
	}

	i.LoadNfo()
	// fixme: this should come from collection package
	response.MediaSources = j.makeMediaSource(i.Video, i.Nfo)
	response.RunTimeTicks = response.MediaSources[0].RunTimeTicks
	response.MediaStreams = response.MediaSources[0].MediaStreams

	// listview = true, movie carousel return both primary and BackdropImageTags
	// non-listview = false, remove primary (thumbnail) image reference
	// if !listView {
	// 	response.ImageTags = nil
	// }

	j.enrichResponseWithNFO(&response, i.Nfo)
	if i.Nfo != nil {
		i.Genres = response.Genres
		i.OfficialRating = response.OfficialRating
		i.Year = response.ProductionYear
	}

	if playstate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(i.ID)); err == nil {
		response.UserData = j.makeJFUserData(userID, i.ID, playstate)
	}
	return response
}

// makeJFItemShow makes show item
func (j *Jellyfin) makeJFItemShow(ctx context.Context, userID string, i *collection.Item, parentID string) (response JFItem) {
	response = JFItem{
		Type:                    "Series",
		ID:                      i.ID,
		ParentID:                parentID,
		ServerID:                serverID,
		Name:                    i.Name,
		OriginalTitle:           i.Name,
		SortName:                i.Name,
		ForcedSortName:          i.Name,
		IsFolder:                true,
		Etag:                    idhash.IdHash(i.ID),
		DateCreated:             time.Unix(i.FirstVideo/1000, 0).UTC(),
		PremiereDate:            time.Unix(i.FirstVideo/1000, 0).UTC(),
		PrimaryImageAspectRatio: 0.6666666666666666,
		CanDelete:               false,
		CanDownload:             true,
		PlayAccess:              "Full",
		ImageTags: &JFImageTags{
			Primary:  "primary_" + i.ID,
			Backdrop: "backdrop_" + i.ID,
		},
		// Required to have Infuse load backdrop of episode
		BackdropImageTags: []string{
			"backdrop_" + i.ID,
		},
	}
	if i.Logo != "" {
		response.ImageTags.Logo = "logo_" + i.ID
	}

	j.enrichResponseWithNFO(&response, i.Nfo)

	response.ChildCount = len(i.Seasons)
	// In case show does not have any seasons no need to calculate userdata
	if response.ChildCount == 0 {
		return response
	}

	// Calculate the number of episodes and played episode in the show
	var playedEpisodes, totalEpisodes int
	var lastestPlayed time.Time
	for _, s := range i.Seasons {
		for _, e := range s.Episodes {
			totalEpisodes++
			episodePlaystate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(e.ID))
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
		playstate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(i.ID))
		if err != nil {
			playstate = database.UserData{
				Timestamp: time.Now().UTC(),
			}
		}
		response.UserData = j.makeJFUserData(userID, i.ID, playstate)

		response.UserData.UnplayedItemCount = totalEpisodes - playedEpisodes
		response.UserData.PlayedPercentage = 100 * playedEpisodes / totalEpisodes
		response.UserData.LastPlayedDate = lastestPlayed
		response.UserData.Key = response.ID
		if playedEpisodes == response.ChildCount {
			response.UserData.Played = true
		}

		// response.UserData = &JFUserData{
		// 	UnplayedItemCount: totalEpisodes - playedEpisodes,
		// 	PlayedPercentage:  100 * playedEpisodes / totalEpisodes,
		// 	LastPlayedDate:    lastestPlayed,
		// 	Key:               response.ID,
		// }
		// if playedEpisodes == response.ChildCount {
		// 	response.UserData.Played = true
		// }
	}
	return response
}

// makeJFItemSeason makes a season
func (j *Jellyfin) makeJFItemSeason(ctx context.Context, userID, seasonID string) (response JFItem, err error) {
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
		ParentLogoItemId: show.ID,
	}
	// Regular season? (>0)
	if season.SeasonNo != 0 {
		response.IndexNumber = season.SeasonNo
		response.Name = makeSeasonName(season.SeasonNo)
		response.SortName = fmt.Sprintf("%04d", season.SeasonNo)
	} else {
		// Specials tend to have season number 0, set season
		// number to 99 to make it sort at the end
		response.IndexNumber = 99
		response.Name = makeSeasonName(season.SeasonNo)
		response.SortName = "9999"
	}

	var playedEpisodes int
	var lastestPlayed time.Time
	for _, e := range season.Episodes {
		episodePlaystate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(e.ID))
		if err == nil {
			if episodePlaystate.Played {
				playedEpisodes++
				if episodePlaystate.Timestamp.After(lastestPlayed) {
					lastestPlayed = episodePlaystate.Timestamp
				}
			}
		}
	}

	playstate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(seasonID))
	if err != nil {
		playstate = database.UserData{
			Timestamp: time.Now().UTC(),
		}
	}
	response.UserData = j.makeJFUserData(userID, seasonID, playstate)

	response.UserData.UnplayedItemCount = response.ChildCount - playedEpisodes
	response.UserData.PlayedPercentage = 100 * playedEpisodes / response.ChildCount
	response.UserData.LastPlayedDate = lastestPlayed
	if playedEpisodes == response.ChildCount {
		response.UserData.Played = true
	}

	// response.UserData = &JFUserData{
	// 	UnplayedItemCount: response.ChildCount - playedEpisodes,
	// 	PlayedPercentage:  100 * playedEpisodes / response.ChildCount,
	// 	LastPlayedDate:    lastestPlayed,
	// 	Key:               response.ID,
	// }
	// if playedEpisodes == response.ChildCount {
	// 	response.UserData.Played = true
	// }

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

// makeJFItemEpisode makes an episode
func (j *Jellyfin) makeJFItemEpisode(ctx context.Context, userID, episodeID string) (response JFItem, err error) {
	_, show, season, episode := j.collections.GetEpisodeByID(trimPrefix(episodeID))
	if episode == nil {
		err = errors.New("could not find episode")
		return
	}

	response = JFItem{
		Type:         "Episode",
		ID:           itemprefix_episode + episodeID,
		Etag:         idhash.IdHash(episodeID),
		ServerID:     serverID,
		SeriesName:   show.Name,
		SeriesID:     show.ID,
		SeasonID:     itemprefix_season + season.ID,
		SeasonName:   makeSeasonName(season.SeasonNo),
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
		ParentLogoItemId: show.ID,
	}

	// Get a bunch of metadata from show-level nfo
	show.LoadNfo()
	if show.Nfo != nil {
		j.enrichResponseWithNFO(&response, show.Nfo)
		if show.Nfo != nil {
			show.Genres = response.Genres
			show.OfficialRating = response.OfficialRating
			show.Year = response.ProductionYear
		}
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

	if playstate, err := j.db.UserDataRepo.Get(ctx, userID, trimPrefix(episodeID)); err == nil {
		response.UserData = j.makeJFUserData(userID, episodeID, playstate)
	}
	return response, nil
}

// makeJFItemFavoritesOverview creates a list of favorite items
func (j *Jellyfin) makeJFItemFavoritesOverview(ctx context.Context, userID string) (items []JFItem, err error) {
	favoriteIDs, err := j.db.UserDataRepo.GetFavorites(ctx, userID)

	// log.Printf("makeJFItemFavoritesOverview: %+v, %+v", favoriteIDs, err)
	if err != nil {
		return
	}

	items = []JFItem{}
	for _, itemID := range favoriteIDs {
		c, i := j.collections.GetItemByID(itemID)
		if i != nil {
			item := j.makeJFItem(ctx, userID, i, CollectionIDToString(c.ID), c.Type, false)
			items = append(items, item)
		}
	}
	return
}

func (j *Jellyfin) makeJFItemPlaylistOverview(ctx context.Context, userID string) (items []JFItem, err error) {
	playlistIDs, err := j.db.PlaylistRepo.GetPlaylists(ctx, userID)

	log.Printf("makeJFItemPlaylistOverview: %+v, %+v", playlistIDs, err)
	if err != nil {
		return
	}
	// In case we have playlists populate, otherwise leave empty list
	for _, playlistID := range playlistIDs {
		item, err := j.makeJFItemPlaylist(ctx, userID, playlistID)
		if err == nil {
			items = append(items, item)
		}
	}
	return
}

func (j *Jellyfin) makeJFItemPlaylist(ctx context.Context, userID, playlistID string) (response JFItem, err error) {
	playlist, err := j.db.PlaylistRepo.GetPlaylist(ctx, userID, trimPrefix(playlistID))
	if playlist == nil {
		err = errors.New("could not find playlist")
		return
	}

	response = JFItem{
		Type:                     "Playlist",
		ID:                       itemprefix_playlist + playlist.ID,
		ServerID:                 serverID,
		ParentID:                 "1071671e7bffa0532e930debee501d2e", // fixme: this should be ID generated by makeJFItemCollectionPlaylist()
		Name:                     playlist.Name,
		SortName:                 playlist.Name,
		Etag:                     idhash.IdHash(playlist.ID),
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

func (j *Jellyfin) makeJFItemGenre(ctx context.Context, genre string) (response JFItem) {

	response = JFItem{
		ID:           idhash.IdHash(genre),
		ServerID:     serverID,
		Type:         "Genre",
		Name:         genre,
		SortName:     genre,
		Etag:         idhash.IdHash(genre),
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

// makeJFUserData creates a JFUserData object from Userdata
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

func (j *Jellyfin) enrichResponseWithNFO(response *JFItem, n *collection.Nfo) {
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
		if n.Season == "0" {
			n.Season = "99"
		}
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
		normalizedGenres := collection.NormalizeGenres(n.Genre)
		// Why do we populate two response fields with same data?
		response.Genres = normalizedGenres
		response.GenreItems = makeJFGenreItems(normalizedGenres)
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

func (j *Jellyfin) makeMediaSource(filename string, n *collection.Nfo) (mediasources []JFMediaSources) {
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

func CollectionIDToString(id int) string {
	return fmt.Sprintf("%d", id)
}
