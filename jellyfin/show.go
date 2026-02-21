package jellyfin

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/database/model"
)

// /Shows/rXlq4EHNxq4HIVQzw3o2/Episodes?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&SeasonId=rXlq4EHNxq4HIVQzw3o2/1
//
// generate episode overview for one season of a show
func (j *Jellyfin) showsEpisodesHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()
	showID := vars["showid"]

	// If provided a seasonID, rewrite request for a showID with a seasonID filter
	if isJFSeasonID(showID) {
		seasonID := trimPrefix(showID)
		if _, show, season := j.collections.GetSeasonByID(seasonID); season != nil {
			queryparams.Set("seasonId", showID)
			showID = show.ID()
		} else {
			apierror(w, "Season not found", http.StatusNotFound)
			return
		}
		log.Printf("showsEpisodesHandler: rewritten seasonID %s request to show request with season filter, showID: %s, seasonID: %s\n", vars["show"], showID, seasonID)
	}

	_, show := j.collections.GetShowByID(showID)
	if show == nil {
		apierror(w, "Show not found", http.StatusNotFound)
		return
	}
	// Create API response for all episodes of the show
	episodes := make([]JFItem, 0)
	for _, s := range show.Seasons {
		if episodesOfSeason, err := j.makeJFEpisodesOverview(r.Context(), accessToken.User.ID, &s); err == nil {
			episodes = append(episodes, episodesOfSeason...)
		}
	}

	// Apply filtering, e.g. if a particular season is requested ("seasonId")
	episodes = j.applyItemsFilter(episodes, queryparams)

	episodes = j.applyItemSorting(episodes, queryparams)

	response := UserItemsResponse{
		Items:            episodes,
		TotalRecordCount: len(episodes),
		StartIndex:       0,
	}
	serveJSON(response, w)
}

// /Shows/4QBdg3S803G190AgFrBf/Seasons?UserId=2b1ec0a52b09456c9823a367d84ac9e5&ExcludeLocationTypes=Virtual&Fields=DateCreated,Etag,Genres,MediaSources,AlternateMediaSources,Overview,ParentId,Path,People,ProviderIds,SortName,RecursiveItemCount,ChildCount
//
// showsSeasonsHandler returns a list of seasons for a specific show
func (j *Jellyfin) showsSeasonsHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	queryparams := r.URL.Query()

	showID := vars["showid"]
	_, show := j.collections.GetShowByID(showID)
	if show == nil {
		apierror(w, "Show not found", http.StatusNotFound)
		return
	}
	seasons, err := j.makeJFSeasonsOverview(r.Context(), accessToken.User.ID, show)
	if err != nil {
		apierror(w, "Could not generate seasons overview", http.StatusInternalServerError)
		return
	}

	seasons = j.applyItemsFilter(seasons, queryparams)

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

// /Shows/NextUp?
//
//	enableImageTypes=Primary&
//	enableImageTypes=Backdrop&
//	enableImageTypes=Thumb&
//	enableResumable=false&
//	fields=MediaSourceCount&limit=20&
//
// /Shows/NextUp?seriesId=NR1sUDwF4p5e7TbxcYGY&userId=K5EIX2NC
// /Shows/NextUp?disableFirstEpisode=true&enableResumable=true&enableRewatching=false&enableTotalRecordCount=false&userId=jack
//
// showsNextUpHandler returns a list of next up items for the user
func (j *Jellyfin) showsNextUpHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}
	queryparams := r.URL.Query()
	seriesID := queryparams.Get("seriesId")

	var nextUpItemIDs []string
	if seriesID != "" {
		recentlyWatchedIDs, err := j.repo.GetRecentlyWatched(r.Context(), accessToken.User.ID, 100000, true)
		if err != nil {
			apierror(w, "Could not get recently watched items list", http.StatusInternalServerError)
			return
		}
		// Get next up items based on recently watched items for a series
		nextUpItemIDs, err = j.collections.NextUpInSeries(recentlyWatchedIDs, seriesID)
		if err != nil {
			apierror(w, "Could not get next up items list", http.StatusInternalServerError)
			return
		}
	}

	// If no next up items found for the series, or no seriesID provided
	// get next up items based on recently watched items across all series
	if len(nextUpItemIDs) == 0 {
		recentlyWatchedIDs, err := j.repo.GetRecentlyWatched(r.Context(), accessToken.User.ID, 10, true)
		if err != nil {
			apierror(w, "Could not get recently watched items list", http.StatusInternalServerError)
			return
		}
		// Get next up items based on recently watched items and optional seriesID filter
		nextUpItemIDs, err = j.collections.NextUpInCollection(recentlyWatchedIDs, seriesID)
		if err != nil {
			apierror(w, "Could not get next up items list", http.StatusInternalServerError)
			return
		}
	}

	items := make([]JFItem, 0, len(nextUpItemIDs))
	for _, id := range nextUpItemIDs {
		if _, i, s, e := j.collections.GetEpisodeByID(id); i != nil {
			jfitem, err := j.makeJFItemEpisode(r.Context(), accessToken.User.ID, e, s.ID())
			if err == nil && j.applyItemFilter(&jfitem, queryparams) {
				items = append(items, jfitem)
			}
			continue
		}
		log.Printf("usersItemsResumeHandler: item %s not found\n", id)
	}

	items = j.applyItemsFilter(items, queryparams)

	// Apply user provided filters & sorting
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

// makeJFItemShow makes show item
func (j *Jellyfin) makeJFItemShow(ctx context.Context, userID string, show *collection.Show, parentID string) (JFItem, error) {
	response := JFItem{
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
		Etag:                    show.Etag(),
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
func (j *Jellyfin) makeJFItemSeason(ctx context.Context, userID string, season *collection.Season, _ string) (JFItem, error) {
	_, show, season := j.collections.GetSeasonByID(season.ID())
	if season == nil {
		return JFItem{}, errors.New("could not find season")
	}

	response := JFItem{
		Type:               itemTypeSeason,
		ID:                 makeJFSeasonID(season.ID()),
		SeriesID:           show.ID(),
		SeriesName:         show.Name(),
		ParentID:           show.ID(),
		ParentLogoItemId:   show.ID(),
		ServerID:           j.serverID,
		IsFolder:           true,
		LocationType:       "FileSystem",
		Etag:               season.Etag(),
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
func (j *Jellyfin) makeJFItemEpisode(ctx context.Context, userID string, episode *collection.Episode, _ string) (JFItem, error) {
	_, show, season, episode := j.collections.GetEpisodeByID(episode.ID())
	if episode == nil {
		return JFItem{}, errors.New("could not find episode")
	}

	response := JFItem{
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
		Etag:              episode.Etag(),
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

// makeJFSeasonID returns an external id for a season ID.
func makeJFSeasonID(seasonID string) string {
	return itemprefix_season + seasonID
}

// makeJFEpisodeID returns an external id for an episode ID.
func makeJFEpisodeID(episodeID string) string {
	return itemprefix_episode + episodeID
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
