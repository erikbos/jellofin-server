package jellyfin

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// /Genres
//
// genresHandler returns a list of genres for one or all collections.
func (j *Jellyfin) genresHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	// Get all items for which we need to get genres.
	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")
	items, err := j.getJFItems(r.Context(), accessToken.User.ID, parentID)
	if err != nil {
		apierror(w, "Failed to get items", http.StatusInternalServerError)
		return
	}

	// Build unique genre from the items.
	genres := []JFItem{}
	genreSet := make(map[string]struct{})
	for _, item := range items {
		for _, genre := range item.GenreItems {
			if _, exists := genreSet[genre.ID]; !exists {
				genreSet[genre.ID] = struct{}{}
				if genreItem, err := j.makeJFItemGenre(r.Context(), accessToken.User.ID, genre.ID); err == nil {
					genres = append(genres, genreItem)
				}
			}
		}
	}

	genres = j.applyItemSorting(genres, r.URL.Query())

	response := UserItemsResponse{
		Items:            genres,
		TotalRecordCount: len(genres),
		StartIndex:       0,
	}

	serveJSON(response, w)
}

// /Genres/Thriller
//
// genreHandler returns details of a specific genre
func (j *Jellyfin) genreHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	vars := mux.Vars(r)
	genre := vars["name"]
	if genre == "" {
		apierror(w, "Missing genre", http.StatusBadRequest)
		return
	}
	var err error
	genre, err = url.PathUnescape(genre)
	if err != nil {
		apierror(w, "Invalid genre name", http.StatusBadRequest)
		return
	}
	//TOD: validate genre is actually in the collection?
	response, err := j.makeJFItemGenre(r.Context(), accessToken.User.ID, makeJFGenreID(genre))
	if err != nil {
		apierror(w, "Genre not found", http.StatusNotFound)
		return
	}
	serveJSON(response, w)
}

// /Items/Filters?userId=XAOVnIQY8sd0&parentId=collection_1
//
// usersItemsFiltersHandler returns a list of genre filter values
func (j *Jellyfin) usersItemsFiltersHandler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	// Get all items for which we need to get genres.
	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")
	items, err := j.getJFItems(r.Context(), accessToken.User.ID, parentID)
	if err != nil {
		apierror(w, "Failed to get items", http.StatusInternalServerError)
		return
	}

	genres := make([]string, 0)
	studios := make([]string, 0)
	tags := make([]string, 0)
	official := make([]string, 0)
	years := make([]int, 0)

	for _, i := range items {
		for _, g := range i.Genres {
			if !slices.Contains(genres, g) {
				genres = append(genres, g)
			}
		}
		for _, s := range i.Studios {
			if !slices.Contains(studios, s.Name) {
				studios = append(studios, s.Name)
			}
		}
		if i.OfficialRating != "" && !slices.Contains(official, i.OfficialRating) {
			official = append(official, i.OfficialRating)
		}
		if i.ProductionYear != 0 && !slices.Contains(years, i.ProductionYear) {
			years = append(years, i.ProductionYear)
		}
	}

	slices.Sort(years)

	response := JFItemFilterResponse{
		Genres:          genres,
		Tags:            tags,
		OfficialRatings: official,
		Years:           years,
	}
	serveJSON(response, w)
}

// /Items/Filters2?userId=XAOVnIQY8sd0&parentId=collection_1
//
// usersItemsFilters2Handler returns a list of genre name and their id.
func (j *Jellyfin) usersItemsFilters2Handler(w http.ResponseWriter, r *http.Request) {
	accessToken := j.getAccessTokenDetails(w, r)
	if accessToken == nil {
		return
	}

	// Get all items for which we need to get genres.
	queryparams := r.URL.Query()
	parentID := queryparams.Get("parentId")
	items, err := j.getJFItems(r.Context(), accessToken.User.ID, parentID)
	if err != nil {
		apierror(w, "Failed to get items", http.StatusInternalServerError)
		return
	}

	// Build unique genre from the items.
	genres := []JFGenreItem{}
	genreIDs := make(map[string]struct{})
	for _, item := range items {
		for _, genre := range item.GenreItems {
			if genre.ID != "" {
				if _, exists := genreIDs[genre.ID]; !exists {
					genreIDs[genre.ID] = struct{}{}
					genres = append(genres, genre)
				}
			}
		}
	}

	response := JFItemFilter2Response{
		Genres: genres,
		Tags:   []string{},
	}
	serveJSON(response, w)

}

func makeJFGenreItems(array []string) (genreItems []JFGenreItem) {
	for _, v := range array {
		genreItems = append(genreItems, JFGenreItem{
			Name: v,
			ID:   makeJFGenreID(v),
		})
	}
	return genreItems
}

func (j *Jellyfin) makeJFItemGenre(ctx context.Context, _, genreID string) (JFItem, error) {
	genre, err := decodeJFGenreID(genreID)
	if err != nil {
		return JFItem{}, err
	}

	response := JFItem{
		ID:           genreID,
		ServerID:     j.serverID,
		Type:         itemTypeGenre,
		Name:         genre,
		SortName:     genre,
		Etag:         genreID,
		DateCreated:  time.Now().UTC(),
		PremiereDate: time.Now().UTC(),
		LocationType: "FileSystem",
		MediaType:    "Unknown",
		ChildCount:   1,
		ImageTags:    j.makeJFImageTags(ctx, genreID, ImageTypePrimary),
	}

	if genreItemCount := j.collections.GenreItemCount(); genreItemCount != nil {
		if genreCount, ok := genreItemCount[genre]; ok {
			response.ChildCount = genreCount
		}
	}
	return response, nil
}

// makeJFGenreID returns an external id for a genre name.
func makeJFGenreID(genre string) string {
	// base64 encoded to handle special characters, as some clients have issues with % characters in IDs.
	return itemprefix_genre + base64.RawURLEncoding.EncodeToString([]byte(genre))
}

// isJFGenreID checks if the provided ID is a genre ID.
func isJFGenreID(id string) bool {
	return strings.HasPrefix(id, itemprefix_genre)
}

// decodeJFGenreID decodes a genre ID to get the original name.
func decodeJFGenreID(genreID string) (string, error) {
	if !strings.HasPrefix(genreID, itemprefix_genre) {
		return "", errors.New("invalid genre ID")
	}
	genreBytes, err := base64.RawURLEncoding.DecodeString(trimPrefix(genreID))
	if err != nil {
		return "", errors.New("cannot decode genre ID")
	}
	return string(genreBytes), nil
}
