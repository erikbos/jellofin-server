package jellyfin

import (
	"context"
	"net/http"

	"github.com/erikbos/jellofin-server/collection"
	"github.com/erikbos/jellofin-server/idhash"
)

// /Movies/Recommendations
//
// moviesRecommendationsHandler returns a list of recommended movie items
func (j *Jellyfin) moviesRecommendationsHandler(w http.ResponseWriter, r *http.Request) {
	// Not implemented
	response := []JFItem{}
	serveJSON(response, w)
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
